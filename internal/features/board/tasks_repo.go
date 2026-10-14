package board

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Task — repo-модель заказа. LockTask/Move/Update/Insert заполняют базовые поля;
// ClientName/AssigneeName/Subtasks* заполняет только Get/List (JOIN'ы, design-sql.md
// §Board List Query) — «Re-fetch-after-write»: ответ клиенту всегда собирается из Get()
// ПОСЛЕ коммита транзакции, не из RETURNING строки записи.
type Task struct {
	ID         int64
	ColumnID   string
	ClientID   string
	ClientName string
	// ClientIsDeleted — клиент заказа мягко удалён (soft-delete-clients DS-014); Get/List.
	ClientIsDeleted bool
	Title           string
	AssigneeID      *string
	AssigneeName    *string
	DueDate         *string // YYYY-MM-DD
	IsPaid          bool
	PaidAt          *time.Time
	Position        string
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	SubtasksTotal   int
	SubtasksDone    int
	SubtasksLate    int
	// DescriptionHTML/TotalAmount/LastLog — фаза 6 (design-sql.md §Totals Expression/
	// §Last Log Expression); DescriptionHTML заполняют также LockTask (текущее значение
	// нужно TaskService.Update, когда description в теле — nil, «не менять»).
	DescriptionHTML string
	TotalAmount     numeric.Amount
	// TransfersAmount — «Продано по накладным» (transfers-sales DS-014); считает только Get.
	TransfersAmount numeric.Amount
	LastLog         *TaskLastLog
	// HasInTransit — оборудование заказа в пути (DS-009); заполняет только Get.
	HasInTransit bool
	// ExecutorUnpaidDone — выполненные услуги с исполнителем без выплаты (D68); только Get.
	ExecutorUnpaidDone int
}

// executorUnpaidDoneSQL — D68: скаляр-фрагмент метки доски, коррелирован с t.id (алиас es —
// во избежание конфликта с внешними алиасами Get/List).
const executorUnpaidDoneSQL = `
	(SELECT count(*) FROM subtasks es WHERE es.task_id = t.id AND es.status = 'done'
	        AND es.assignee_id IS NOT NULL AND NOT es.is_executor_paid)`

// columnFlags — колонка под FOR SHARE (design-sql.md §Lock Target Column Query).
type columnFlags struct {
	ID            string
	IsFinal       bool
	ConsumesStock bool
}

// taskRow — параметры записи Insert/Update (design-go-layers.md §Repository Interface:
// `in taskRow`).
type taskRow struct {
	Title           string
	ClientID        string
	ColumnID        string
	AssigneeID      *string
	DueDate         *string
	IsPaid          bool
	Position        string
	DescriptionHTML string
}

type TaskRepo struct {
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo { return &TaskRepo{pool: pool} }

// taskWriteReturning — общий список колонок RETURNING для Insert/Update/Move (без JOIN —
// денормализованные поля добавляет только Get/List).
const taskWriteReturning = `id, column_id, client_id, title, assignee_id, due_date::text, is_paid, paid_at, position::text, version, created_at, updated_at`

func scanTaskRow(row pgx.Row, t *Task) error {
	return row.Scan(&t.ID, &t.ColumnID, &t.ClientID, &t.Title, &t.AssigneeID, &t.DueDate,
		&t.IsPaid, &t.PaidAt, &t.Position, &t.Version, &t.CreatedAt, &t.UpdatedAt)
}

// TaskColumnID — design-sql.md §Task Column Id Query: первый шаг lockRowsScope (card_
// service.go), читается ДО блокировки колонки/заказа. 0 строк → errNotFound.
func (r *TaskRepo) TaskColumnID(ctx context.Context, tx pgx.Tx, id int64) (string, error) {
	var colID string
	err := tx.QueryRow(ctx, `SELECT column_id FROM tasks WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&colID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return colID, err
}

// lastLogJoinSQL — design-sql.md §Last Log Expression, LEFT JOIN LATERAL коррелированный
// с t.id; алиасы ls/lu — во избежание конфликта с внешними s/u (Board List/Get). Go:
// `cut` → добавить «…» (buildLastLog).
const lastLogJoinSQL = `
	LEFT JOIN LATERAL (
	    SELECT left(l.text, 120) AS text, char_length(l.text) > 120 AS cut, lu.name AS author_name, l.created_at
	    FROM subtasks ls JOIN subtask_logs l ON l.subtask_id = ls.id JOIN users lu ON lu.id = l.user_id
	    WHERE ls.task_id = t.id ORDER BY l.created_at DESC, l.id DESC LIMIT 1) ll ON true`

// transfersAmountSQL — «Продано по накладным» (transfers-sales DS-014): Σ сумм накладных
// «склад → клиент» заказа t в статусах «В пути» и «Получено» (заявка и отменённая не входят);
// сумма накладной — Σ round(price × qty_sent, 2) товарных строк (DS-038). NUMERIC-скаляр,
// коррелирован с t.id.
const transfersAmountSQL = `
	(SELECT round(coalesce(sum(round(si.price * si.qty_sent, 2)), 0), 2)
	 FROM transfers st JOIN transfer_items si ON si.transfer_id = st.id
	 WHERE st.task_id = t.id AND st.deleted_at IS NULL AND st.direction = 'warehouse_to_client'
	   AND st.status IN ('in_transit', 'received') AND si.kind = 'product')`

// transfersAmountSelectSQL — transfersAmountSQL текстом (task.transfers_amount карточки).
const transfersAmountSelectSQL = transfersAmountSQL + `::text`

// totalAmountSelectSQL — design-sql.md §Totals Expression, скаляр-фрагмент коррелирован с
// t.id (алиас s2 — во избежание конфликта с внешними алиасами Get/List). Итог заказа =
// услуги + товары услуг + накладные (transfers-sales DS-014); единственное выражение итога
// на проект — им пользуются карточка (Get) и доска (boardListSQL).
const totalAmountSelectSQL = `
	((SELECT round(coalesce(sum(coalesce(s2.price, 0) +
	        coalesce((SELECT sum(round(sp.qty * coalesce(sp.price, 0), 2))
	                  FROM subtask_products sp WHERE sp.subtask_id = s2.id), 0)), 0), 2)
	 FROM subtasks s2 WHERE s2.task_id = t.id) + ` + transfersAmountSQL + `)::text`

// buildLastLog — nil, если записей журнала у заказа нет вовсе (ll.text IS NULL).
func buildLastLog(text, author sql.NullString, cut sql.NullBool, createdAt sql.NullTime) *TaskLastLog {
	if !text.Valid {
		return nil
	}
	t := text.String
	if cut.Valid && cut.Bool {
		t += "…"
	}
	return &TaskLastLog{Text: t, AuthorName: author.String, CreatedAt: createdAt.Time}
}

// LockTargetColumn — design-sql.md §Lock Target Column Query. 0 строк → errNotFound
// (сервис мапит в 400 field column_id, «404 — только id в пути»). Колонка мягко удалена →
// errColumnDeleted (404 COLUMN_NOT_FOUND, soft-delete-board-columns DS-026). Признак читается
// после ожидания блокировки: создание/перенос заказа, ждавшие параллельное удаление колонки,
// получают отказ; удаление, ждавшее FOR SHARE, видит действующий заказ (COLUMN_NOT_EMPTY).
func (r *TaskRepo) LockTargetColumn(ctx context.Context, tx pgx.Tx, columnID string) (*columnFlags, error) {
	var f columnFlags
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT id, is_final, consumes_stock, deleted_at IS NOT NULL FROM columns WHERE id = $1 FOR SHARE`, columnID).
		Scan(&f.ID, &f.IsFinal, &f.ConsumesStock, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if deleted {
		return nil, errColumnDeleted
	}
	return &f, nil
}

// LockTask — design-sql.md §Lock Task Query: ДЕЙСТВУЮЩИЙ заказ под FOR UPDATE. 0 строк →
// errNotFound (ErrTaskNotFound) — в том числе для мягко удалённого заказа (soft-delete-core
// DS-026: удалённый заказ для обычных адресов не существует). Условие проверяется после
// ожидания блокировки: правка, ждавшая параллельное удаление, получает 404.
func (r *TaskRepo) LockTask(ctx context.Context, tx pgx.Tx, id int64) (*Task, error) {
	t, deleted, err := r.LockTaskAny(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if deleted {
		return nil, errNotFound
	}
	return t, nil
}

// LockTaskAny — заказ под FOR UPDATE независимо от отметки удаления (восстановление и
// служебная уборка, soft-delete-core DS-005, DS-036); deleted — заказ мягко удалён.
func (r *TaskRepo) LockTaskAny(ctx context.Context, tx pgx.Tx, id int64) (*Task, bool, error) {
	var t Task
	var deleted bool
	err := tx.QueryRow(ctx, `
		SELECT id, column_id, client_id, title, assignee_id, due_date::text, is_paid, paid_at, position::text, version, description_html,
		       deleted_at IS NOT NULL
		FROM tasks WHERE id = $1 FOR UPDATE`, id).
		Scan(&t.ID, &t.ColumnID, &t.ClientID, &t.Title, &t.AssigneeID, &t.DueDate, &t.IsPaid, &t.PaidAt, &t.Position, &t.Version, &t.DescriptionHTML,
			&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, errNotFound
	}
	if err != nil {
		return nil, false, err
	}
	return &t, deleted, nil
}

// SoftDelete — отметка удаления (soft-delete-core DS-024): атомарный UPDATE с условием «ещё
// не удалён»; version + 1, updated_at = now(). 0 строк → errNotFound. Вызывается под LockTask
// после StockEngine.Reconcile(false) — товары заказа уже на складе.
func (r *TaskRepo) SoftDelete(ctx context.Context, tx pgx.Tx, id int64, userID, reason string) (time.Time, error) {
	var at time.Time
	err := tx.QueryRow(ctx, `
		UPDATE tasks SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return at, errNotFound
	}
	return at.UTC(), err
}

// Restore — снятие отметки удаления (soft-delete-core DS-024): атомарный UPDATE с условием
// «удалён»; три поля очищаются, version + 1 (один раз — и при подмене колонки). columnID /
// position не nil — заказ встаёт в другую колонку (DS-039). 0 строк → errNotFound (не удалён).
func (r *TaskRepo) Restore(ctx context.Context, tx pgx.Tx, id int64, columnID, position *string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE tasks SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
		       column_id = COALESCE($2::uuid, column_id), position = COALESCE($3::numeric, position),
		       version = version + 1, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL`, id, columnID, position)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// TaskColumnIDAny — колонка заказа независимо от отметки удаления: первый шаг восстановления
// (читается ДО блокировок, как TaskColumnID). 0 строк → errNotFound.
func (r *TaskRepo) TaskColumnIDAny(ctx context.Context, tx pgx.Tx, id int64) (string, error) {
	var colID string
	err := tx.QueryRow(ctx, `SELECT column_id FROM tasks WHERE id = $1`, id).Scan(&colID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return colID, err
}

// restoreColumn — колонка под FOR SHARE при восстановлении заказа.
type restoreColumn struct {
	columnFlags
	Name     string
	Position int
	// Deleted — колонка мягко удалена (возможна только у колонки самого заказа).
	Deleted bool
}

// LockRestoreColumns — колонка заказа columnID (в том числе мягко удалённая) и все действующие
// нефинальные колонки под FOR SHARE (кандидаты на подмену колонки: удалённой —
// soft-delete-board-columns DS-016, ставшей финальной — soft-delete-core DS-039). Порядок
// блокировки — id, как у ColumnRepo.LockAll: восстановление и правка колонок не блокируют
// друг друга взаимно.
func (r *TaskRepo) LockRestoreColumns(ctx context.Context, tx pgx.Tx, columnID string) ([]restoreColumn, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, is_final, consumes_stock, name, position, deleted_at IS NOT NULL FROM columns
		WHERE id = $1 OR (NOT is_final AND deleted_at IS NULL) ORDER BY id FOR SHARE`, columnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []restoreColumn
	for rows.Next() {
		var c restoreColumn
		if err := rows.Scan(&c.ID, &c.IsFinal, &c.ConsumesStock, &c.Name, &c.Position, &c.Deleted); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// OpenSubtasks — design-sql.md §Open Subtasks Query (D11/D22): незавершённые услуги
// заказа, проверка перед переносом в is_final.
func (r *TaskRepo) OpenSubtasks(ctx context.Context, tx pgx.Tx, taskID int64) ([]OpenSubtask, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, sv.name, s.status
		FROM subtasks s JOIN services sv ON sv.id = s.service_id
		WHERE s.task_id = $1 AND s.status <> 'done'
		ORDER BY s.position, s.id`, taskID)
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

// NextPosition — делегирует position.go (design-sql.md §Position Query). taskID=0 —
// сентинел нового (ещё не вставленного) заказа, используется Create.
func (r *TaskRepo) NextPosition(ctx context.Context, tx pgx.Tx, columnID string, taskID int64, prevID, nextID *int64) (string, error) {
	return nextPosition(ctx, tx, columnID, taskID, prevID, nextID)
}

// Move — design-sql.md §Move Task Query.
func (r *TaskRepo) Move(ctx context.Context, tx pgx.Tx, id int64, columnID, pos string) (*Task, error) {
	var t Task
	err := scanTaskRow(tx.QueryRow(ctx, `
		UPDATE tasks SET column_id = $2::uuid, position = $3::numeric, version = version + 1, updated_at = now()
		WHERE id = $1
		RETURNING `+taskWriteReturning, id, columnID, pos), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &t, err
}

// Update — design-sql.md §Update Task Query. 0 строк при уже подтверждённом (LockTask)
// существовании заказа → errVersionConflict.
func (r *TaskRepo) Update(ctx context.Context, tx pgx.Tx, id, version int64, in taskRow) (*Task, error) {
	var t Task
	err := scanTaskRow(tx.QueryRow(ctx, `
		UPDATE tasks SET title = $3, client_id = $4::uuid, column_id = $5::uuid, assignee_id = $6::uuid,
		       due_date = $7::date, is_paid = $8,
		       paid_at = CASE WHEN $8 AND NOT is_paid THEN now() WHEN NOT $8 THEN NULL ELSE paid_at END,
		       position = $9::numeric, description_html = $10, version = version + 1, updated_at = now()
		WHERE id = $1 AND version = $2
		RETURNING `+taskWriteReturning,
		id, version, in.Title, in.ClientID, in.ColumnID, in.AssigneeID, in.DueDate, in.IsPaid, in.Position, in.DescriptionHTML), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errVersionConflict
	}
	return &t, err
}

// Insert — design-sql.md §Insert Task Query.
func (r *TaskRepo) Insert(ctx context.Context, tx pgx.Tx, in taskRow, createdBy string) (*Task, error) {
	var t Task
	err := scanTaskRow(tx.QueryRow(ctx, `
		INSERT INTO tasks (column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, created_by, description_html)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::date, $6, CASE WHEN $6 THEN now() END, $7::numeric, $8::uuid, $9)
		RETURNING `+taskWriteReturning,
		in.ColumnID, in.ClientID, in.Title, in.AssigneeID, in.DueDate, in.IsPaid, in.Position, createdBy, in.DescriptionHTML), &t)
	return &t, err
}

// Delete — design-sql.md §Delete Task Query: ФИЗИЧЕСКОЕ удаление строки — только служебная
// уборка автотестов (TaskService.Purge, soft-delete-core DS-036); продукт заказ удаляет мягко
// (SoftDelete). Вызывающая сторона уже отработала StockEngine.Reconcile(target=false) под тем
// же FOR UPDATE заказа.
func (r *TaskRepo) Delete(ctx context.Context, tx pgx.Tx, id int64) error {
	tag, err := tx.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ClientExists — проверка client_id (design-go-layers.md §Service Interface: «проверка
// client/assignee» перед Insert/Update). Запрос — в переданной tx (тот же снапшот
// READ COMMITTED, что и остальные шаги Create/Update).
//
// soft-delete-clients DS-014, DS-024: мягко удалённый клиент для новых заказов и смены клиента
// не существует. Строка клиента блокируется FOR KEY SHARE до конца транзакции: удаление клиента
// (clients.Repo.SoftDeleteClient, FOR UPDATE) либо уже видно — false, либо ждёт заказ, и тогда
// заказ остаётся у удалённого клиента с бейджем (допустимо).
func (r *TaskRepo) ClientExists(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM clients WHERE id = $1 AND deleted_at IS NULL FOR KEY SHARE`, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// DeletedRefs — восстановление заказа (soft-delete-clients DS-016): клиент заказа в удалённых;
// хотя бы одна единица оборудования заказа в удалённых. Заказ уже под FOR UPDATE.
func (r *TaskRepo) DeletedRefs(ctx context.Context, tx pgx.Tx, id int64) (clientDeleted, equipmentDeleted bool, err error) {
	err = tx.QueryRow(ctx, `
		SELECT cl.deleted_at IS NOT NULL,
		       EXISTS (SELECT 1 FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id
		               WHERE te.task_id = t.id AND e.deleted_at IS NOT NULL)
		FROM tasks t JOIN clients cl ON cl.id = t.client_id WHERE t.id = $1`, id).Scan(&clientDeleted, &equipmentDeleted)
	return clientDeleted, equipmentDeleted, err
}

// ActiveAssigneeExists — design-errors.md §Validation Fields: assignee_id «NULL или uuid
// активного пользователя».
func (r *TaskRepo) ActiveAssigneeExists(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND is_active)`, id).Scan(&exists)
	return exists, err
}

// Get — design-sql.md §Board List Query (без пагинации/фильтров, одна строка): полная
// карточка заказа с денормализованными именами и счётчиками подзадач — источник ответа
// GET /tasks/:id и всех mutation-ответов (Re-fetch-after-write).
//
// today — «сегодня» (YYYY-MM-DD) в поясе пользователя для subtasks_late (user-timezone
// DS-032, тот же источник, что TaskFilter.Today в List); пусто — current_date БД.
func (r *TaskRepo) Get(ctx context.Context, id int64, today string) (*Task, error) {
	var t Task
	var totalAmount, transfersAmount string
	var llText, llAuthor sql.NullString
	var llCut sql.NullBool
	var llCreatedAt sql.NullTime
	var todayArg any
	if today != "" {
		todayArg = today
	}
	err := r.pool.QueryRow(ctx, `
		SELECT t.id, t.column_id, t.client_id, cl.company_name, cl.deleted_at IS NOT NULL, t.title, t.assignee_id, u.name,
		       t.due_date::text, t.is_paid, t.paid_at, t.position::text, t.version, t.created_at, t.updated_at,
		       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id),
		       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status = 'done'),
		       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done' AND s.due_date < COALESCE($2::date, current_date)),
		       t.description_html, `+totalAmountSelectSQL+`, `+transfersAmountSelectSQL+`,
		       ll.text, ll.cut, ll.author_name, ll.created_at,
		       EXISTS (SELECT 1 FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id
		               WHERE te.task_id = t.id AND e.location_status = 'in_transit'),
		       `+executorUnpaidDoneSQL+`
		FROM tasks t
		JOIN clients cl ON cl.id = t.client_id
		LEFT JOIN users u ON u.id = t.assignee_id
		`+lastLogJoinSQL+`
		WHERE t.id = $1 AND t.deleted_at IS NULL`, id, todayArg).
		Scan(&t.ID, &t.ColumnID, &t.ClientID, &t.ClientName, &t.ClientIsDeleted, &t.Title, &t.AssigneeID, &t.AssigneeName,
			&t.DueDate, &t.IsPaid, &t.PaidAt, &t.Position, &t.Version, &t.CreatedAt, &t.UpdatedAt,
			&t.SubtasksTotal, &t.SubtasksDone, &t.SubtasksLate, &t.DescriptionHTML, &totalAmount, &transfersAmount,
			&llText, &llCut, &llAuthor, &llCreatedAt, &t.HasInTransit, &t.ExecutorUnpaidDone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	t.TotalAmount = numeric.Amount(totalAmount)
	t.TransfersAmount = numeric.Amount(transfersAmount)
	t.LastLog = buildLastLog(llText, llAuthor, llCut, llCreatedAt)
	return &t, nil
}

// boardListSQL — design-sql.md §Board List Query, `due_date`/`position` приведены к text
// (TaskCard.DueDate/Position — *string/string, не time.Time/Amount).
//
// pagination DS-002: фильтры считаются в CTE base по «лёгким» колонкам; там же окна
// row_number()/count(*) по колонке (column_total) и count(*) OVER () (total — без окна
// per_column, но с column_id). Тяжёлые скаляры (подзадачи, итог, последний лог) — только
// для строк, прошедших окно $11 и LIMIT/OFFSET. Порядок items — прежний контракт.
const boardListSQL = `
WITH base AS (
  SELECT t.id,
         row_number() OVER (PARTITION BY t.column_id ORDER BY t.position, t.id) AS rn,
         count(*) OVER (PARTITION BY t.column_id)                               AS column_total,
         count(*) OVER ()                                                       AS total
  FROM tasks t
  JOIN columns c  ON c.id = t.column_id
  JOIN clients cl ON cl.id = t.client_id
  WHERE t.deleted_at IS NULL
    AND ($1::uuid IS NULL OR t.client_id = $1)
    AND ($2::text IS NULL OR t.title ILIKE '%' || $2 || '%' ESCAPE '\' OR cl.company_name ILIKE '%' || $2 || '%' ESCAPE '\'
         OR t.id = $3::bigint
         OR t.description_html ILIKE '%' || $12 || '%' ESCAPE '\'
         OR EXISTS (SELECT 1 FROM subtasks s JOIN services sv ON sv.id = s.service_id
                    WHERE s.task_id = t.id AND sv.name ILIKE '%' || $2 || '%' ESCAPE '\')
         OR EXISTS (SELECT 1 FROM subtasks s JOIN subtask_products sp ON sp.subtask_id = s.id
                    JOIN products pr ON pr.id = sp.product_id
                    WHERE s.task_id = t.id AND pr.name ILIKE '%' || $2 || '%' ESCAPE '\')
         OR EXISTS (SELECT 1 FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id
                    WHERE te.task_id = t.id AND (e.model ILIKE '%' || $2 || '%' ESCAPE '\'
                                              OR e.serial ILIKE '%' || $2 || '%' ESCAPE '\')))
    AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM subtasks s JOIN services sv ON sv.id = s.service_id
                                     WHERE s.task_id = t.id AND sv.category_id = $4))
    AND ($5::boolean IS NULL OR t.is_paid = $5)
    AND ($6::uuid IS NULL OR t.assignee_id = $6
         OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.assignee_id = $6))
    AND ($7::text IS NULL OR (NOT c.is_final AND (
           ($7 = 'overdue' AND t.due_date < COALESCE($15::date, current_date))
        OR ($7 = 'soon'    AND t.due_date <= COALESCE($15::date, current_date) + 1)
        OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                   AND s.due_date < COALESCE($15::date, current_date)))))
    AND ($10::uuid IS NULL OR t.column_id = $10)
    AND ($13::uuid IS NULL OR EXISTS (SELECT 1 FROM task_equipment te
           JOIN equipment e ON e.id = te.equipment_id
           LEFT JOIN transfers tr ON tr.id = e.active_transfer_id
           WHERE te.task_id = t.id
             AND ((e.location_status = 'at_warehouse' AND e.warehouse_id = $13)
               OR (e.location_status = 'in_transit' AND tr.to_warehouse_id = $13))))
    AND ($14::text IS NULL
         OR ($14 = 'unpaid' AND EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id
                AND s.assignee_id IS NOT NULL AND NOT s.is_executor_paid))
         OR ($14 = 'paid' AND EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id
                AND s.assignee_id IS NOT NULL)
             AND NOT EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id
                AND s.assignee_id IS NOT NULL AND NOT s.is_executor_paid)))
), page AS (
  SELECT b.id, b.column_total, b.total
  FROM base b
  JOIN tasks t   ON t.id = b.id
  JOIN columns c ON c.id = t.column_id
  WHERE ($11::int IS NULL OR b.rn <= $11)
  ORDER BY c.position, c.id, t.position, t.id
  LIMIT $8 OFFSET $9
)
SELECT t.id, t.column_id, t.client_id, cl.company_name, cl.deleted_at IS NOT NULL, t.title, t.assignee_id, u.name,
       t.due_date::text, t.is_paid, t.position::text, t.version,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id)                       AS subtasks_total,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status = 'done') AS subtasks_done,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                                           AND s.due_date < COALESCE($15::date, current_date)) AS subtasks_late,
       ` + totalAmountSelectSQL + ` AS total_amount,
       ll.text, ll.cut, ll.author_name, ll.created_at,
       p.column_total, p.total,
       EXISTS (SELECT 1 FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id
		               WHERE te.task_id = t.id AND e.location_status = 'in_transit') AS has_in_transit,
       ` + executorUnpaidDoneSQL + ` AS executor_unpaid_done
FROM page p
JOIN tasks t    ON t.id = p.id
JOIN columns c  ON c.id = t.column_id
JOIN clients cl ON cl.id = t.client_id
LEFT JOIN users u ON u.id = t.assignee_id
` + lastLogJoinSQL + `
ORDER BY c.position, c.id, t.position, t.id`

// List — design-sql.md §Board List Query. Поиск q — заказ/описание/услуга/товар/#номер и
// модель/SN оборудования заказа (plan.md §Доска, §Оборудование клиента). Q-3 передаётся вызывающей стороной уже
// экранированным (pkg/sqlutil.EscapeLike, tasks_filter.go).
func (r *TaskRepo) List(ctx context.Context, f TaskFilter) ([]TaskCard, int, error) {
	// today — «сегодня» в поясе пользователя (user-timezone DS-032); nil — current_date БД.
	var q, qDesc, due, executorPaid, today any
	if f.Today != "" {
		today = f.Today
	}
	if f.Q != "" {
		q, qDesc = f.Q, f.QDesc
	}
	if f.Due != "" {
		due = f.Due
	}
	if f.ExecutorPaid != "" {
		executorPaid = f.ExecutorPaid
	}

	rows, err := r.pool.Query(ctx, boardListSQL,
		f.ClientID, q, f.TaskNo, f.CategoryID, f.Paid, f.AssigneeID, due, f.Limit, f.Offset,
		f.ColumnID, f.PerColumn, qDesc, f.WarehouseID, executorPaid, today)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var total int
	out := []TaskCard{}
	for rows.Next() {
		var c TaskCard
		var totalAmount string
		var llText, llAuthor sql.NullString
		var llCut sql.NullBool
		var llCreatedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.ColumnID, &c.ClientID, &c.ClientName, &c.ClientIsDeleted, &c.Title, &c.AssigneeID, &c.AssigneeName,
			&c.DueDate, &c.IsPaid, &c.Position, &c.Version, &c.SubtasksTotal, &c.SubtasksDone, &c.SubtasksLate,
			&totalAmount, &llText, &llCut, &llAuthor, &llCreatedAt, &c.ColumnTotal, &total, &c.HasInTransit, &c.ExecutorUnpaidDone); err != nil {
			return nil, 0, err
		}
		c.TotalAmount = numeric.Amount(totalAmount)
		c.LastLog = buildLastLog(llText, llAuthor, llCut, llCreatedAt)
		out = append(out, c)
	}
	return out, total, rows.Err()
}
