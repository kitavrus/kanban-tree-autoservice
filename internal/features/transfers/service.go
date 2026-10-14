package transfers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

const (
	codeStatusConflict = "TRANSFER_STATUS_CONFLICT"
	// codeEquipmentInTransfer — единица уже стоит в другом незавершённом документе (DS-019);
	// details.transfer_id / details.equipment_id. Тот же код отдаёт board при удалении такой
	// единицы из заказа (DS-026).
	codeEquipmentInTransfer = "EQUIPMENT_IN_TRANSFER"
	// codeUnreceiveBlocked — отмена проведения запрещена единицей документа (transfers-revert
	// DS-023): её увёз более поздний документ либо она стоит в другом незавершённом;
	// details.transfer_id / details.equipment_id.
	codeUnreceiveBlocked = "TRANSFER_UNRECEIVE_BLOCKED"
)

var (
	ErrTransferNotFound = errorspkg.New("TRANSFER_NOT_FOUND", http.StatusNotFound, "Перемещение не найдено")
	// ErrVersionConflict — перемещение изменилось с момента загрузки (DS-048).
	ErrVersionConflict = errorspkg.New("TRANSFER_VERSION_CONFLICT", http.StatusConflict,
		"Перемещение уже изменено, обновите")
	// ErrStatusConflict — действие недоступно в текущем статусе; сообщение уточняет
	// statusConflict по паре действие × статус (§6.4–§6.7).
	ErrStatusConflict = errorspkg.New(codeStatusConflict, http.StatusConflict,
		"Действие недоступно в текущем статусе перемещения")
	ErrStockOverflow = errorspkg.New("STOCK_OVERFLOW", http.StatusConflict,
		"Слишком большое количество на складе")
	// ErrEquipmentNotAtSource — единица числится не на складе отправления (DS-036);
	// details.equipment_id.
	ErrEquipmentNotAtSource = errorspkg.New("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", http.StatusConflict,
		"Оборудование находится не на складе отправления")
	// ErrEquipmentNotAtClient — тот же код для приёма от клиента: единица уже не «у клиента»
	// (DS-013); details.equipment_id.
	ErrEquipmentNotAtClient = errorspkg.New("TRANSFER_EQUIPMENT_NOT_AT_SOURCE", http.StatusConflict,
		"Оборудование уже не у клиента")
	// ErrForbidden — исполнитель создаёт документ без клиента (DS-018).
	ErrForbidden = errorspkg.New("FORBIDDEN", http.StatusForbidden, "Нет доступа")
	// ErrTaskDeleted — новый документ по мягко удалённому заказу (soft-delete-transfers DS-007).
	ErrTaskDeleted = errorspkg.New("TASK_DELETED", http.StatusConflict, "Заказ удалён — сначала восстановите его")
	// ErrHasDeletedRefs — документ не может снова занять запись из своего состава, пока она в
	// удалённых (soft-delete DS-045): восстановление заявки и отмена проведения полученного
	// документа; details.kind — вид записи, details.id — её идентификатор.
	ErrHasDeletedRefs = errorspkg.New("TRANSFER_HAS_DELETED_REFS", http.StatusConflict,
		"В перемещении есть удалённые записи — сначала восстановите их")
	// ErrClientMismatch — клиент заказа сменился, пока документ был удалён: восстановить его
	// нельзя (soft-delete-transfers DS-040).
	ErrClientMismatch = errorspkg.New("TRANSFER_CLIENT_MISMATCH", http.StatusConflict,
		"Клиент заказа изменён — восстановить перемещение нельзя")
	// ErrEquipmentNotInTask — единицу заявки убрали из оборудования заказа, пока документ был
	// удалён: восстановить его нельзя (по аналогии с DS-018 / DS-040); details.equipment_id.
	ErrEquipmentNotInTask = errorspkg.New("TRANSFER_EQUIPMENT_NOT_IN_TASK", http.StatusConflict,
		"Оборудование убрано из заказа — восстановить перемещение нельзя")
)

// Actor — пользователь запроса: права по направлениям (DS-018) и часовой пояс для границ
// дня фильтра «период» (DS-034).
type Actor struct {
	ID       string
	IsAdmin  bool
	Timezone string
}

// canAccess — документ направления direction доступен пользователю: администратору — любой,
// исполнителю — только с клиентом (DS-018, DS-042).
func (a Actor) canAccess(direction string) bool {
	return a.IsAdmin || isClientDirection(direction)
}

// statusMessages — «своё сообщение» каждого недопустимого перехода (§5, §6.4–§6.7).
var statusMessages = map[string]map[string]string{
	actionUpdate: {
		StatusInTransit: "Перемещение уже отправлено — править нельзя",
		StatusReceived:  "Перемещение уже получено — править нельзя",
		StatusCancelled: "Перемещение отменено — править нельзя",
	},
	actionSend: {
		StatusInTransit: "Перемещение уже отправлено",
		StatusReceived:  "Перемещение уже получено",
		StatusCancelled: "Перемещение отменено",
	},
	actionReceive: {
		StatusRequested: "Перемещение ещё не отправлено",
		StatusReceived:  "Перемещение уже получено",
		StatusCancelled: "Перемещение отменено",
	},
	actionCancel: {
		StatusReceived:  "Перемещение получено — сначала отмените проведение",
		StatusCancelled: "Перемещение уже отменено",
	},
	actionUnreceive: {
		StatusRequested: "Перемещение ещё не проведено",
		StatusInTransit: "Перемещение ещё не получено",
		StatusCancelled: "Перемещение отменено",
	},
	actionDelete: {
		StatusInTransit: "Перемещение в пути — сначала отмените его",
		StatusReceived:  "Перемещение получено — сначала отмените проведение, затем отмените",
	},
}

func statusConflict(action, status string) *errorspkg.AppError {
	if msg := statusMessages[action][status]; msg != "" {
		return errorspkg.New(codeStatusConflict, http.StatusConflict, msg)
	}
	return ErrStatusConflict
}

const (
	defaultLimit = 50
	maxLimit     = 500
)

type Service struct {
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{repo: NewRepo(pool)} }

// ListQuery — сырые параметры списка (§6.1). TaskID, CreatedFrom, CreatedTo — строки запроса.
type ListQuery struct {
	Status, WarehouseID, Direction, SupplierID string
	TaskID, CreatedFrom, CreatedTo             string
	Limit, Offset                              int
}

func isUUID(v string) bool { return len(v) == 36 && uuid.Validate(v) == nil }

// parseListQuery — валидация фильтров и перевод периода в границы UTC: [начало дня
// created_from; начало дня, следующего за created_to) в часовом поясе пользователя (DS-034).
func parseListQuery(q ListQuery, tz string) (ListParams, []errorspkg.Field) {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }
	p := ListParams{Status: q.Status, WarehouseID: normID(q.WarehouseID), Direction: q.Direction, SupplierID: normID(q.SupplierID)}
	if q.Status != "" && !isStatus(q.Status) {
		add("status", msgInvalid)
	}
	if p.WarehouseID != "" && !isUUID(p.WarehouseID) {
		add("warehouse_id", msgInvalid)
	}
	if q.Direction != "" && !isDirection(q.Direction) {
		add("direction", msgInvalid)
	}
	if p.SupplierID != "" && !isUUID(p.SupplierID) {
		add("supplier_id", msgInvalid)
	}
	if q.TaskID != "" {
		id, err := strconv.ParseInt(q.TaskID, 10, 64)
		if err != nil || id <= 0 {
			add("task_id", msgInvalid)
		} else {
			p.TaskID = &id
		}
	}
	var from, to time.Time
	fromOK, toOK := false, false
	if q.CreatedFrom != "" {
		if from, fromOK = timezone.DayStart(tz, q.CreatedFrom); !fromOK {
			add("created_from", msgInvalid)
		} else {
			p.CreatedFrom = &from
		}
	}
	if q.CreatedTo != "" {
		if to, toOK = timezone.DayStart(tz, q.CreatedTo); !toOK {
			add("created_to", msgInvalid)
		}
	}
	if toOK {
		if fromOK && to.Before(from) {
			add("created_to", msgBeforeFrom)
		} else {
			// конец дня включительно = начало следующего календарного дня, исключая.
			end := timezone.NextDay(to)
			p.CreatedTo = &end
		}
	}
	return p, f
}

func (s *Service) List(ctx context.Context, a Actor, q ListQuery) (*TransferListResponse, error) {
	p, f := parseListQuery(q, a.Timezone)
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	p.Limit, p.Offset = limit, offset
	// скоуп исполнителя накладывает сервер: чужое направление фильтром → пустой список.
	p.ClientOnly = !a.IsAdmin
	items, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return &TransferListResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) Get(ctx context.Context, a Actor, rawID string) (*TransferResponse, error) {
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	if !a.canAccess(t.Direction) {
		// существование документа чужого направления не раскрывается (DS-042).
		return nil, ErrTransferNotFound
	}
	return t, nil
}

// head — направление и заказ документа с проверкой скоупа: документ чужого направления для
// исполнителя — «не найдено» (DS-042).
func (s *Service) head(ctx context.Context, a Actor, id int64) (string, *int64, error) {
	direction, taskID, err := s.repo.Head(ctx, id)
	if err != nil {
		return "", nil, mapErr(err)
	}
	if !a.canAccess(direction) {
		return "", nil, ErrTransferNotFound
	}
	return direction, taskID, nil
}

// Create — send: создать и отправить; receive: создать, отправить и принять (только приём от
// клиента, DS-013). Исполнитель создаёт только документы с клиентом, иначе 403 (DS-018).
func (s *Service) Create(ctx context.Context, a Actor, in Input, send, receive bool) (*TransferResponse, error) {
	in = in.normalize()
	if isDirection(in.Direction) && !a.canAccess(in.Direction) {
		return nil, ErrForbidden
	}
	f := append(in.validate(), createFlags(in.Direction, receive)...)
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	t, err := s.repo.Create(ctx, in, a.ID, send, receive)
	action := "create"
	switch {
	case receive:
		action = "create_receive"
	case send:
		action = "create_send"
	}
	return logged(ctx, t, err, action, a.ID)
}

// Update — правка заявки. direction / taskID — значения из тела: после создания не меняются,
// переданы → 400 not_allowed (§6.4); валидатор работает по направлению документа.
func (s *Service) Update(ctx context.Context, a Actor, rawID string, in Input, direction *string, taskID *int64, version *int) (*TransferResponse, error) {
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	docDirection, docTaskID, err := s.head(ctx, a, id)
	if err != nil {
		return nil, err
	}
	in.Direction, in.TaskID = docDirection, docTaskID
	in = in.normalize()
	f := append(immutableGuards(direction, taskID), in.validate()...)
	if version == nil {
		f = append(f, errorspkg.Field{Name: "version", Message: msgRequired})
	}
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	t, err := s.repo.Update(ctx, id, in, *version)
	return logged(ctx, t, err, actionUpdate, a.ID)
}

func (s *Service) Send(ctx context.Context, a Actor, rawID string, version *int) (*TransferResponse, error) {
	id, err := s.actionTarget(ctx, a, rawID, version)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Send(ctx, id, *version, a.ID)
	return logged(ctx, t, err, actionSend, a.ID)
}

func (s *Service) Receive(ctx context.Context, a Actor, rawID string, version *int, items []ReceiveItemInput) (*TransferResponse, error) {
	id, err := s.actionTarget(ctx, a, rawID, version)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Receive(ctx, id, *version, items, a.ID)
	return logged(ctx, t, err, actionReceive, a.ID)
}

func (s *Service) Cancel(ctx context.Context, a Actor, rawID string, version *int) (*TransferResponse, error) {
	id, err := s.actionTarget(ctx, a, rawID, version)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.Cancel(ctx, id, *version, a.ID)
	return logged(ctx, t, err, actionCancel, a.ID)
}

// Unreceive — «Отменить проведение» (transfers-revert DS-017): только администратор —
// исполнитель получает 403 для любого документа, в т.ч. с клиентом (DS-018; проверка роли
// первой — существование документа не раскрывается).
func (s *Service) Unreceive(ctx context.Context, a Actor, rawID string, version *int, reason string) (*TransferResponse, error) {
	if !a.IsAdmin {
		return nil, ErrForbidden
	}
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	reason, f := validateReason(reason)
	if version == nil {
		f = append([]errorspkg.Field{{Name: "version", Message: msgRequired}}, f...)
	}
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	t, err := s.repo.Unreceive(ctx, id, *version, reason, a.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	slog.InfoContext(ctx, "transfer mutated", "transfer_id", t.ID, "direction", t.Direction,
		"action", actionUnreceive, "user_id", a.ID, "reason", reason)
	return t, nil
}

// Delete — МЯГКОЕ удаление документа в статусе «Заявка» или «Отменён» (soft-delete-transfers
// §6.1, DS-018): только администратор — роль проверяется первой, до поиска записи (DS-003).
// Причина и version — из тела (DS-020, DS-028). Документ уходит в раздел «Удалённые».
func (s *Service) Delete(ctx context.Context, a Actor, rawID string, in TransferDeleteRequest) (*TransferDeleteResponse, error) {
	if !a.IsAdmin {
		return nil, ErrForbidden
	}
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	reason, f := softdelete.ValidateReason(in.Reason)
	version, vf := parseVersion(in.Version)
	if f = append(f, vf...); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	status, deletedAt, err := s.repo.SoftDelete(ctx, id, version, a.ID, reason)
	if err != nil {
		return nil, mapErr(err)
	}
	// Аудит (DS-030): факт удаления — в журнал приложения; причина в журнал не пишется.
	slog.InfoContext(ctx, "transfer deleted", "transfer_id", id, "status", status, "user_id", a.ID, "action", "delete")
	return &TransferDeleteResponse{ID: id, DeletedAt: deletedAt}, nil
}

// Restore — восстановление мягко удалённого документа из раздела «Удалённые»
// (soft-delete-transfers §6.2): только администратор, роль — первой (DS-003).
func (s *Service) Restore(ctx context.Context, a Actor, rawID string) (*TransferRestoreResponse, error) {
	if !a.IsAdmin {
		return nil, ErrForbidden
	}
	id, err := parseID(rawID)
	if err != nil {
		return nil, err
	}
	status, err := s.repo.Restore(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	slog.InfoContext(ctx, "transfer restored", "transfer_id", id, "status", status, "user_id", a.ID, "action", "restore")
	return &TransferRestoreResponse{ID: id}, nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка документа автотестами (soft-delete-transfers §6.3,
// DS-038): вид transfers адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард admin — в
// пакете board, существует только при TEST_PURGE_ENABLED=true). Документа нет → 404 NOT_FOUND
// (уборка не раскрывает вид записи); «в пути» / «получено» → 409 TRANSFER_STATUS_CONFLICT.
func (s *Service) Purge(ctx context.Context, rawID string) error {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return errorspkg.NotFound("Не найдено")
	}
	if err := s.repo.Purge(ctx, id); err != nil {
		if errors.Is(err, errNotFound) {
			return errorspkg.NotFound("Не найдено")
		}
		return mapErr(err)
	}
	return nil
}

// actionTarget — id документа перехода: скоуп исполнителя (чужое направление → 404, раньше
// проверки тела — существование не раскрывается), затем обязательный version.
func (s *Service) actionTarget(ctx context.Context, a Actor, rawID string, version *int) (int64, error) {
	id, err := parseID(rawID)
	if err != nil {
		return 0, err
	}
	if !a.IsAdmin {
		if _, _, err := s.head(ctx, a, id); err != nil {
			return 0, err
		}
	}
	if version == nil {
		return 0, errorspkg.Validation(errorspkg.Field{Name: "version", Message: msgRequired})
	}
	return id, nil
}

// logged — slog по успешной мутации документа (§3: id, действие, пользователь) + маппинг
// ошибок repo в wire-ошибки.
func logged(ctx context.Context, t *TransferResponse, err error, action, userID string) (*TransferResponse, error) {
	if err != nil {
		return nil, mapErr(err)
	}
	slog.InfoContext(ctx, "transfer mutated", "transfer_id", t.ID, "direction", t.Direction, "action", action, "user_id", userID)
	return t, nil
}

// parseID — #N перемещения: положительное целое, иначе 404 (как битый uuid у справочников).
func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrTransferNotFound
	}
	return id, nil
}

func isStatus(v string) bool {
	for _, s := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}

func mapErr(err error) error {
	var (
		fe   *fieldsError
		se   *statusError
		busy *equipmentBusyError
		nas  *notAtSourceError
		ub   *unreceiveBlockedError
		nit  *notInTaskError
		dr   *deletedRefError
	)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotFound):
		return ErrTransferNotFound
	case errors.Is(err, errVersion):
		return ErrVersionConflict
	case errors.Is(err, errTaskDeleted):
		return ErrTaskDeleted
	case errors.Is(err, errNotDeleted):
		return softdelete.ErrNotDeleted
	case errors.Is(err, errClientMismatch):
		return ErrClientMismatch
	case errors.As(err, &dr):
		return ErrHasDeletedRefs.WithDetail("kind", dr.kind).WithDetail("id", dr.id)
	case errors.As(err, &nit):
		return ErrEquipmentNotInTask.WithDetail("equipment_id", nit.equipmentID)
	case errors.As(err, &se):
		return statusConflict(se.action, se.status)
	case errors.Is(err, errOverflow):
		return ErrStockOverflow
	case errors.As(err, &busy) && busy.transferID == 0:
		// страховочная ветка guard-UPDATE'а: мешающий документ неизвестен — без номера.
		return errorspkg.New(codeEquipmentInTransfer, http.StatusConflict, "Оборудование уже в перемещении")
	case errors.As(err, &busy):
		e := errorspkg.New(codeEquipmentInTransfer, http.StatusConflict,
			fmt.Sprintf("Оборудование уже в перемещении #%d", busy.transferID))
		return e.WithDetail("transfer_id", busy.transferID).WithDetail("equipment_id", busy.equipmentID)
	case errors.As(err, &nas) && nas.fromClient:
		return ErrEquipmentNotAtClient.WithDetail("equipment_id", nas.equipmentID)
	case errors.As(err, &nas):
		return ErrEquipmentNotAtSource.WithDetail("equipment_id", nas.equipmentID)
	case errors.As(err, &ub):
		return unreceiveBlocked(ub)
	case errors.As(err, &fe):
		return errorspkg.Validation(fe.Fields...)
	}
	return err
}

// unreceiveBlocked — 409 TRANSFER_UNRECEIVE_BLOCKED с номером мешающего документа (§6.1).
func unreceiveBlocked(e *unreceiveBlockedError) error {
	msg := "Оборудование находится не там, куда его привезло это перемещение"
	switch {
	case e.transferID != 0 && e.later:
		msg = fmt.Sprintf("Сначала отмените проведение перемещения #%d", e.transferID)
	case e.transferID != 0:
		msg = fmt.Sprintf("Сначала отмените перемещение #%d", e.transferID)
	}
	out := errorspkg.New(codeUnreceiveBlocked, http.StatusConflict, msg).WithDetail("equipment_id", e.equipmentID)
	if e.transferID != 0 {
		out = out.WithDetail("transfer_id", e.transferID)
	}
	return out
}
