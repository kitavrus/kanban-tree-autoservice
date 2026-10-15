package board

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/kitavrus/kanban-tree/internal/features/clients"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Actor — вызывающий пользователь (design-go-layers.md §Service Interface). Из
// middleware.CurrentPrincipal(c) — эталон catalog/products_handler.go:77-80.
type Actor struct {
	ID      string
	IsAdmin bool
	// Today — «сегодня» (YYYY-MM-DD) в часовом поясе пользователя (user-timezone DS-032):
	// от него считаются просрочки (фильтр срока, subtasks_late) в списке, карточке и ответах
	// мутаций. Заполняет currentActor; пусто — current_date сервера БД.
	Today string
}

// TaskService — концентрирует Lock Order (columns→tasks→products, ADR-104) операций
// над заказами: список/чтение без транзакции, мутации — withTx + StockEngine.Reconcile.
type TaskService struct {
	pool   *pgxpool.Pool
	repo   *TaskRepo
	engine *StockEngine
	files  filestore.FileStore // фаза 7: файлы вложений удаляются после Delete заказа
}

func NewTaskService(pool *pgxpool.Pool, repo *TaskRepo, engine *StockEngine) *TaskService {
	return &TaskService{pool: pool, repo: repo, engine: engine}
}

// WithFiles — хранилище вложений (фаза 7): Delete заказа удаляет их файлы после commit.
func (s *TaskService) WithFiles(fs filestore.FileStore) *TaskService {
	s.files = fs
	return s
}

func (s *TaskService) List(ctx context.Context, f TaskFilter) (*TaskListResponse, error) {
	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return &TaskListResponse{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *TaskService) Get(ctx context.Context, a Actor, idStr string) (*TaskResponse, error) {
	id, ok := parseTaskID(idStr)
	if !ok {
		return nil, ErrTaskNotFound
	}
	t, err := s.repo.Get(ctx, id, a.Today)
	if errors.Is(err, errNotFound) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toTaskResponse(*t)
	return &resp, nil
}

// Create — design-go-layers.md §Service Interface: Lock Target Column → автомобиль (найден
// либо создан; клиент заказа — его владелец) → проверка assignee → Insert (позиция в конец,
// tasks.car_id) → Reconcile (no-op без услуг; единообразие).
func (s *TaskService) Create(ctx context.Context, a Actor, in TaskCreateRequest) (*TaskMutationResponse, error) {
	in = in.normalize()
	car, f := in.prepare()
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	if !a.IsAdmin && in.IsPaid != nil && *in.IsPaid {
		return nil, ErrPaidAdminOnly
	}

	var taskID int64
	var reconcile ReconcileResult
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		target, err := s.repo.LockTargetColumn(ctx, tx, in.ColumnID)
		if err != nil {
			return columnRefError(err)
		}

		// Клиент/автомобиль резолвятся (либо создаются) в этой же tx — любой отказ ниже
		// откатывает и нового клиента, и новый автомобиль.
		clientID, title, carID, err := s.resolveCar(ctx, tx, in, car)
		if err != nil {
			return err
		}

		assigneeID := nilIfEmpty(in.AssigneeID)
		if assigneeID != nil {
			ok, err := s.repo.ActiveAssigneeExists(ctx, tx, *assigneeID)
			if err != nil {
				return err
			}
			if !ok {
				return errorspkg.Validation(errorspkg.Field{Name: "assignee_id", Message: msgNotFound})
			}
		}

		pos, err := s.repo.NextPosition(ctx, tx, in.ColumnID, 0, nil, nil)
		if err != nil {
			return err
		}

		// description — POST: nil → "" (ADR-006, design-go-layers.md §DTO Task Changes).
		descHTML := ""
		if in.Description != nil {
			descHTML = descriptionToHTML(*in.Description)
		}

		created, err := s.repo.Insert(ctx, tx, taskRow{
			Title: title, ClientID: clientID, CarID: carID, ColumnID: in.ColumnID,
			AssigneeID: assigneeID, DueDate: nilIfEmpty(in.DueDate),
			IsPaid: in.IsPaid != nil && *in.IsPaid, Position: pos, DescriptionHTML: descHTML,
		}, a.ID)
		if err != nil {
			return mapTaskFKError(err)
		}
		taskID = created.ID

		res, err := s.engine.Reconcile(ctx, tx, taskID, target.ConsumesStock, a.ID)
		if err != nil {
			return err
		}
		reconcile = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.mutationResponse(ctx, a, taskID, reconcile)
}

// resolveCar — автомобиль нового заказа (tx вызывающего): найденный автомобиль → его владелец
// (сверка госномера и client_id), иначе новый автомобиль у client_id либо у нового клиента.
// Возвращает клиента заказа, название (пустой title → госномер) и автомобиль заказа
// (tasks.car_id). Ошибки — Validation с путями полей формы.
func (s *TaskService) resolveCar(ctx context.Context, tx pgx.Tx, in TaskCreateRequest, car *carOrder) (clientID, title, equipmentID string, err error) {
	clientID, title = in.ClientID, in.Title
	field := func(name, msg string) error {
		return errorspkg.Validation(errorspkg.Field{Name: name, Message: msg})
	}

	if car.equipmentID != "" {
		found, err := s.repo.CarForOrder(ctx, tx, car.equipmentID)
		if errors.Is(err, errNotFound) {
			return "", "", "", field("equipment_id", msgNotFound)
		}
		if err != nil {
			return "", "", "", err
		}
		if sqlutil.PlateCompact(found.PlateNumber) != sqlutil.PlateCompact(car.plate) {
			return "", "", "", field("equipment_id", msgPlateMismatch)
		}
		if clientID != "" && clientID != found.ClientID {
			return "", "", "", field("equipment_id", msgForeignClient)
		}
		if title == "" {
			title = carTitle(found.PlateNumber)
		}
		return found.ClientID, title, car.equipmentID, nil
	}

	if car.newClient != nil {
		clientID, err = clients.CreateClientTx(ctx, tx, *car.newClient)
		if err != nil {
			return "", "", "", err
		}
	}
	equipmentID, err = clients.CreateEquipmentTx(ctx, tx, clientID, *car.newEquipment)
	switch {
	case errors.Is(err, clients.ErrOrderClientGone):
		return "", "", "", field("client_id", msgNotFound)
	case err != nil:
		return "", "", "", err
	}
	if title == "" {
		title = carTitle(car.newEquipment.PlateNumber)
	}
	return clientID, title, equipmentID, nil
}

// Update — полная замена карточки (LWW по version). column_id сменился → шаги
// Lock/OpenSubtasks/Position/Reconcile выполняются как при Move; иначе позиция и склад
// не трогаются (design-go-layers.md §Service Interface).
func (s *TaskService) Update(ctx context.Context, a Actor, idStr string, in TaskUpdateRequest) (*TaskMutationResponse, error) {
	id, ok := parseTaskID(idStr)
	if !ok {
		return nil, ErrTaskNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	var reconcile ReconcileResult
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		target, err := s.repo.LockTargetColumn(ctx, tx, in.ColumnID)
		if err != nil {
			return columnRefError(err)
		}
		current, err := s.repo.LockTask(ctx, tx, id)
		if err != nil {
			return taskLockError(err)
		}

		// version-конфликт — ДО paid-гарда (AC-010/AC-011, code review MEDIUM-2): executor
		// со устаревшей формой должен получить 409 TASK_VERSION_CONFLICT (клиент по коду
		// перезагружает карточку), а не ложный 403 PAID_ADMIN_ONLY из-за того, что is_paid
		// успели сменить с момента открытия формы. current — из LockTask (FOR UPDATE),
		// проверка авторитетна: значение не может смениться до конца этой транзакции.
		if *in.Version != current.Version {
			return ErrTaskVersionConflict
		}

		if !a.IsAdmin {
			wantPaid := in.IsPaid != nil && *in.IsPaid
			if wantPaid != current.IsPaid {
				return ErrPaidAdminOnly
			}
		}

		// Клиент и автомобиль заказа не меняются (one-car-order): в теле PUT их нет, заказ
		// удалённого клиента/автомобиля правится как обычно.
		assigneeID := nilIfEmpty(in.AssigneeID)
		if !samePtr(current.AssigneeID, assigneeID) && assigneeID != nil {
			ok, err := s.repo.ActiveAssigneeExists(ctx, tx, *assigneeID)
			if err != nil {
				return err
			}
			if !ok {
				return errorspkg.Validation(errorspkg.Field{Name: "assignee_id", Message: msgNotFound})
			}
		}

		// description — PUT: nil → не менять (текущее значение из LockTask), иначе
		// пересобрать HTML (ADR-006).
		descHTML := current.DescriptionHTML
		if in.Description != nil {
			descHTML = descriptionToHTML(*in.Description)
		}

		columnChanged := current.ColumnID != in.ColumnID
		pos := current.Position
		if columnChanged {
			if target.IsFinal {
				open, err := s.repo.OpenSubtasks(ctx, tx, id)
				if err != nil {
					return err
				}
				if len(open) > 0 {
					return ErrSubtasksNotDone.WithDetail("subtasks", open)
				}
			}
			pos, err = s.repo.NextPosition(ctx, tx, in.ColumnID, id, nil, nil)
			if err != nil {
				return err
			}
		}

		if _, err := s.repo.Update(ctx, tx, id, *in.Version, taskRow{
			Title: in.Title, ColumnID: in.ColumnID,
			AssigneeID: assigneeID, DueDate: nilIfEmpty(in.DueDate),
			IsPaid: in.IsPaid != nil && *in.IsPaid, Position: pos, DescriptionHTML: descHTML,
		}); err != nil {
			if errors.Is(err, errVersionConflict) {
				return ErrTaskVersionConflict
			}
			return mapTaskFKError(err)
		}

		if columnChanged {
			res, err := s.engine.Reconcile(ctx, tx, id, target.ConsumesStock, a.ID)
			if err != nil {
				return err
			}
			reconcile = res
		} else {
			reconcile = ReconcileResult{Warnings: []StockWarning{}}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.mutationResponse(ctx, a, id, reconcile)
}

// Move — design-go-layers.md §Service Interface, шаги 1-7.
func (s *TaskService) Move(ctx context.Context, a Actor, idStr string, in TaskMoveRequest) (*TaskMutationResponse, error) {
	id, ok := parseTaskID(idStr)
	if !ok {
		return nil, ErrTaskNotFound
	}

	spanCtx, span := otel.Tracer("board").Start(ctx, "tasks.move")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id))

	var fromColumn string
	var reconcile ReconcileResult
	err := withTx(spanCtx, s.pool, func(tx pgx.Tx) error {
		target, err := s.repo.LockTargetColumn(spanCtx, tx, in.ColumnID)
		if err != nil {
			return columnRefError(err)
		}
		current, err := s.repo.LockTask(spanCtx, tx, id)
		if err != nil {
			return taskLockError(err)
		}
		fromColumn = current.ColumnID
		columnChanged := current.ColumnID != in.ColumnID

		if target.IsFinal && columnChanged {
			open, err := s.repo.OpenSubtasks(spanCtx, tx, id)
			if err != nil {
				return err
			}
			if len(open) > 0 {
				return ErrSubtasksNotDone.WithDetail("subtasks", open)
			}
		}

		pos, err := s.repo.NextPosition(spanCtx, tx, in.ColumnID, id, in.PrevID, in.NextID)
		if err != nil {
			return err
		}
		if _, err := s.repo.Move(spanCtx, tx, id, in.ColumnID, pos); err != nil {
			return err
		}

		res, err := s.engine.Reconcile(spanCtx, tx, id, target.ConsumesStock, a.ID)
		if err != nil {
			return err
		}
		reconcile = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	span.SetAttributes(
		attribute.String("from_column", fromColumn),
		attribute.String("to_column", in.ColumnID),
		attribute.Int("movements_count", reconcile.Movements),
	)
	return s.mutationResponse(ctx, a, id, reconcile)
}

// Delete — МЯГКОЕ удаление заказа (soft-delete-core §6.1): только admin (роут + повторная
// проверка роли в сервисе — первой, до поиска записи, DS-003), причина обязательна (DS-020).
// Одна транзакция под блокировкой заказа: StockEngine.Reconcile(false) возвращает все товары
// заказа на склад (DS-004) → отметка «кто, когда, почему». Строка, части заказа и файлы
// вложений остаются (DS-013). Заказ уже удалён либо не существует → 404 (DS-024).
func (s *TaskService) Delete(ctx context.Context, a Actor, idStr string, in softdelete.DeleteRequest) (*TaskDeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	id, ok := parseTaskID(idStr)
	if !ok {
		return nil, ErrTaskNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}

	var reconcile ReconcileResult
	var deletedAt time.Time
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.repo.LockTask(ctx, tx, id); err != nil {
			return taskLockError(err)
		}
		res, err := s.engine.Reconcile(ctx, tx, id, false, a.ID)
		if err != nil {
			return err
		}
		reconcile = res
		at, err := s.repo.SoftDelete(ctx, tx, id, a.ID, reason)
		if err != nil {
			return taskLockError(err)
		}
		deletedAt = at
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Аудит (DS-030): факт удаления — в журнал приложения; причина в журнал не пишется.
	slog.InfoContext(ctx, "soft delete", "kind", "tasks", "id", id, "user_id", a.ID, "action", "delete")
	return &TaskDeleteResponse{ID: id, DeletedAt: deletedAt, Warnings: stockWarningsOrEmpty(reconcile.Warnings)}, nil
}

// noticeColumnReplaced — код предупреждения восстановления: заказ поставлен в другую колонку.
const noticeColumnReplaced = "column_replaced"

// noticeClientDeleted / noticeEquipmentDeleted — коды предупреждений восстановления заказа,
// чей клиент либо оборудование в удалённых (soft-delete-clients §6.5, DS-016).
const (
	noticeClientDeleted    = "client_deleted"
	noticeEquipmentDeleted = "equipment_deleted"
)

// Restore — восстановление мягко удалённого заказа (soft-delete-core §6.2): только admin.
// Одна транзакция, Lock Order общий (columns → tasks → products): колонка заказа и все
// нефинальные колонки под FOR SHARE (порядок id — как ColumnRepo.LockAll, без взаимной
// блокировки с правкой колонок), затем заказ FOR UPDATE. Заказ не удалён → 409 NOT_DELETED
// (DS-024). Колонка заказа за время удаления стала финальной, а у заказа есть незавершённые
// услуги → заказ встаёт в конец первой нефинальной колонки, notices: column_replaced (DS-039).
// Колонка заказа мягко удалена → то же: конец первой нефинальной действующей колонки и
// column_replaced с текстом «Колонка заказа удалена…» (soft-delete-board-columns DS-016); сама
// колонка не восстанавливается.
// Затем товары списываются снова по правилу колонки заказа; нехватка — предупреждения, не
// отказ (DS-005).
func (s *TaskService) Restore(ctx context.Context, a Actor, idStr string) (*TaskRestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	id, ok := parseTaskID(idStr)
	if !ok {
		return nil, ErrTaskNotFound
	}

	var reconcile ReconcileResult
	notices := []RestoreNotice{}
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		notices = notices[:0]
		colID, err := s.repo.TaskColumnIDAny(ctx, tx, id)
		if err != nil {
			return taskLockError(err)
		}
		cols, err := s.repo.LockRestoreColumns(ctx, tx, colID)
		if err != nil {
			return err
		}
		task, deleted, err := s.repo.LockTaskAny(ctx, tx, id)
		if err != nil {
			return taskLockError(err)
		}
		if !deleted {
			return softdelete.ErrNotDeleted
		}
		if task.ColumnID != colID {
			return errTaskMovedRetry
		}
		var target *restoreColumn
		for i := range cols {
			if cols[i].ID == colID {
				target = &cols[i]
			}
		}
		if target == nil {
			return ErrColumnNotFound
		}

		var newColumn, newPosition *string
		var repl *restoreColumn
		why := ""
		switch {
		case target.Deleted:
			// DS-016: на доске всегда есть действующая нефинальная колонка (инвариант DS-019).
			if repl = firstNonFinal(cols); repl == nil {
				return ErrLastNonFinalColumn
			}
			why = "Колонка заказа удалена"
		case target.IsFinal:
			open, err := s.repo.OpenSubtasks(ctx, tx, id)
			if err != nil {
				return err
			}
			if len(open) > 0 {
				repl = firstNonFinal(cols)
				why = "Колонка заказа стала финальной"
			}
		}
		if repl != nil {
			pos, err := s.repo.NextPosition(ctx, tx, repl.ID, id, nil, nil)
			if err != nil {
				return err
			}
			newColumn, newPosition = &repl.ID, &pos
			target = repl
			notices = append(notices, RestoreNotice{
				Code:    noticeColumnReplaced,
				Message: why + " — заказ поставлен в колонку «" + repl.Name + "»",
			})
		}

		if err := s.repo.Restore(ctx, tx, id, newColumn, newPosition); err != nil {
			if errors.Is(err, errNotFound) {
				return softdelete.ErrNotDeleted
			}
			return err
		}
		res, err := s.engine.Reconcile(ctx, tx, id, target.ConsumesStock, a.ID)
		if err != nil {
			return err
		}
		reconcile = res

		// DS-016: клиент либо оборудование заказа в удалённых — заказ восстанавливается, записи
		// остаются с пометкой «удалён», пользователю — предупреждение.
		clientDeleted, equipmentDeleted, err := s.repo.DeletedRefs(ctx, tx, id)
		if err != nil {
			return err
		}
		if clientDeleted {
			notices = append(notices, RestoreNotice{Code: noticeClientDeleted, Message: "Клиент заказа в удалённых"})
		}
		if equipmentDeleted {
			notices = append(notices, RestoreNotice{Code: noticeEquipmentDeleted, Message: "Автомобиль заказа в удалённых"})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", "tasks", "id", id, "user_id", a.ID, "action", "restore")
	return &TaskRestoreResponse{ID: id, Warnings: stockWarningsOrEmpty(reconcile.Warnings), Notices: notices}, nil
}

// firstNonFinal — первая по позиции (затем id) действующая нефинальная колонка; nil — таких нет.
func firstNonFinal(cols []restoreColumn) *restoreColumn {
	var best *restoreColumn
	for i := range cols {
		c := &cols[i]
		if c.IsFinal || c.Deleted {
			continue
		}
		if best == nil || c.Position < best.Position || (c.Position == best.Position && c.ID < best.ID) {
			best = c
		}
	}
	return best
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка заказа автотестами (soft-delete-core §6.5, DS-027,
// DS-036): прежнее безвозвратное удаление заказа, работает и для действующего, и для мягко
// удалённого. Товары возвращаются на склад
// (у удалённого заказа уже возвращены — Reconcile ничего не двигает); строка tasks удаляется
// (части — каскадом FK); файлы вложений — с диска после commit. Заказа нет → errNotFound
// наружу как 404 NOT_FOUND (сервис уборки не раскрывает вид записи).
func (s *TaskService) Purge(ctx context.Context, a Actor, idStr string) error {
	if !a.IsAdmin {
		return errorspkg.Forbidden("Нет доступа")
	}
	id, ok := parseTaskID(idStr)
	if !ok {
		return errPurgeNotFound()
	}

	var fileKeys []string
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, _, err := s.repo.LockTaskAny(ctx, tx, id); err != nil {
			if errors.Is(err, errNotFound) {
				return errPurgeNotFound()
			}
			return err
		}
		if _, err := s.engine.Reconcile(ctx, tx, id, false, a.ID); err != nil {
			return err
		}
		// Строки task_attachments уйдут каскадом FK, файлы — после commit.
		keys, err := attachmentKeysTx(ctx, tx, id)
		if err != nil {
			return err
		}
		fileKeys = keys
		if err := s.repo.Delete(ctx, tx, id); err != nil {
			if errors.Is(err, errNotFound) {
				return errPurgeNotFound()
			}
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	removeAttachmentFiles(ctx, s.files, fileKeys)
	return nil
}

func errPurgeNotFound() error { return errorspkg.NotFound("Не найдено") }

func stockWarningsOrEmpty(w []StockWarning) []StockWarning {
	if w == nil {
		return []StockWarning{}
	}
	return w
}

// mutationResponse — Re-fetch-after-write: ответ Create/Update/Move собирается ИЗ Get()
// после коммита транзакции, не из RETURNING-строки мутации (денормализованные поля
// TaskCard — JOIN'ы, посчитанные заново).
func (s *TaskService) mutationResponse(ctx context.Context, a Actor, id int64, reconcile ReconcileResult) (*TaskMutationResponse, error) {
	full, err := s.repo.Get(ctx, id, a.Today)
	if err != nil {
		return nil, err
	}
	warnings := reconcile.Warnings
	if warnings == nil {
		warnings = []StockWarning{}
	}
	return &TaskMutationResponse{Task: toTaskResponse(*full), Warnings: warnings}, nil
}

func toTaskResponse(t Task) TaskResponse {
	return TaskResponse{
		TaskCard: TaskCard{
			ID: t.ID, ColumnID: t.ColumnID, ClientID: t.ClientID, ClientName: t.ClientName,
			ClientIsDeleted: t.ClientIsDeleted,
			Title:           t.Title, AssigneeID: t.AssigneeID, AssigneeName: t.AssigneeName,
			DueDate: t.DueDate, IsPaid: t.IsPaid, Position: t.Position, Version: t.Version,
			SubtasksTotal: t.SubtasksTotal, SubtasksDone: t.SubtasksDone, SubtasksLate: t.SubtasksLate,
			TotalAmount: t.TotalAmount, LastLog: t.LastLog,
			ExecutorUnpaidDone: t.ExecutorUnpaidDone,
		},
		PaidAt: t.PaidAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DescriptionHTML: t.DescriptionHTML,
	}
}

// parseTaskID — design-errors.md: «tasks.id в пути — strconv.ParseInt>0 до SQL».
func parseTaskID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id > 0
}

// nilIfEmpty — пустая строка внутри указателя (после normalize) трактуется как «нет
// значения» на уровне записи в БД/проверки существования.
func nilIfEmpty(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// samePtr — nil-безопасное сравнение двух *string (assignee_id «сменился» ли, design-
// errors.md §Validation Fields).
func samePtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// columnRefError — LockTargetColumn(errNotFound) → 400 field column_id «не найден»
// (design-errors.md: «404 — только id в пути», ссылки из тела — VALIDATION). Мягко удалённая
// колонка (errColumnDeleted) → 404 COLUMN_NOT_FOUND (soft-delete-board-columns §6.3, DS-026).
func columnRefError(err error) error {
	if errors.Is(err, errColumnDeleted) {
		return ErrColumnNotFound
	}
	if errors.Is(err, errNotFound) {
		return errorspkg.Validation(errorspkg.Field{Name: "column_id", Message: msgNotFound})
	}
	return err
}

func taskLockError(err error) error {
	if errors.Is(err, errNotFound) {
		return ErrTaskNotFound
	}
	return err
}

// mapTaskFKError — гонка с параллельным удалением client_id/assignee_id между проверкой
// существования и INSERT/UPDATE (design-errors.md §Error Mapping).
func mapTaskFKError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		switch pgErr.ConstraintName {
		case "tasks_client_id_fkey":
			return errorspkg.Validation(errorspkg.Field{Name: "client_id", Message: msgNotFound})
		case "tasks_car_id_fkey":
			return errorspkg.Validation(errorspkg.Field{Name: "equipment_id", Message: msgNotFound})
		case "tasks_assignee_id_fkey":
			return errorspkg.Validation(errorspkg.Field{Name: "assignee_id", Message: msgNotFound})
		}
	}
	return err
}
