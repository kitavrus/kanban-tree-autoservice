package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// TaskEquipmentRepo — оборудование заказа (design-sql.md §Task Equipment Queries,
// §Foreign Equipment Query). Без pool: все методы принимают чужую tx (мутации всегда под
// LockTask, ADR-104).
type TaskEquipmentRepo struct{}

func NewTaskEquipmentRepo() *TaskEquipmentRepo { return &TaskEquipmentRepo{} }

// EquipmentClient — владелец действующей единицы (для сравнения с tasks.client_id, AC-004).
// Мягко удалённая единица для новых строк заказа не существует — errNotFound (soft-delete-clients
// DS-014). FOR SHARE: удаление единицы и клиента пишет строку equipment (блокировка NO KEY
// UPDATE, с FOR KEY SHARE она совместима — поэтому SHARE) — оно либо уже видно, либо ждёт
// добавление в заказ, и единица уходит в удалённые уже в составе заказа с бейджем. Порядок
// блокировок: заказ (LockTask) → equipment.
func (r *TaskEquipmentRepo) EquipmentClient(ctx context.Context, tx pgx.Tx, equipmentID string) (string, error) {
	var clientID string
	err := tx.QueryRow(ctx, `SELECT client_id FROM equipment WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, equipmentID).Scan(&clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return clientID, err
}

// OrderCar — автомобиль для нового заказа по госномеру (CarForOrder).
type OrderCar struct {
	ClientID    string
	PlateNumber string
}

// CarForOrder — действующий автомобиль действующего клиента под FOR SHARE (как
// EquipmentClient: удаление единицы/клиента либо уже видно, либо ждёт создание заказа).
// Нет/удалён → errNotFound.
func (r *TaskEquipmentRepo) CarForOrder(ctx context.Context, tx pgx.Tx, equipmentID string) (*OrderCar, error) {
	var c OrderCar
	err := tx.QueryRow(ctx, `
		SELECT e.client_id, e.plate_number
		FROM equipment e JOIN clients cl ON cl.id = e.client_id AND cl.deleted_at IS NULL
		WHERE e.id = $1 AND e.deleted_at IS NULL
		FOR SHARE OF e`, equipmentID).Scan(&c.ClientID, &c.PlateNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Insert — идемпотентно (ON CONFLICT DO NOTHING): повторный POST той же единицы — 201 без
// изменений, не ошибка (design-errors.md §Validation Fields). inserted=false — единица уже
// была в заказе.
func (r *TaskEquipmentRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, in TaskEquipmentCreateRequest) (inserted bool, err error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO task_equipment (task_id, equipment_id, completeness, condition) VALUES ($1, $2, $3, $4)
		ON CONFLICT (task_id, equipment_id) DO NOTHING`,
		taskID, in.EquipmentID, in.Completeness, in.Condition)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Update — 0 строк → errNotFound (ErrTaskEquipmentNotFound).
func (r *TaskEquipmentRepo) Update(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string, in TaskEquipmentUpdateRequest) error {
	tag, err := tx.Exec(ctx, `
		UPDATE task_equipment SET completeness = $3, condition = $4 WHERE task_id = $1 AND equipment_id = $2`,
		taskID, equipmentID, in.Completeness, in.Condition)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// UsedBy — незавершённые/все услуги заказа, где выбрана эта единица оборудования
// (design-sql.md: «удаление единицы»). Непустой результат → 409 TASK_EQUIPMENT_IN_USE.
func (r *TaskEquipmentRepo) UsedBy(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) ([]OpenSubtask, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, sv.name, s.status FROM subtasks s JOIN services sv ON sv.id = s.service_id
		WHERE s.task_id = $1 AND s.equipment_id = $2 ORDER BY s.position, s.id`, taskID, equipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenSubtask{}
	for rows.Next() {
		var o OpenSubtask
		if err := rows.Scan(&o.ID, &o.ServiceName, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Delete — 0 строк → errNotFound (ErrTaskEquipmentNotFound).
func (r *TaskEquipmentRepo) Delete(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM task_equipment WHERE task_id = $1 AND equipment_id = $2`, taskID, equipmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// Attached — единица уже принята в заказ (design-errors.md §Validation Fields: `subtasks.
// equipment_id` — «NULL или единица из task_equipment этого заказа», используется
// валидатором услуги в фазе 03).
func (r *TaskEquipmentRepo) Attached(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM task_equipment WHERE task_id = $1 AND equipment_id = $2)`,
		taskID, equipmentID).Scan(&exists)
	return exists, err
}

// Deleted — единица мягко удалена (soft-delete-clients DS-014): назначить её услуге нельзя.
// FOR SHARE — как EquipmentClient: признак читается под блокировкой строки equipment.
func (r *TaskEquipmentRepo) Deleted(ctx context.Context, tx pgx.Tx, equipmentID string) (bool, error) {
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM equipment WHERE id = $1 FOR SHARE`, equipmentID).Scan(&deleted)
	return deleted, err
}

// ForeignInUse — design-sql.md §Foreign Equipment Query: смена client_id заказа — открытые
// (по позиции) услуги с оборудованием ПРЕЖНЕГО клиента.
func (r *TaskEquipmentRepo) ForeignInUse(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) ([]OpenSubtask, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, sv.name, s.status
		FROM subtasks s JOIN services sv ON sv.id = s.service_id JOIN equipment e ON e.id = s.equipment_id
		WHERE s.task_id = $1 AND e.client_id <> $2 ORDER BY s.position, s.id`, taskID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenSubtask{}
	for rows.Next() {
		var o OpenSubtask
		if err := rows.Scan(&o.ID, &o.ServiceName, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// DeleteForeign — чужие (после смены клиента) task_equipment убираются из заказа молча
// (design-sql.md §Foreign Equipment Query).
func (r *TaskEquipmentRepo) DeleteForeign(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM task_equipment te USING equipment e
		WHERE te.task_id = $1 AND e.id = te.equipment_id AND e.client_id <> $2`, taskID, clientID)
	return err
}
