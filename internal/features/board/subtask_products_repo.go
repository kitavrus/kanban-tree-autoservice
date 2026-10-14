package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// SubtaskProductRepo — товары услуги (design-sql.md §Subtask Parent/Lock/Subtask Product
// Queries). Без pool: мутации всегда под LockTask+Lock Subtask (Lock Order).
type SubtaskProductRepo struct{}

func NewSubtaskProductRepo() *SubtaskProductRepo { return &SubtaskProductRepo{} }

// ParentOf — design-sql.md §Subtask Parent Query (второй запрос): task_id + subtask_id
// владельца строки товара. 0 → errNotFound (SUBTASK_PRODUCT_NOT_FOUND).
func (r *SubtaskProductRepo) ParentOf(ctx context.Context, tx pgx.Tx, id string) (taskID int64, subtaskID string, err error) {
	err = tx.QueryRow(ctx, `
		SELECT s.task_id, sp.subtask_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
		WHERE sp.id = $1`, id).Scan(&taskID, &subtaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", errNotFound
	}
	return taskID, subtaskID, err
}

// Lock — design-sql.md §Lock Subtask Query (второй запрос), ПОСЛЕ SubtaskRepo.Lock
// (Lock Order). 0 → errNotFound.
func (r *SubtaskProductRepo) Lock(ctx context.Context, tx pgx.Tx, id, subtaskID string) error {
	var got string
	err := tx.QueryRow(ctx, `SELECT id FROM subtask_products WHERE id = $1 AND subtask_id = $2 FOR UPDATE`, id, subtaskID).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	return err
}

// ProductState — существование + is_active товара (400 различает «не найден»/«в архиве»).
// Мягко удалённый товар — «не найден» (soft-delete-catalog DS-014). Строка товара берётся FOR
// SHARE до конца транзакции и ДО строк product_stock (их берёт Reconcile; порядок «товар →
// product_stock» общий с удалением товара): удаление товара либо уже видно, либо ждёт.
func (r *SubtaskProductRepo) ProductState(ctx context.Context, tx pgx.Tx, id string) (exists, active bool, err error) {
	err = tx.QueryRow(ctx, `SELECT true, is_active FROM products WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, id).Scan(&exists, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return exists, active, err
}

// errNoActiveWarehouse — нет ни одного активного склада (D47 не даёт архивировать
// последний — защитная ветка); наружу — 409 NO_ACTIVE_WAREHOUSE, не 500.
var errNoActiveWarehouse = ErrNoActiveWarehouse

// DefaultWarehouse — склад по умолчанию строки товара услуги subtaskID (DS-005, D58):
// текущий склад единицы оборудования услуги (subtasks.equipment_id → equipment, transfers-core
// DS-046), если единица на складе (at_warehouse — не у клиента и не в пути) и склад активен;
// иначе первый активный склад по position (default_warehouse_id(), D49). Мягко удалённый склад
// по умолчанию не выбирается (soft-delete-warehouses-suppliers DS-021). Найденный склад берётся
// FOR SHARE с условием «не удалён» (как WarehouseState): если его успели удалить между выбором и
// блокировкой, выбор повторяется — следующий запрос (READ COMMITTED) удаление уже видит.
func (r *SubtaskProductRepo) DefaultWarehouse(ctx context.Context, tx pgx.Tx, subtaskID string) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		var id *string
		if err := tx.QueryRow(ctx, `
		SELECT coalesce((
		    SELECT e.warehouse_id FROM subtasks s
		    JOIN equipment e  ON e.id = s.equipment_id
		    JOIN warehouses w ON w.id = e.warehouse_id AND w.is_active AND w.deleted_at IS NULL
		    WHERE s.id = $1 AND e.location_status = 'at_warehouse'), default_warehouse_id())::text`, subtaskID).Scan(&id); err != nil {
			return "", err
		}
		if id == nil {
			return "", errNoActiveWarehouse
		}
		// под блокировкой склад обязан быть и не удалён, и активен: архивирование между выбором и
		// блокировкой — тоже повод выбрать заново.
		exists, active, err := r.WarehouseState(ctx, tx, *id)
		if err != nil {
			return "", err
		}
		if exists && active {
			return *id, nil
		}
	}
	return "", errNoActiveWarehouse
}

// WarehouseState — существование + is_active действующего склада (400 различает «не найден»/«в
// архиве»); мягко удалённый склад — как несуществующий (DS-014). Строка берётся FOR SHARE до
// конца транзакции: новая строка товара заказа — ссылка на склад и, если заказ уже списывает
// остаток, остаток на нём; удаление склада (FOR UPDATE той же строки) либо уже видно → not_found,
// либо ждёт и затем видит остаток (409 WAREHOUSE_IN_USE). Порядок: заказ → склад → product_stock.
func (r *SubtaskProductRepo) WarehouseState(ctx context.Context, tx pgx.Tx, id string) (exists, active bool, err error) {
	err = tx.QueryRow(ctx, `SELECT true, is_active FROM warehouses WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, id).Scan(&exists, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	return exists, active, err
}

// QtyIncreaseOnDeletedWarehouse — склад warehouseID строки товара id мягко удалён И новое
// количество newQty больше текущего. Строка склада берётся FOR SHARE до конца транзакции (как у
// новой строки, WarehouseState): удаление склада (FOR UPDATE той же строки) либо уже видно —
// увеличение отклоняется, либо ждёт правку строки. Строка товара уже заблокирована (Lock).
func (r *SubtaskProductRepo) QtyIncreaseOnDeletedWarehouse(ctx context.Context, tx pgx.Tx, id, warehouseID, newQty string) (bool, error) {
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM warehouses WHERE id = $1 FOR SHARE`, warehouseID).Scan(&deleted); err != nil {
		return false, err
	}
	if !deleted {
		return false, nil
	}
	var increase bool
	err := tx.QueryRow(ctx, `SELECT $2::numeric > qty FROM subtask_products WHERE id = $1`, id, newQty).Scan(&increase)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errNotFound
	}
	return increase, err
}

// WarehouseOf — текущий склад строки товара (под Lock).
func (r *SubtaskProductRepo) WarehouseOf(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var wh string
	err := tx.QueryRow(ctx, `SELECT warehouse_id::text FROM subtask_products WHERE id = $1`, id).Scan(&wh)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return wh, err
}

// Insert — design-sql.md §Subtask Product Queries: цена копируется из ТЕКУЩЕЙ
// products.price (AC-008), склад — warehouseID (DS-005). 0 строк (товар не найден/архивен
// — гонка) → errNotFound.
func (r *SubtaskProductRepo) Insert(ctx context.Context, tx pgx.Tx, subtaskID, productID, qty, warehouseID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO subtask_products (subtask_id, product_id, qty, price, position, warehouse_id)
		SELECT $1, p.id, $3::numeric, p.price, coalesce((SELECT max(position) FROM subtask_products WHERE subtask_id = $1), 0) + 1, $4::uuid
		FROM products p WHERE p.id = $2 AND p.is_active AND p.deleted_at IS NULL
		RETURNING id`, subtaskID, productID, qty, warehouseID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return id, err
}

// Update — LWW (D28): полная замена qty/price + склад. price=nil → NULL («без цены»,
// итог 0).
func (r *SubtaskProductRepo) Update(ctx context.Context, tx pgx.Tx, id, qty string, price *string, warehouseID string) error {
	tag, err := tx.Exec(ctx, `UPDATE subtask_products SET qty = $2::numeric, price = $3::numeric, warehouse_id = $4::uuid WHERE id = $1`,
		id, qty, price, warehouseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// Delete — 0 строк → errNotFound (идемпотентность DELETE — повтор тоже 404, LV-173).
func (r *SubtaskProductRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM subtask_products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
