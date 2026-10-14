package transfers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

var (
	errNotFound = errors.New("transfers: not found")
	// errVersion — присланный version не совпал с текущим (409, DS-048).
	errVersion = errors.New("transfers: version mismatch")
	// errOverflow — 22003 numeric_value_out_of_range при изменении остатка.
	errOverflow = errors.New("transfers: stock overflow")
	// errTaskDeleted — заказ нового документа мягко удалён (409 TASK_DELETED, DS-007).
	errTaskDeleted = errors.New("transfers: task deleted")
	// errNotDeleted — восстановление действующего документа (409 NOT_DELETED, DS-024).
	errNotDeleted = errors.New("transfers: not deleted")
	// errClientMismatch — клиент заказа сменился, пока документ был удалён (409
	// TRANSFER_CLIENT_MISMATCH, DS-040).
	errClientMismatch = errors.New("transfers: client mismatch")
)

// Действия над документом — ключ сообщения 409 TRANSFER_STATUS_CONFLICT (§6.4–§6.7).
const (
	actionUpdate  = "update"
	actionSend    = "send"
	actionReceive = "receive"
	actionCancel  = "cancel"
	// transfers-revert §6.1, §6.2.
	actionUnreceive = "unreceive"
	actionDelete    = "delete"
)

// statusError — действие недоступно в текущем статусе документа (409; сообщение зависит от
// пары действие × статус).
type statusError struct{ action, status string }

func (e *statusError) Error() string {
	return "transfers: status conflict: " + e.action + " in " + e.status
}

// equipmentBusyError — единица уже стоит в другом незавершённом документе (DS-019, 409
// EQUIPMENT_IN_TRANSFER).
type equipmentBusyError struct {
	equipmentID string
	transferID  int64
}

func (e *equipmentBusyError) Error() string { return "transfers: equipment in transfer" }

// notAtSourceError — единица числится не в источнике документа (DS-036, DS-013; 409
// TRANSFER_EQUIPMENT_NOT_AT_SOURCE): не на складе отправления либо, при fromClient (приём от
// клиента), не «у клиента».
type notAtSourceError struct {
	equipmentID string
	fromClient  bool
}

func (e *notAtSourceError) Error() string { return "transfers: equipment not at source" }

// notInTaskError — восстановление заявки с заказом: единицу equipmentID убрали из оборудования
// заказа, пока документ был удалён (409 TRANSFER_EQUIPMENT_NOT_IN_TASK). При создании и правке
// то же условие — ошибка поля not_in_task (checkRefs, DS-035).
type notInTaskError struct{ equipmentID string }

// deletedRefError — запись из состава документа мягко удалена (soft-delete-clients DS-045):
// восстановление заявки и отмена проведения полученного документа → 409
// TRANSFER_HAS_DELETED_REFS (details.kind, details.id); при создании и правке документа то же
// условие для единицы — ошибка поля not_found (DS-014).
type deletedRefError struct{ kind, id string }

func (e *deletedRefError) Error() string { return "transfers: deleted ref " + e.kind }

// refKindEquipment — вид удалённой записи состава документа (details.kind).
const refKindEquipment = "equipment"

// refKindProduct — details.kind отказа по мягко удалённому товару (soft-delete-catalog DS-045,
// DS-046).
const refKindProduct = "product"

// refKindWarehouse, refKindSupplier — details.kind отказа по мягко удалённому складу либо
// поставщику документа (soft-delete-warehouses-suppliers DS-045, DS-046, DS-047).
const (
	refKindWarehouse = "warehouse"
	refKindSupplier  = "supplier"
)

func (e *notInTaskError) Error() string { return "transfers: equipment not in task" }

// unreceiveBlockedError — отмена проведения запрещена единицей equipmentID (transfers-revert
// DS-023, 409 TRANSFER_UNRECEIVE_BLOCKED): later — её увёз более поздний полученный документ
// transferID; иначе она стоит в другом незавершённом документе transferID. transferID = 0 —
// единица числится не там, куда её привёз документ, а мешающего документа нет (местоположение
// правили в обход перемещений).
type unreceiveBlockedError struct {
	equipmentID string
	transferID  int64
	later       bool
}

func (e *unreceiveBlockedError) Error() string { return "transfers: unreceive blocked" }

// fieldsError — ошибки полей, найденные под транзакцией (склад/товар/единица не существует
// или в архиве, факт приёмки вне границ).
type fieldsError struct{ Fields []errorspkg.Field }

func (e *fieldsError) Error() string { return "transfers: invalid fields" }

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ListParams — фильтры списка (комбинация по «И»). CreatedFrom/CreatedTo — границы периода
// в UTC: [CreatedFrom; CreatedTo) (DS-034). ClientOnly — скоуп исполнителя: только документы
// с клиентом (DS-018).
type ListParams struct {
	Status, WarehouseID, Direction, SupplierID string
	TaskID                                     *int64
	CreatedFrom, CreatedTo                     *time.Time
	ClientOnly                                 bool
	Limit, Offset                              int
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// amountSelectSQL — сумма накладной (transfers-sales DS-038): Σ round(price × qty_sent, 2)
// товарных строк документа t; цена есть только у накладной «склад → клиент», поэтому вне её
// и без товаров — 0.00. Скаляр-фрагмент коррелирован с t.id.
const amountSelectSQL = `
       (SELECT round(coalesce(sum(round(ai.price * ai.qty_sent, 2)), 0), 2)
        FROM transfer_items ai WHERE ai.transfer_id = t.id AND ai.kind = 'product')::text`

// List — действующие перемещения, новые сверху; warehouse_id — откуда ИЛИ куда. Склады и
// поставщик соединяются БЕЗ условия «не удалён»: документ показывает их с признаком
// *_is_deleted (soft-delete-warehouses-suppliers DS-014). Мягко
// удалённые документы в список не входят (soft-delete-transfers DS-026); task_is_deleted —
// заказ документа мягко удалён (DS-007; false, если заказа нет).
func (r *Repo) List(ctx context.Context, p ListParams) ([]TransferSummary, int, error) {
	rows, err := r.pool.Query(ctx, `
SELECT t.id, t.direction, t.from_warehouse_id::text, fw.name, fw.deleted_at IS NOT NULL,
       t.to_warehouse_id::text, tw.name, tw.deleted_at IS NOT NULL,
       t.task_id, tk.deleted_at IS NOT NULL, t.client_id::text, c.company_name, c.deleted_at IS NOT NULL,
       t.supplier_id::text, sp.name, sp.deleted_at IS NOT NULL,
       t.status, t.note,
       (SELECT count(*) FROM transfer_items i WHERE i.transfer_id = t.id)::int,
       cu.name, t.unreceived_at, t.created_at, t.updated_at, t.version, `+amountSelectSQL+`, count(*) OVER () AS total
FROM transfers t
LEFT JOIN warehouses fw ON fw.id = t.from_warehouse_id
LEFT JOIN warehouses tw ON tw.id = t.to_warehouse_id
LEFT JOIN tasks tk      ON tk.id = t.task_id
LEFT JOIN clients c     ON c.id = t.client_id
LEFT JOIN suppliers sp  ON sp.id = t.supplier_id
JOIN users cu           ON cu.id = t.created_by
WHERE t.deleted_at IS NULL
  AND ($1::text IS NULL OR t.status = $1)
  AND ($2::uuid IS NULL OR t.from_warehouse_id = $2::uuid OR t.to_warehouse_id = $2::uuid)
  AND ($3::text IS NULL OR t.direction = $3)
  AND ($4::bigint IS NULL OR t.task_id = $4)
  AND ($5::uuid IS NULL OR t.supplier_id = $5::uuid)
  AND ($6::timestamptz IS NULL OR t.created_at >= $6)
  AND ($7::timestamptz IS NULL OR t.created_at < $7)
  AND (NOT $8::boolean OR t.direction = ANY($9::text[]))
ORDER BY t.id DESC
LIMIT $10 OFFSET $11`, nullable(p.Status), nullable(p.WarehouseID), nullable(p.Direction), p.TaskID,
		nullable(p.SupplierID), p.CreatedFrom, p.CreatedTo, p.ClientOnly, clientDirections, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []TransferSummary{}
	total := 0
	for rows.Next() {
		var t TransferSummary
		var amount string
		if err := rows.Scan(&t.ID, &t.Direction, &t.FromWarehouseID, &t.FromWarehouseName, &t.FromWarehouseIsDeleted,
			&t.ToWarehouseID, &t.ToWarehouseName, &t.ToWarehouseIsDeleted,
			&t.TaskID, &t.TaskIsDeleted, &t.ClientID, &t.ClientName, &t.ClientIsDeleted, &t.SupplierID, &t.SupplierName, &t.SupplierIsDeleted,
			&t.Status, &t.Note, &t.ItemsCount, &t.CreatedByName, &t.UnreceivedAt, &t.CreatedAt, &t.UpdatedAt, &t.Version, &amount, &total); err != nil {
			return nil, 0, err
		}
		t.Amount = numeric.Amount(amount)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// Get — карточка действующего перемещения со строками (q — пул или транзакция); мягко
// удалённый документ — errNotFound (DS-026).
func (r *Repo) Get(ctx context.Context, id int64) (*TransferResponse, error) {
	return getTransfer(ctx, r.pool, id)
}

func getTransfer(ctx context.Context, q querier, id int64) (*TransferResponse, error) {
	var t TransferResponse
	var amount string
	err := q.QueryRow(ctx, `
SELECT t.id, t.direction, t.from_warehouse_id::text, fw.name, fw.deleted_at IS NOT NULL,
       t.to_warehouse_id::text, tw.name, tw.deleted_at IS NOT NULL,
       t.task_id, tk.title, tk.deleted_at IS NOT NULL, t.client_id::text, c.company_name, c.deleted_at IS NOT NULL,
       t.supplier_id::text, sp.name, sp.deleted_at IS NOT NULL,
       t.status, t.note,
       t.created_by::text, cu.name,
       t.sent_by::text, su.name, t.sent_at,
       t.received_by::text, ru.name, t.received_at,
       t.cancelled_by::text, xu.name, t.cancelled_at,
       t.unreceived_by::text, uu.name, t.unreceived_at, t.unreceive_reason,
       t.created_at, t.updated_at, t.version, `+amountSelectSQL+`
FROM transfers t
LEFT JOIN warehouses fw ON fw.id = t.from_warehouse_id
LEFT JOIN warehouses tw ON tw.id = t.to_warehouse_id
LEFT JOIN tasks tk      ON tk.id = t.task_id
LEFT JOIN clients c     ON c.id = t.client_id
LEFT JOIN suppliers sp  ON sp.id = t.supplier_id
JOIN users cu      ON cu.id = t.created_by
LEFT JOIN users su ON su.id = t.sent_by
LEFT JOIN users ru ON ru.id = t.received_by
LEFT JOIN users xu ON xu.id = t.cancelled_by
LEFT JOIN users uu ON uu.id = t.unreceived_by
WHERE t.id = $1 AND t.deleted_at IS NULL`, id).Scan(&t.ID, &t.Direction, &t.FromWarehouseID, &t.FromWarehouseName, &t.FromWarehouseIsDeleted,
		&t.ToWarehouseID, &t.ToWarehouseName, &t.ToWarehouseIsDeleted,
		&t.TaskID, &t.TaskTitle, &t.TaskIsDeleted, &t.ClientID, &t.ClientName, &t.ClientIsDeleted, &t.SupplierID, &t.SupplierName, &t.SupplierIsDeleted,
		&t.Status, &t.Note, &t.CreatedBy, &t.CreatedByName,
		&t.SentBy, &t.SentByName, &t.SentAt,
		&t.ReceivedBy, &t.ReceivedByName, &t.ReceivedAt,
		&t.CancelledBy, &t.CancelledByName, &t.CancelledAt,
		&t.UnreceivedBy, &t.UnreceivedByName, &t.UnreceivedAt, &t.UnreceiveReason,
		&t.CreatedAt, &t.UpdatedAt, &t.Version, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Amount = numeric.Amount(amount)

	rows, err := q.Query(ctx, `
SELECT i.id::text, i.kind, i.product_id::text, p.deleted_at IS NOT NULL, p.name, p.unit,
       i.equipment_id::text, e.deleted_at IS NOT NULL,
       CASE WHEN e.id IS NULL THEN NULL
            ELSE concat_ws(' ', et.name, nullif(e.brand, ''), e.model) END,
       e.serial, c.company_name,
       i.qty_sent::text, i.qty_received::text,
       i.price::text, round(i.price * i.qty_sent, 2)::text
FROM transfer_items i
LEFT JOIN products p         ON p.id = i.product_id
LEFT JOIN equipment e        ON e.id = i.equipment_id
LEFT JOIN equipment_types et ON et.id = e.type_id
LEFT JOIN clients c          ON c.id = e.client_id
WHERE i.transfer_id = $1 ORDER BY i.position, i.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t.Items = []TransferItemResponse{}
	for rows.Next() {
		var it TransferItemResponse
		var sent, received, price, lineTotal *string
		if err := rows.Scan(&it.ID, &it.Kind, &it.ProductID, &it.ProductIsDeleted, &it.ProductName, &it.Unit,
			&it.EquipmentID, &it.EquipmentIsDeleted, &it.EquipmentName, &it.EquipmentSerial, &it.EquipmentClientName,
			&sent, &received, &price, &lineTotal); err != nil {
			return nil, err
		}
		it.QtySent, it.QtyReceived = amountPtr(sent), amountPtr(received)
		it.Price, it.LineTotal = amountPtr(price), amountPtr(lineTotal)
		t.Items = append(t.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	t.Warnings = []StockWarning{}
	return &t, nil
}

func amountPtr(s *string) *numeric.Amount {
	if s == nil {
		return nil
	}
	a := numeric.Amount(*s)
	return &a
}

// inTx — транзакция с откатом при ошибке; fn возвращает id перемещения для re-fetch.
func (r *Repo) inTx(ctx context.Context, fn func(tx pgx.Tx) ([]StockWarning, int64, error)) (*TransferResponse, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	warnings, id, err := fn(tx)
	if err != nil {
		return nil, mapPgErr(err)
	}
	t, err := getTransfer(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapPgErr(err)
	}
	if warnings != nil {
		t.Warnings = warnings
	}
	return t, nil
}

func mapPgErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "22003":
		return errOverflow
	case pgErr.Code == "23503" && pgErr.ConstraintName == "transfers_task_id_fkey":
		// заказ удалён между проверкой checkRefs и вставкой документа (гонка с DELETE заказа).
		return &fieldsError{Fields: []errorspkg.Field{{Name: "task_id", Message: msgNotFound}}}
	}
	return err
}

// Head — направление и заказ документа без блокировки (оба после создания не меняются):
// вход валидатора правки и проверки скоупа исполнителя.
func (r *Repo) Head(ctx context.Context, id int64) (direction string, taskID *int64, err error) {
	err = r.pool.QueryRow(ctx, `SELECT direction, task_id FROM transfers WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&direction, &taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, errNotFound
	}
	return direction, taskID, err
}

// Create — новое перемещение (requested) и, при send, сразу отправка (in_transit); при
// receive (только приём от клиента, DS-013) — ещё и приёмка, всё одной транзакцией. Единицы
// строк занимаются документом (DS-019). Lock Order (DS-048): документ → product_stock →
// equipment.
func (r *Repo) Create(ctx context.Context, in Input, userID string, send, receive bool) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		clientID, err := checkRefs(ctx, tx, in, nil, "", true)
		if err != nil {
			return nil, 0, err
		}
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, task_id, client_id, supplier_id, note, created_by)
VALUES ($1, $2::uuid, $3::uuid, $4, $5::uuid, $6::uuid, $7, $8::uuid) RETURNING id`,
			in.Direction, nullable(in.FromWarehouseID), nullable(in.ToWarehouseID), in.TaskID, clientID,
			nullable(in.SupplierID), in.Note, userID).Scan(&id); err != nil {
			return nil, 0, err
		}
		if err := insertItems(ctx, tx, id, in.Direction, in.Items); err != nil {
			return nil, 0, err
		}
		send = send || receive
		if send && sendsStock(in.Direction) {
			// остатки блокируются ДО единиц — порядок блокировок общий с Send.
			if err := lockPairs(ctx, tx, id, in.FromWarehouseID); err != nil {
				return nil, 0, err
			}
		}
		if err := claimEquipment(ctx, tx, id, equipmentIDs(in.Items)); err != nil {
			return nil, 0, deletedAsField(err, in.Items)
		}
		if !send {
			return nil, id, nil
		}
		doc := lockedTransfer{status: StatusRequested, direction: in.Direction, from: in.FromWarehouseID, to: in.ToWarehouseID}
		w, err := doSend(ctx, tx, id, doc, userID)
		if err != nil || !receive {
			return w, id, err
		}
		return w, id, doReceive(ctx, tx, id, doc, nil, userID)
	})
}

// Update — правка заявки (только requested): точки маршрута по направлению, примечание,
// строки целиком; снятые единицы освобождаются, добавленные занимаются (DS-019). Направление
// и заказ не меняются — in.Direction / in.TaskID заполнены из документа (Head).
func (r *Repo) Update(ctx context.Context, id int64, in Input, version int) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		doc, err := lockTransfer(ctx, tx, id, version, actionUpdate, StatusRequested)
		if err != nil {
			return nil, 0, err
		}
		existing, err := existingProductIDs(ctx, tx, id)
		if err != nil {
			return nil, 0, err
		}
		// заказ заявки мог быть мягко удалён — правка такой заявки разрешена (DS-007): заказ у
		// документа не меняется, запрет TASK_DELETED действует только на создание.
		if _, err := checkRefs(ctx, tx, in, existing, doc.supplier, false); err != nil {
			return nil, 0, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE transfers SET from_warehouse_id = $2::uuid, to_warehouse_id = $3::uuid, supplier_id = $4::uuid, note = $5,
       updated_at = now(), version = version + 1
WHERE id = $1`, id, nullable(in.FromWarehouseID), nullable(in.ToWarehouseID), nullable(in.SupplierID), in.Note); err != nil {
			return nil, 0, err
		}
		// единицы документа (прежние и новые) блокируются одним проходом в порядке id.
		prev, err := transferEquipmentIDs(ctx, tx, id)
		if err != nil {
			return nil, 0, err
		}
		next := equipmentIDs(in.Items)
		if _, err := lockEquipment(ctx, tx, append(prev, next...)); err != nil {
			return nil, 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE equipment SET active_transfer_id = NULL WHERE active_transfer_id = $1`, id); err != nil {
			return nil, 0, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM transfer_items WHERE transfer_id = $1`, id); err != nil {
			return nil, 0, err
		}
		if err := insertItems(ctx, tx, id, in.Direction, in.Items); err != nil {
			return nil, 0, err
		}
		return nil, id, deletedAsField(claimEquipment(ctx, tx, id, next), in.Items)
	})
}

// Send — requested → in_transit. Склад → склад: −qty_sent с источника (transfer_out);
// накладная «склад → клиент»: −qty_sent со склада отправления (client_out, transfers-sales
// DS-011); единицы из источника (склад отправления либо клиент) → «в пути»; приход от
// поставщика — только статус (DS-040). Заявка с мягко удалённым товаром не отправляется —
// deletedRefError (409 TRANSFER_HAS_DELETED_REFS, details.kind = product); отмена документа «В
// пути» с удалённым товаром по-прежнему проходит — возврат на источник (DS-042).
func (r *Repo) Send(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		doc, err := lockTransfer(ctx, tx, id, version, actionSend, StatusRequested)
		if err != nil {
			return nil, 0, err
		}
		// soft-delete-catalog DS-046 (распространён на отправку): товар заявки в удалённых —
		// отправка отклоняется до любых изменений: она списала бы остаток удалённого товара
		// (в минус — на момент удаления он нулевой). Порядок блокировок как в Unreceive:
		// документ → warehouses (FOR SHARE) → products (FOR SHARE) → product_stock → equipment.
		// Склад заявки в удалённых (soft-delete-warehouses-suppliers DS-047) — отказ тем же кодом,
		// details.kind = warehouse: страховка, по правилам такой заявки не бывает. Удалённый
		// поставщик отправке не мешает — он не держит ни остатка, ни местоположения.
		if err := checkLiveWarehouses(ctx, tx, doc.from, doc.to); err != nil {
			return nil, 0, err
		}
		if err := checkLiveProducts(ctx, tx, id); err != nil {
			return nil, 0, err
		}
		w, err := doSend(ctx, tx, id, doc, userID)
		return w, id, err
	})
}

// Receive — in_transit → received (эффекты по направлению — doReceive).
func (r *Repo) Receive(ctx context.Context, id int64, version int, in []ReceiveItemInput, userID string) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		doc, err := lockTransfer(ctx, tx, id, version, actionReceive, StatusInTransit)
		if err != nil {
			return nil, 0, err
		}
		return nil, id, doReceive(ctx, tx, id, doc, in, userID)
	})
}

// doReceive — приёмка документа «в пути» (строка transfers уже заблокирована):
//   - склад → склад: факт по товарным строкам, на получателе +qty_sent (transfer_in) и
//     −недостача (transfer_loss);
//   - приход от поставщика: +факт на складе назначения (supplier_in; факт 0 — без движения),
//     факт может превышать заказанное (DS-015);
//   - документы с клиентом: факта нет; товар накладной «склад → клиент» списан при
//     отправке — движений нет, qty_received остаётся NULL (transfers-sales DS-038).
//
// Единицы — целиком в точку назначения (склад либо клиент) и освобождаются (DS-037).
func doReceive(ctx context.Context, tx pgx.Tx, id int64, doc lockedTransfer, in []ReceiveItemInput, userID string) error {
	items, err := sentItems(ctx, tx, id)
	if err != nil {
		return err
	}
	facts, fields := resolveReceive(doc.direction, items, in)
	if len(fields) > 0 {
		return &fieldsError{Fields: fields}
	}
	if len(facts) > 0 {
		ids := make([]string, 0, len(facts))
		qtys := make([]string, 0, len(facts))
		for k, v := range facts {
			ids = append(ids, k)
			qtys = append(qtys, v)
		}
		if _, err := tx.Exec(ctx, `
UPDATE transfer_items i SET qty_received = u.q::numeric
FROM unnest($2::uuid[], $3::text[]) AS u(id, q)
WHERE i.transfer_id = $1 AND i.id = u.id AND i.kind = 'product'`, id, ids, qtys); err != nil {
			return err
		}
		if err := lockPairs(ctx, tx, id, doc.to); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
UPDATE product_stock ps SET stock = ps.stock + i.qty_received
FROM transfer_items i
WHERE i.transfer_id = $1 AND i.kind = 'product'
  AND ps.product_id = i.product_id AND ps.warehouse_id = $2::uuid`, id, doc.to); err != nil {
			return err
		}
		if doc.direction == DirSupplierToWarehouse {
			_, err = tx.Exec(ctx, `
INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
SELECT i.product_id, $2::uuid, i.qty_received, $3, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product' AND i.qty_received > 0`,
				id, doc.to, reasonSupplierIn, userID)
		} else {
			_, err = tx.Exec(ctx, `
INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
SELECT i.product_id, $2::uuid, i.qty_sent, $3, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product'
UNION ALL
SELECT i.product_id, $2::uuid, -(i.qty_sent - i.qty_received), $5, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product' AND i.qty_received < i.qty_sent`,
				id, doc.to, reasonIn, userID, reasonLoss)
		}
		if err != nil {
			return err
		}
	}
	// склад → клиент: doc.to пуст — единицы оседают «у клиента».
	if err := settleEquipment(ctx, tx, id, doc.to); err != nil {
		return err
	}
	// received_at — clock_timestamp(), не now() (время начала транзакции): порядок приёмок
	// документов с общей единицей обязан совпадать с порядком взятия блокировки её строки —
	// по нему отмена проведения ищет «более поздний» документ (transfers-revert DS-023).
	_, err = tx.Exec(ctx, `
UPDATE transfers SET status = $2, received_by = $3::uuid, received_at = clock_timestamp(), updated_at = now(), version = version + 1
WHERE id = $1`, id, StatusReceived, userID)
	return err
}

// Cancel — requested → cancelled (без движений, единицы освобождаются); in_transit →
// cancelled: склад → склад и накладная «склад → клиент» — +qty_sent на склад отправления
// (transfer_return); единицы — обратно в источник (склад отправления либо клиент); приход от
// поставщика — без движений (DS-040).
func (r *Repo) Cancel(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		st, err := lockTransfer(ctx, tx, id, version, actionCancel, StatusRequested, StatusInTransit)
		if err != nil {
			return nil, 0, err
		}
		if st.status == StatusInTransit {
			if sendsStock(st.direction) {
				if err := lockPairs(ctx, tx, id, st.from); err != nil {
					return nil, 0, err
				}
				if _, err := tx.Exec(ctx, `
UPDATE product_stock ps SET stock = ps.stock + i.qty_sent
FROM transfer_items i
WHERE i.transfer_id = $1 AND i.kind = 'product'
  AND ps.product_id = i.product_id AND ps.warehouse_id = $2::uuid`, id, st.from); err != nil {
					return nil, 0, err
				}
				if _, err := tx.Exec(ctx, `
INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
SELECT i.product_id, $2::uuid, i.qty_sent, $3, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product'`,
					id, st.from, reasonReturn, userID); err != nil {
					return nil, 0, err
				}
			}
			// приём от клиента: st.from пуст — единицы возвращаются «к клиенту».
			if err := settleEquipment(ctx, tx, id, st.from); err != nil {
				return nil, 0, err
			}
		} else {
			ids, err := transferEquipmentIDs(ctx, tx, id)
			if err != nil {
				return nil, 0, err
			}
			if _, err := lockEquipment(ctx, tx, ids); err != nil {
				return nil, 0, err
			}
			if _, err := tx.Exec(ctx, `UPDATE equipment SET active_transfer_id = NULL WHERE active_transfer_id = $1`, id); err != nil {
				return nil, 0, err
			}
		}
		_, err = tx.Exec(ctx, `
UPDATE transfers SET status = $2, cancelled_by = $3::uuid, cancelled_at = now(), updated_at = now(), version = version + 1
WHERE id = $1`, id, StatusCancelled, userID)
		return nil, id, err
	})
}

// receivesStock — приёмка документа приходует факт на склад назначения (а отмена проведения
// его снимает): склад → склад и приход от поставщика. Товар накладной «склад → клиент» списан
// при отправке, у приёма от клиента товарных строк нет.
func receivesStock(direction string) bool {
	return direction == DirWarehouseToWarehouse || direction == DirSupplierToWarehouse
}

// Unreceive — «Отменить проведение»: received → in_transit (transfers-revert DS-017).
//   - склад → склад и приход от поставщика: −факт приёмки на складе назначения
//     (transfer_unreceive; факт 0 — без движения); остаток может уйти в минус — это только
//     предупреждение (DS-020);
//   - документы с клиентом: движений нет (товар накладной списан при отправке и остаётся
//     списанным, пока документ «в пути»);
//   - единицы — снова «в пути» и заняты документом; запрещено, если единицу увёз более поздний
//     полученный документ либо она стоит в другом незавершённом (DS-023);
//   - факт по строкам и отметка «кто/когда принял» обнуляются; на документе — кто, когда и
//     почему отменил проведение (DS-044).
//
// Отмена проведения отклоняется, если товар документа мягко удалён (soft-delete-catalog DS-046):
// откат изменил бы остаток удалённого товара и вернул документ в «В пути» — deletedRefError
// (409 TRANSFER_HAS_DELETED_REFS, details.kind = product). Приём документа «В пути» с удалённым
// товаром по-прежнему проходит (DS-042).
//
// То же — при мягко удалённом складе документа (details.kind = warehouse,
// soft-delete-warehouses-suppliers DS-046).
//
// Lock Order общий: документ → warehouses (FOR SHARE) → products (FOR SHARE) → product_stock →
// equipment.
func (r *Repo) Unreceive(ctx context.Context, id int64, version int, reason, userID string) (*TransferResponse, error) {
	return r.inTx(ctx, func(tx pgx.Tx) ([]StockWarning, int64, error) {
		doc, err := lockTransfer(ctx, tx, id, version, actionUnreceive, StatusReceived)
		if err != nil {
			return nil, 0, err
		}
		// soft-delete-warehouses-suppliers DS-046: склад отправления либо назначения в удалённых —
		// откат отклоняется до любых изменений: он изменил бы остаток на удалённом складе и вернул
		// документ в «В пути», а с такими документами склад удалённым быть не может. Строки складов
		// держатся FOR SHARE до коммита — удаление склада ждёт откат и затем видит документ «В
		// пути». Удалённый поставщик отмене проведения не мешает.
		if err := checkLiveWarehouses(ctx, tx, doc.from, doc.to); err != nil {
			return nil, 0, err
		}
		// soft-delete-catalog DS-046: товар документа в удалённых — откат отклоняется до любых
		// изменений (остатки не тронуты, документ остаётся «Получено»).
		if err := checkLiveProducts(ctx, tx, id); err != nil {
			return nil, 0, err
		}
		warnings := []StockWarning{}
		if receivesStock(doc.direction) {
			if err := lockPairs(ctx, tx, id, doc.to); err != nil {
				return nil, 0, err
			}
			if _, err := tx.Exec(ctx, `
UPDATE product_stock ps SET stock = ps.stock - d.qty
FROM (SELECT i.product_id, sum(i.qty_received) AS qty FROM transfer_items i
      WHERE i.transfer_id = $1 AND i.kind = 'product' AND i.qty_received > 0
      GROUP BY i.product_id) d
WHERE ps.product_id = d.product_id AND ps.warehouse_id = $2::uuid`, id, doc.to); err != nil {
				return nil, 0, err
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
SELECT i.product_id, $2::uuid, -i.qty_received, $3, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product' AND i.qty_received > 0`,
				id, doc.to, reasonUnreceive, userID); err != nil {
				return nil, 0, err
			}
			// до обнуления факта: предупреждения — только по строкам с движением (факт > 0).
			if warnings, err = stockWarnings(ctx, tx, id, doc.to, true); err != nil {
				return nil, 0, err
			}
		}
		if err := unsettleEquipment(ctx, tx, id, doc.to); err != nil {
			return nil, 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE transfer_items SET qty_received = NULL WHERE transfer_id = $1`, id); err != nil {
			return nil, 0, err
		}
		_, err = tx.Exec(ctx, `
UPDATE transfers SET status = $2, received_by = NULL, received_at = NULL,
       unreceived_by = $3::uuid, unreceived_at = now(), unreceive_reason = $4,
       updated_at = now(), version = version + 1
WHERE id = $1`, id, StatusInTransit, userID, reason)
		return warnings, id, err
	})
}

// unsettleEquipment — обратное settleEquipment приёмки: единицы полученного документа снова
// «в пути» и заняты им. Под блокировкой строк единиц (порядок id) каждая обязана (DS-023):
//   - быть свободной — иначе она стоит в другом незавершённом документе;
//   - не входить в более поздний полученный документ. Порядок — по (received_at, id), а не по
//     текущему местоположению: единица, которую увезли и вернули на тот же склад, числится «на
//     месте», но откат обязан идти в обратном порядке. Называется САМЫЙ поздний документ — с
//     него откат и начинают;
//   - числиться там, куда её привёз документ (warehouseID; пустой — «у клиента»).
//
// Иначе unreceiveBlockedError по первой единице в порядке id; ничего не изменено.
//
// soft-delete-clients DS-041, DS-045: единица, которую документ «склад → клиент» привёз клиенту,
// свободна и у клиента — её (либо клиента целиком) могли мягко удалить. Отмена проведения
// вернула бы удалённую единицу «в путь» и заняла её незавершённым документом, поэтому
// удалённая единица — отказ deletedRefError (409 TRANSFER_HAS_DELETED_REFS): сначала
// восстановить её либо клиента. Признак читается под блокировкой строки (lockEquipment) и
// повторён в UPDATE: гонка «удалить ↔ отменить проведение» даёт ровно один успех.
func unsettleEquipment(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error {
	ids, err := transferEquipmentIDs(ctx, tx, transferID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	states, err := lockEquipment(ctx, tx, ids)
	if err != nil {
		return err
	}
	// читается ПОСЛЕ блокировки единиц: приёмка более позднего документа держит блокировку
	// единицы до своего коммита, поэтому здесь она уже видна (READ COMMITTED).
	later := map[string]int64{}
	rows, err := tx.Query(ctx, `
SELECT DISTINCT ON (i.equipment_id) i.equipment_id::text, t.id
FROM transfer_items i
JOIN transfers t  ON t.id = i.transfer_id
JOIN transfers me ON me.id = $1
WHERE i.kind = 'equipment' AND i.equipment_id = ANY($2::uuid[])
  AND t.id <> me.id AND t.status = $3
  AND (t.received_at, t.id) > (me.received_at, me.id)
ORDER BY i.equipment_id, t.received_at DESC, t.id DESC`, transferID, ids, StatusReceived)
	if err != nil {
		return err
	}
	for rows.Next() {
		var eq string
		var tid int64
		if err := rows.Scan(&eq, &tid); err != nil {
			rows.Close()
			return err
		}
		later[eq] = tid
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, eq := range ids { // ids отсортированы по id
		if states[eq].deleted {
			return &deletedRefError{kind: refKindEquipment, id: eq}
		}
	}
	for _, eq := range ids {
		st := states[eq]
		if st.activeID != nil {
			return &unreceiveBlockedError{equipmentID: eq, transferID: *st.activeID}
		}
		if tid, ok := later[eq]; ok {
			return &unreceiveBlockedError{equipmentID: eq, transferID: tid, later: true}
		}
		atDestination := st.location == locationAtWarehouse && st.warehouseID != nil && *st.warehouseID == warehouseID
		if warehouseID == "" {
			atDestination = st.location == locationAtClient
		}
		if !atDestination {
			return &unreceiveBlockedError{equipmentID: eq}
		}
	}
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET location_status = $3, warehouse_id = NULL, active_transfer_id = $1, updated_at = now()
WHERE id = ANY($2::uuid[]) AND deleted_at IS NULL AND active_transfer_id IS NULL`, transferID, ids, locationInTransit)
	if err != nil {
		return err
	}
	// под блокировкой строк недостижимо; страховка guard-UPDATE'а (как в settleEquipment).
	if int(tag.RowsAffected()) != len(ids) {
		return fmt.Errorf("transfers: unsettle equipment of transfer %d: %d of %d units updated", transferID, tag.RowsAffected(), len(ids))
	}
	return nil
}

// SoftDelete — МЯГКОЕ удаление документа, не влияющего на учёт (soft-delete-transfers §6.1,
// DS-018): только requested / cancelled. Одна транзакция: документ под FOR UPDATE (версия →
// 409, статус → 409, уже удалён → errNotFound), единицы заявки освобождаются под блокировкой
// строк equipment (порядок id — общий Lock Order «документ → equipment»), отметка «кто, когда,
// почему». Строки документа, stock_movements, остатки и местоположение единиц не трогаются
// (DS-008). Возвращает статус документа и момент удаления (UTC).
func (r *Repo) SoftDelete(ctx context.Context, id int64, version int, userID, reason string) (string, time.Time, error) {
	var at time.Time
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", at, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	doc, err := lockTransfer(ctx, tx, id, version, actionDelete, StatusRequested, StatusCancelled)
	if err != nil {
		return "", at, err
	}
	if doc.status == StatusRequested {
		ids, err := transferEquipmentIDs(ctx, tx, id)
		if err != nil {
			return "", at, err
		}
		if _, err := lockEquipment(ctx, tx, ids); err != nil {
			return "", at, err
		}
		if _, err := tx.Exec(ctx, `UPDATE equipment SET active_transfer_id = NULL WHERE active_transfer_id = $1`, id); err != nil {
			return "", at, err
		}
	}
	// условия версии, статуса и «ещё не удалён» повторены в UPDATE (DS-024): под блокировкой
	// строки они уже проверены, guard — страховка атомарности.
	err = tx.QueryRow(ctx, `
UPDATE transfers SET deleted_at = now(), deleted_by = $3::uuid, delete_reason = $4,
       updated_at = now(), version = version + 1
WHERE id = $1 AND version = $2 AND deleted_at IS NULL AND status IN ('requested', 'cancelled')
RETURNING deleted_at`, id, version, userID, reason).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", at, errNotFound
	}
	if err != nil {
		return "", at, err
	}
	return doc.status, at.UTC(), tx.Commit(ctx)
}

// Restore — восстановление мягко удалённого документа (soft-delete-transfers §6.2). Одна
// транзакция, Lock Order пакета «документ → заказ → equipment»:
//   - документ под FOR UPDATE: нет → errNotFound; не удалён → errNotDeleted (DS-024);
//   - у документа с заказом заказ блокируется FOR KEY SHARE (как checkRefs): смена клиента
//     заказа (board, LockTask FOR UPDATE) либо уже видна — клиент документа не совпал →
//     errClientMismatch, либо ждёт восстановление и после него видит действующий документ
//     (409 TASK_CLIENT_LOCKED_BY_TRANSFERS) — DS-040;
//   - у заявки с заказом каждая единица обязана по-прежнему входить в оборудование заказа
//     (её могли убрать из заказа, пока документ был удалён) — иначе notInTaskError, документ
//     остаётся удалённым;
//   - у заявки единицы занимаются снова под блокировкой строк equipment (порядок id); единица
//     в другом незавершённом документе → equipmentBusyError, документ остаётся удалённым
//     (DS-018); единица мягко удалена (сама либо вместе с клиентом) → deletedRefError, документ
//     остаётся удалённым (soft-delete-clients DS-045); отменённый документ не проверяется;
//   - у заявки товар строки мягко удалён → deletedRefError (kind = product), документ остаётся
//     удалённым (soft-delete-catalog DS-045); проверка — под FOR SHARE строк products;
//   - у заявки склад отправления либо назначения мягко удалён → deletedRefError (kind =
//     warehouse), поставщик мягко удалён → deletedRefError (kind = supplier), документ остаётся
//     удалённым (soft-delete-warehouses-suppliers DS-045); склады — под FOR SHARE строк
//     warehouses, до товаров;
//   - отметка удаления стирается (DS-030), version + 1.
//
// Остатки и местоположение единиц не меняются (DS-008). Возвращает статус документа.
func (r *Repo) Restore(ctx context.Context, id int64) (string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var (
		status           string
		deleted          bool
		taskID           *int64
		clientID         *string
		from, to, supply string
	)
	err = tx.QueryRow(ctx, `
SELECT status, deleted_at IS NOT NULL, task_id, client_id::text,
       coalesce(from_warehouse_id::text, ''), coalesce(to_warehouse_id::text, ''), coalesce(supplier_id::text, '')
FROM transfers WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &deleted, &taskID, &clientID, &from, &to, &supply)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	if err != nil {
		return "", err
	}
	if !deleted {
		return "", errNotDeleted
	}
	if taskID != nil {
		var taskClient string
		if err := tx.QueryRow(ctx, `SELECT client_id::text FROM tasks WHERE id = $1 FOR KEY SHARE`, *taskID).Scan(&taskClient); err != nil {
			return "", err
		}
		if clientID == nil || *clientID != taskClient {
			return "", errClientMismatch
		}
	}
	if status == StatusRequested {
		// soft-delete-warehouses-suppliers DS-045: заявка не восстанавливается, пока её склад или
		// поставщик в удалённых — иначе она привезла бы товар либо оборудование на удалённый склад.
		// Строки складов — FOR SHARE до коммита: удаление склада ждёт восстановление и затем видит
		// действующую заявку (409 WAREHOUSE_IN_USE). Поставщик читается без блокировки: его
		// удаление сразу после восстановления заявки разрешено и так (DS-014). Отменённый документ
		// не проверяется.
		if err := checkLiveWarehouses(ctx, tx, from, to); err != nil {
			return "", err
		}
		if supply != "" {
			var supplierDeleted bool
			if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM suppliers WHERE id = $1::uuid`, supply).
				Scan(&supplierDeleted); err != nil {
				return "", err
			}
			if supplierDeleted {
				return "", &deletedRefError{kind: refKindSupplier, id: supply}
			}
		}
		// soft-delete-catalog DS-045: заявка не восстанавливается, пока её товар в удалённых —
		// иначе её отправка и приём двигали бы остаток удалённого товара. Отменённый документ
		// не проверяется. Товары — до единиц (порядок «товар → equipment»).
		if err := checkLiveProducts(ctx, tx, id); err != nil {
			return "", err
		}
		ids, err := transferEquipmentIDs(ctx, tx, id)
		if err != nil {
			return "", err
		}
		if taskID != nil && len(ids) > 0 {
			// та же выборка, что в checkRefs (DS-035), под уже взятой FOR KEY SHARE заказа:
			// снятие единицы с заказа (board, LockTask FOR UPDATE) либо уже видно, либо ждёт
			// восстановление и после него получает 409 EQUIPMENT_IN_TRANSFER.
			inTask, err := idSet(ctx, tx, `
SELECT te.equipment_id::text FROM task_equipment te
JOIN equipment e ON e.id = te.equipment_id
WHERE te.task_id = $2 AND e.client_id = $3::uuid AND te.equipment_id = ANY($1::uuid[])`, ids, *taskID, *clientID)
			if err != nil {
				return "", err
			}
			for _, eq := range ids { // ids отсортированы по id
				if !inTask[eq] {
					return "", &notInTaskError{equipmentID: eq}
				}
			}
		}
		if err := claimEquipment(ctx, tx, id, ids); err != nil {
			return "", err
		}
	}
	tag, err := tx.Exec(ctx, `
UPDATE transfers SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
       updated_at = now(), version = version + 1
WHERE id = $1 AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", errNotDeleted
	}
	return status, tx.Commit(ctx)
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка документа автотестами (soft-delete-transfers §6.3,
// DS-038): документ в requested / cancelled, действующий или мягко удалённый. Строки уходят
// каскадом FK, stock_movements.transfer_id и equipment.active_transfer_id (единицы заявки) —
// SET NULL. Проведённый документ → statusError (409), документа нет → errNotFound.
func (r *Repo) Purge(ctx context.Context, id int64) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM transfers WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if status != StatusRequested && status != StatusCancelled {
		return &statusError{action: actionDelete, status: status}
	}
	// единицы заявки блокируются явно в порядке id (освобождает их FK SET NULL) — порядок
	// блокировок общий с остальными операциями пакета.
	ids, err := transferEquipmentIDs(ctx, tx, id)
	if err != nil {
		return err
	}
	if _, err := lockEquipment(ctx, tx, ids); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM transfers WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockedTransfer — шапка документа под блокировкой: from / to / supplier — пустая строка,
// если точка маршрута направлению не положена (NULL).
type lockedTransfer struct {
	status    string
	direction string
	from, to  string
	supplier  string
}

// lockTransfer — FOR UPDATE строки перемещения (первая блокировка транзакции, D50: затем
// product_stock, затем equipment); version сверяется ДО статуса — устаревшая форма получает
// 409 version. Мягко удалённый документ для мутаций не существует — errNotFound (DS-026).
func lockTransfer(ctx context.Context, tx pgx.Tx, id int64, version int, action string, allowed ...string) (lockedTransfer, error) {
	var st lockedTransfer
	var cur int
	err := tx.QueryRow(ctx, `
SELECT status, direction, coalesce(from_warehouse_id::text, ''), coalesce(to_warehouse_id::text, ''),
       coalesce(supplier_id::text, ''), version
FROM transfers WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).
		Scan(&st.status, &st.direction, &st.from, &st.to, &st.supplier, &cur)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, errNotFound
	}
	if err != nil {
		return st, err
	}
	if cur != version {
		return st, errVersion
	}
	for _, a := range allowed {
		if st.status == a {
			return st, nil
		}
	}
	return st, &statusError{action: action, status: st.status}
}

// lockPairs — D34 для перемещений: отсутствующие строки product_stock (товар×склад)
// создаются ON CONFLICT DO NOTHING, затем блокируются FOR UPDATE в порядке
// (product_id, warehouse_id). Только товарные строки.
func lockPairs(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO product_stock (product_id, warehouse_id)
SELECT i.product_id, $2::uuid FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product'
ORDER BY i.product_id
ON CONFLICT DO NOTHING`, transferID, warehouseID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
SELECT ps.product_id FROM product_stock ps
JOIN transfer_items i ON i.product_id = ps.product_id AND i.transfer_id = $1
WHERE ps.warehouse_id = $2::uuid
ORDER BY ps.product_id, ps.warehouse_id FOR UPDATE OF ps`, transferID, warehouseID)
	if err != nil {
		return err
	}
	rows.Close()
	return rows.Err()
}

// equipmentState — местоположение и занятие единицы под блокировкой строки.
type equipmentState struct {
	location    string
	warehouseID *string
	activeID    *int64
	// deleted — единица мягко удалена (soft-delete-clients DS-041): занять её документом нельзя.
	deleted bool
}

// lockEquipment — блокировка строк equipment в порядке id (последняя ступень Lock Order).
// FOR NO KEY UPDATE, не FOR UPDATE: вставка строки документа уже держит FOR KEY SHARE на
// единице (FK transfer_items.equipment_id) — два параллельных создания с одной единицей на
// FOR UPDATE взаимно заблокировались бы (40P01), а так второй ждёт первого и получает 409.
func lockEquipment(ctx context.Context, tx pgx.Tx, ids []string) (map[string]equipmentState, error) {
	out := map[string]equipmentState{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `
SELECT id::text, location_status, warehouse_id::text, active_transfer_id, deleted_at IS NOT NULL
FROM equipment WHERE id = ANY($1::uuid[]) ORDER BY id FOR NO KEY UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var st equipmentState
		if err := rows.Scan(&id, &st.location, &st.warehouseID, &st.activeID, &st.deleted); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// claimEquipment — занятие единиц документом (DS-019, DS-041): guard-UPDATE
// active_transfer_id IS NULL; единица занята другим документом → equipmentBusyError с его
// номером. ids — в порядке строк запроса; ошибка — по первой занятой в порядке id.
//
// soft-delete-clients DS-041, DS-045: мягко удалённую единицу занять нельзя — deletedRefError.
// Признак читается под блокировкой строки equipment (lockEquipment): удаление единицы и
// клиента пишет ту же строку, поэтому гонка «удалить ↔ занять» даёт ровно один успех.
func claimEquipment(ctx context.Context, tx pgx.Tx, transferID int64, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	states, err := lockEquipment(ctx, tx, ids)
	if err != nil {
		return err
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for _, id := range sorted {
		if states[id].deleted {
			return &deletedRefError{kind: refKindEquipment, id: id}
		}
	}
	for _, id := range sorted {
		if st := states[id]; st.activeID != nil && *st.activeID != transferID {
			return &equipmentBusyError{equipmentID: id, transferID: *st.activeID}
		}
	}
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET active_transfer_id = $1
WHERE id = ANY($2::uuid[]) AND active_transfer_id IS NULL AND deleted_at IS NULL`, transferID, ids)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(ids) {
		// под блокировкой строк недостижимо; страховка guard-UPDATE'а. Номер мешающего
		// документа здесь неизвестен (transferID = 0) — mapErr отдаёт текст без номера.
		return &equipmentBusyError{}
	}
	return nil
}

// settleEquipment — единицы документа «в пути» оседают в точке warehouseID (приёмка — точка
// назначения, отмена — источник) и освобождаются. Пустой warehouseID — точка «клиент»:
// единица становится at_client, склад пуст (DS-013).
func settleEquipment(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error {
	ids, err := transferEquipmentIDs(ctx, tx, transferID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := lockEquipment(ctx, tx, ids); err != nil {
		return err
	}
	location := locationAtWarehouse
	if warehouseID == "" {
		location = locationAtClient
	}
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET location_status = $3, warehouse_id = $2::uuid, active_transfer_id = NULL, updated_at = now()
WHERE active_transfer_id = $1 AND location_status = $4`, transferID, nullable(warehouseID), location, locationInTransit)
	if err != nil {
		return err
	}
	// каждая единица документа «в пути» обязана осесть: расхождение — нарушенный инвариант
	// (единица не «в пути» либо занята не этим документом) → ошибка и откат транзакции.
	if int(tag.RowsAffected()) != len(ids) {
		return fmt.Errorf("transfers: settle equipment of transfer %d: %d of %d units updated", transferID, tag.RowsAffected(), len(ids))
	}
	return nil
}

// sendsStock — отправка документа списывает товар со склада отправления (а отмена «в пути»
// возвращает): склад → склад и накладная «склад → клиент» (transfers-sales DS-011).
func sendsStock(direction string) bool {
	return direction == DirWarehouseToWarehouse || direction == DirWarehouseToClient
}

// doSend — отправка заявки (строка transfers уже заблокирована) + статус in_transit.
// Склад → склад (transfer_out) и накладная «склад → клиент» (client_out): списание с
// источника, возвращает предупреждения по его остаткам
// (отрицательный/на минимуме — разрешено, D17/D18). Приход от поставщика: движений нет —
// товар ещё не наш (DS-040). Каждая единица обязана числиться в источнике документа: на
// складе отправления (DS-036) либо, для приёма от клиента, «у клиента» (DS-013) — иначе
// notAtSourceError.
func doSend(ctx context.Context, tx pgx.Tx, id int64, doc lockedTransfer, userID string) ([]StockWarning, error) {
	from := doc.from
	moveStock := sendsStock(doc.direction)
	reason := reasonOut
	if doc.direction == DirWarehouseToClient {
		reason = reasonClientOut
	}
	if moveStock {
		if err := lockPairs(ctx, tx, id, from); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
UPDATE product_stock ps SET stock = ps.stock - i.qty_sent
FROM transfer_items i
WHERE i.transfer_id = $1 AND i.kind = 'product'
  AND ps.product_id = i.product_id AND ps.warehouse_id = $2::uuid`, id, from); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
SELECT i.product_id, $2::uuid, -i.qty_sent, $3, $4::uuid, $1
FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product'`,
			id, from, reason, userID); err != nil {
			return nil, err
		}
	}

	eqIDs, err := transferEquipmentIDs(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	states, err := lockEquipment(ctx, tx, eqIDs)
	if err != nil {
		return nil, err
	}
	fromClient := doc.direction == DirClientToWarehouse
	for _, eq := range eqIDs { // eqIDs отсортированы по id
		st := states[eq]
		// страховка DS-041: удалённая единица в заявке не бывает (удаление требует свободной
		// единицы, занятие — действующей); отправить её нельзя в любом случае.
		if st.deleted {
			return nil, &deletedRefError{kind: refKindEquipment, id: eq}
		}
		atSource := st.location == locationAtWarehouse && st.warehouseID != nil && *st.warehouseID == from
		if fromClient {
			atSource = st.location == locationAtClient
		}
		if !atSource || st.activeID == nil || *st.activeID != id {
			return nil, &notAtSourceError{equipmentID: eq, fromClient: fromClient}
		}
	}
	if len(eqIDs) > 0 {
		if _, err := tx.Exec(ctx, `
UPDATE equipment SET location_status = $2, warehouse_id = NULL, updated_at = now()
WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, eqIDs, locationInTransit); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
UPDATE transfers SET status = $2, sent_by = $3::uuid, sent_at = now(), updated_at = now(), version = version + 1
WHERE id = $1`, id, StatusInTransit, userID); err != nil {
		return nil, err
	}
	if !moveStock {
		return []StockWarning{}, nil
	}
	return stockWarnings(ctx, tx, id, from, false)
}

// stockWarnings — остатки товаров документа на складе warehouseID, ставшие отрицательными либо
// на/ниже минимума (после списания отправкой или отмены проведения) — только предупреждение.
// receivedOnly — отмена проведения: строки с фактом 0 остаток не двигали и в предупреждения не
// входят (вызывается ДО обнуления qty_received).
func stockWarnings(ctx context.Context, tx pgx.Tx, id int64, warehouseID string, receivedOnly bool) ([]StockWarning, error) {
	out := []StockWarning{}
	rows, err := tx.Query(ctx, `
SELECT p.id::text, w.id::text, w.name, p.name, ps.stock::text, ps.min_stock::text,
       CASE WHEN ps.stock < 0 THEN 'negative' ELSE 'below_min' END
FROM transfer_items i
JOIN product_stock ps ON ps.product_id = i.product_id AND ps.warehouse_id = $2::uuid
JOIN products p ON p.id = i.product_id
JOIN warehouses w ON w.id = ps.warehouse_id
WHERE i.transfer_id = $1 AND ps.stock <= ps.min_stock
  AND (NOT $3::boolean OR i.qty_received > 0)
ORDER BY p.name, p.id`, id, warehouseID, receivedOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var w StockWarning
		var stock, minStock string
		if err := rows.Scan(&w.ProductID, &w.WarehouseID, &w.WarehouseName, &w.Name, &stock, &minStock, &w.Kind); err != nil {
			return nil, err
		}
		w.Stock, w.MinStock = numeric.Amount(stock), numeric.Amount(minStock)
		out = append(out, w)
	}
	return out, rows.Err()
}

// insertItems — строки документа. Товарная строка накладной «склад → клиент» несёт цену
// продажи: не передана → products.price, прочитанная в этой транзакции (DS-039); в остальных
// направлениях и у строк-единиц цена NULL.
func insertItems(ctx context.Context, tx pgx.Tx, id int64, direction string, items []ItemInput) error {
	for i, it := range items {
		var err error
		switch {
		case it.Kind == KindEquipment:
			_, err = tx.Exec(ctx, `
INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2::uuid, $3)`,
				id, it.EquipmentID, i+1)
		case pricedDirection(direction):
			var price *string
			if it.Price != nil {
				v := it.Price.String()
				price = &v
			}
			_, err = tx.Exec(ctx, `
INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, price)
VALUES ($1, 'product', $2::uuid, $3, $4::numeric,
        coalesce($5::numeric, (SELECT p.price FROM products p WHERE p.id = $2::uuid)))`,
				id, it.ProductID, i+1, it.QtySent.String(), price)
		default:
			_, err = tx.Exec(ctx, `
INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent) VALUES ($1, 'product', $2::uuid, $3, $4::numeric)`,
				id, it.ProductID, i+1, it.QtySent.String())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// equipmentIDs — id единиц из строк запроса (порядок строк).
func equipmentIDs(items []ItemInput) []string {
	var out []string
	for _, it := range items {
		if it.Kind == KindEquipment {
			out = append(out, it.EquipmentID)
		}
	}
	return out
}

// transferEquipmentIDs — id единиц строк документа, по возрастанию id.
func transferEquipmentIDs(ctx context.Context, tx pgx.Tx, id int64) ([]string, error) {
	rows, err := tx.Query(ctx, `
SELECT equipment_id::text FROM transfer_items
WHERE transfer_id = $1 AND kind = 'equipment' ORDER BY equipment_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var eq string
		if err := rows.Scan(&eq); err != nil {
			return nil, err
		}
		out = append(out, eq)
	}
	return out, rows.Err()
}

func sentItems(ctx context.Context, tx pgx.Tx, id int64) ([]sentItem, error) {
	rows, err := tx.Query(ctx, `
SELECT id::text, kind, coalesce(qty_sent::text, '') FROM transfer_items WHERE transfer_id = $1 ORDER BY position, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sentItem
	for rows.Next() {
		var it sentItem
		if err := rows.Scan(&it.ID, &it.Kind, &it.QtySent); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func existingProductIDs(ctx context.Context, tx pgx.Tx, id int64) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `
SELECT product_id::text FROM transfer_items WHERE transfer_id = $1 AND kind = 'product'`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		out[pid] = true
	}
	return out, rows.Err()
}

// checkLiveProducts — товары строк документа transferID не должны быть мягко удалены
// (soft-delete-catalog DS-045, DS-046). Строки products берутся FOR SHARE по возрастанию id и
// держатся до коммита: удаление товара (UPDATE строки) либо уже видно → deletedRefError по
// первому удалённому товару в порядке id, либо ждёт и после коммита видит действующий документ
// (и остаток, если он появился). Вызывается ДО блокировки строк product_stock (порядок «товар →
// product_stock» общий с удалением товара).
func checkLiveProducts(ctx context.Context, tx pgx.Tx, transferID int64) error {
	rows, err := tx.Query(ctx, `
SELECT p.id::text, p.deleted_at IS NOT NULL FROM products p
WHERE p.id IN (SELECT i.product_id FROM transfer_items i WHERE i.transfer_id = $1 AND i.kind = 'product')
ORDER BY p.id FOR SHARE OF p`, transferID)
	if err != nil {
		return err
	}
	defer rows.Close()
	deletedID := ""
	for rows.Next() {
		var id string
		var deleted bool
		if err := rows.Scan(&id, &deleted); err != nil {
			return err
		}
		if deleted && deletedID == "" {
			deletedID = id
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if deletedID != "" {
		return &deletedRefError{kind: refKindProduct, id: deletedID}
	}
	return nil
}

// productFlag — состояние товара для проверки строки документа.
type productFlag struct{ active, deleted bool }

// productFlags — id → {is_active, удалён} товаров строк документа. Строки products берутся FOR
// SHARE по возрастанию id до конца транзакции (и до строк product_stock): удаление товара либо
// уже видно (новая строка → not_found), либо ждёт создание/правку документа.
func productFlags(ctx context.Context, tx pgx.Tx, ids []string) (map[string]productFlag, error) {
	out := map[string]productFlag{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT id::text, is_active, deleted_at IS NOT NULL FROM products
WHERE id = ANY($1::uuid[]) ORDER BY id FOR SHARE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var f productFlag
		if err := rows.Scan(&id, &f.active, &f.deleted); err != nil {
			return nil, err
		}
		out[id] = f
	}
	return out, rows.Err()
}

// checkLiveWarehouses — склады документа (отправления from, назначения to; "" — точки нет) не
// должны быть мягко удалены (soft-delete-warehouses-suppliers DS-045, DS-046, DS-047). Строки
// warehouses берутся FOR SHARE по возрастанию id и держатся до коммита: удаление склада (FOR
// UPDATE тех же строк в том же порядке) либо уже видно → deletedRefError (сначала склад
// отправления), либо ждёт и после коммита видит документ и изменённый им остаток. Вызывается
// сразу после блокировки документа — ДО строк products и product_stock.
func checkLiveWarehouses(ctx context.Context, tx pgx.Tx, from, to string) error {
	var ids []string
	for _, id := range []string{from, to} {
		if id != "" {
			ids = append(ids, id)
		}
	}
	flags, err := refFlags(ctx, tx, "warehouses", ids, true)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if flags[id].deleted {
			return &deletedRefError{kind: refKindWarehouse, id: id}
		}
	}
	return nil
}

// refFlag — состояние точки маршрута: есть, активна, мягко удалена.
type refFlag struct{ active, deleted bool }

// refFlags — id → {is_active, удалён} для таблицы точек маршрута (warehouses, suppliers). share
// — строки берутся FOR SHARE по возрастанию id до конца транзакции (склады: удаление склада
// блокирует их FOR UPDATE в том же порядке и либо уже видно, либо ждёт документ).
func refFlags(ctx context.Context, tx pgx.Tx, table string, ids []string, share bool) (map[string]refFlag, error) {
	out := map[string]refFlag{}
	if len(ids) == 0 {
		return out, nil
	}
	lock := ""
	if share {
		lock = " FOR SHARE"
	}
	rows, err := tx.Query(ctx, `SELECT id::text, is_active, deleted_at IS NOT NULL FROM `+table+
		` WHERE id = ANY($1::uuid[]) ORDER BY id`+lock, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var f refFlag
		if err := rows.Scan(&id, &f.active, &f.deleted); err != nil {
			return nil, err
		}
		out[id] = f
	}
	return out, rows.Err()
}

// checkRefs — точки маршрута и строки документа существуют и допустимы:
//   - склады направления существуют, не удалены и активны (мягко удалённый — not_found,
//     soft-delete-warehouses-suppliers DS-014);
//   - поставщик (приход от поставщика) существует, не удалён и активен; архивный либо мягко
//     удалённый, уже стоящий в документе (grandSupplier), допустим при пересохранении (TS-020;
//     удалённый поставщик не мешает доведению начатого документа, DS-047); удалённый в новом
//     документе либо при смене — not_found;
//   - заказ (документы с клиентом) существует — возвращается его клиент (DS-012); при
//     liveTaskOnly (создание документа) заказ не должен быть мягко удалён — errTaskDeleted
//     (soft-delete-transfers DS-007);
//   - товары существуют, не удалены и активны (архивный либо мягко удалённый, уже бывший в
//     заявке, — допустим при пересохранении, D52; удалённый в НОВОЙ строке → not_found,
//     soft-delete-catalog DS-014);
//   - единицы существуют; в документе с клиентом — входят в оборудование заказа и
//     принадлежат его клиенту, иначе not_in_task (DS-035).
func checkRefs(ctx context.Context, tx pgx.Tx, in Input, grandfathered map[string]bool, grandSupplier string, liveTaskOnly bool) (*string, error) {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	points := []struct{ name, id string }{}
	var whIDs []string
	for _, w := range []struct{ name, id string }{
		{"from_warehouse_id", in.FromWarehouseID}, {"to_warehouse_id", in.ToWarehouseID},
	} {
		if w.id != "" {
			points = append(points, w)
			whIDs = append(whIDs, w.id)
		}
	}
	// Склады — FOR SHARE до коммита, первой блокировкой после документа: удаление склада либо
	// уже видно (not_found), либо ждёт документ и затем отказывает 409 WAREHOUSE_IN_USE.
	warehouses, err := refFlags(ctx, tx, "warehouses", whIDs, true)
	if err != nil {
		return nil, err
	}
	for _, w := range points {
		switch fl, ok := warehouses[w.id]; {
		case !ok || fl.deleted:
			add(w.name, msgNotFound)
		case !fl.active:
			add(w.name, msgArchived)
		}
	}

	if in.SupplierID != "" {
		suppliers, err := refFlags(ctx, tx, "suppliers", []string{in.SupplierID}, false)
		if err != nil {
			return nil, err
		}
		grand := in.SupplierID == grandSupplier
		switch fl, ok := suppliers[in.SupplierID]; {
		case !ok || (fl.deleted && !grand):
			add("supplier_id", msgNotFound)
		case !fl.active && !grand:
			add("supplier_id", msgArchived)
		}
	}

	var clientID *string
	if in.TaskID != nil {
		var cid string
		var taskDeleted bool
		// FOR KEY SHARE: заказ блокируется ДО чтения task_equipment и занятия единиц и держится
		// до коммита. Снятие единицы с заказа и смена клиента заказа (board, LockTask FOR UPDATE)
		// ждут документ и после него видят занятую единицу (409); если они успели раньше —
		// следующий запрос (READ COMMITTED) уже видит их результат: not_in_task / новый клиент
		// (DS-035, DS-026). Порядок блокировок общий с board: заказ → equipment.
		// deleted_at читается под той же блокировкой: мягкое удаление заказа (FOR UPDATE) ждёт
		// создание документа, а успевшее раньше — уже видно (TASK_DELETED, DS-007).
		err := tx.QueryRow(ctx, `SELECT client_id::text, deleted_at IS NOT NULL FROM tasks WHERE id = $1 FOR KEY SHARE`, *in.TaskID).
			Scan(&cid, &taskDeleted)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			add("task_id", msgNotFound)
		case err != nil:
			return nil, err
		case taskDeleted && liveTaskOnly:
			return nil, errTaskDeleted
		default:
			clientID = &cid
		}
	}

	var productIDs []string
	for _, it := range in.Items {
		if it.Kind == KindProduct {
			productIDs = append(productIDs, it.ProductID)
		}
	}
	products, err := productFlags(ctx, tx, productIDs)
	if err != nil {
		return nil, err
	}
	eqIDs := equipmentIDs(in.Items)
	// мягко удалённая единица для новых строк не существует (soft-delete-clients DS-014).
	// Инвариант DS-041 «удалённая единица не стоит в незавершённом документе» держат: удаление
	// единицы и клиента (только свободная единица у клиента) и все пути, которые занимают
	// единицу документом, — claimEquipment (создание, правка, восстановление заявки) и
	// unsettleEquipment (отмена проведения): оба отказывают по удалённой единице.
	known, err := idSet(ctx, tx, `SELECT id::text FROM equipment WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, eqIDs)
	if err != nil {
		return nil, err
	}
	// inTask == nil — проверка не применяется (не документ с клиентом либо заказ не найден).
	var inTask map[string]bool
	if clientID != nil {
		inTask, err = idSet(ctx, tx, `
SELECT te.equipment_id::text FROM task_equipment te
JOIN equipment e ON e.id = te.equipment_id
WHERE te.task_id = $2 AND e.client_id = $3::uuid AND te.equipment_id = ANY($1::uuid[])`, eqIDs, *in.TaskID, *clientID)
		if err != nil {
			return nil, err
		}
	}
	for i, it := range in.Items {
		switch it.Kind {
		case KindProduct:
			switch p, ok := products[it.ProductID]; {
			case !ok, p.deleted && !grandfathered[it.ProductID]:
				add(fmt.Sprintf("items[%d].product_id", i), msgNotFound)
			case !p.active && !grandfathered[it.ProductID]:
				add(fmt.Sprintf("items[%d].product_id", i), msgArchived)
			}
		case KindEquipment:
			switch {
			case !known[it.EquipmentID]:
				add(fmt.Sprintf("items[%d].equipment_id", i), msgNotFound)
			case inTask != nil && !inTask[it.EquipmentID]:
				add(fmt.Sprintf("items[%d].equipment_id", i), msgNotInTask)
			}
		}
	}
	if len(f) > 0 {
		return nil, &fieldsError{Fields: f}
	}
	return clientID, nil
}

// deletedAsField — создание и правка документа: единица строки мягко удалена между проверкой
// checkRefs и занятием (гонка с удалением единицы либо клиента) → та же ошибка поля, что и до
// гонки: items[i].equipment_id: not_found (soft-delete-clients DS-014).
func deletedAsField(err error, items []ItemInput) error {
	var dr *deletedRefError
	if !errors.As(err, &dr) || dr.kind != refKindEquipment {
		return err
	}
	for i, it := range items {
		if it.Kind == KindEquipment && it.EquipmentID == dr.id {
			return &fieldsError{Fields: []errorspkg.Field{{Name: fmt.Sprintf("items[%d].equipment_id", i), Message: msgNotFound}}}
		}
	}
	return err
}

// idSet — множество id (text) из запроса с первым параметром-массивом ids; пустой ids —
// пустое множество без запроса.
func idSet(ctx context.Context, tx pgx.Tx, sql string, ids []string, args ...any) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, sql, append([]any{ids}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
