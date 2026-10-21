// Package service — бизнес-правила закупок (§4–§6): права по ролям (DS-050, DS-051, DS-060),
// матрица переходов §5, сброс ответов при правке (DS-034, DS-048), «Собрано» (DS-038).
// Проверки выполняются ВНУТРИ транзакции repository над заблокированной шапкой заказа.
package service

import (
	"context"
	"errors"
	"log/slog"
	"math/big"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/mappers"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/repository"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/validators"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Роли — единый источник internal/server/middleware (без дублирования строк).
const (
	RoleAdmin    = middleware.RoleAdmin
	RoleExecutor = middleware.RoleExecutor
	RoleSupplier = middleware.RoleSupplier
)

// Actor — кто делает запрос (только из сессии, DS-053).
type Actor struct {
	ID         string
	Role       string
	SupplierID string
}

func (a Actor) isAdmin() bool { return a.Role == RoleAdmin }

// owner — фильтр автора: executor видит только свои заказы (DS-060), admin — все ("").
func (a Actor) owner() string {
	if a.isAdmin() {
		return ""
	}
	return a.ID
}

type Service struct {
	repo *repository.Repository
	firm config.FirmInfo
}

func New(repo *repository.Repository, firm config.FirmInfo) *Service {
	return &Service{repo: repo, firm: firm}
}

// Firm — реквизиты мастерской (кабинет и PDF, DS-046).
func (s *Service) Firm() config.FirmInfo { return s.firm }

func staffOnly(a Actor) error {
	if a.Role != RoleAdmin && a.Role != RoleExecutor {
		return errorspkg.ErrPurchaseOrderForbidden
	}
	return nil
}

func supplierOnly(a Actor) error {
	if a.Role != RoleSupplier || a.SupplierID == "" {
		return errorspkg.ErrPurchaseSupplierCabinetOnly
	}
	return nil
}

func mapNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return errorspkg.ErrPurchaseOrderNotFound
	}
	return err
}

// ListQuery — сырые query-параметры списков.
type ListQuery struct {
	Status, Limit, Offset string
}

// List — §6.1.
func (s *Service) List(ctx context.Context, a Actor, q ListQuery) (*dto.ListResponse, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	p, f := validators.ValidatePage("status", q.Status, constants.Statuses, q.Limit, q.Offset,
		constants.ListDefaultLimit, constants.ListMaxLimit)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	rows, total, err := s.repo.List(ctx, repository.ListFilter{Status: p.Filter, CreatedBy: a.owner(), Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		return nil, err
	}
	resp := &dto.ListResponse{Items: make([]dto.ListItem, 0, len(rows)), Total: total, Limit: p.Limit, Offset: p.Offset}
	for i := range rows {
		resp.Items = append(resp.Items, mappers.ToListItem(&rows[i]))
	}
	return resp, nil
}

// Model — заказ со строками для карточки и PDF §6.11 (executor — только свой).
func (s *Service) Model(ctx context.Context, a Actor, id string) (*models.PurchaseOrder, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	o, err := s.repo.Get(ctx, id, a.owner())
	if err != nil {
		return nil, mapNotFound(err)
	}
	return o, nil
}

// Get — §6.3.
func (s *Service) Get(ctx context.Context, a Actor, id string) (*dto.Order, error) {
	o, err := s.Model(ctx, a, id)
	if err != nil {
		return nil, err
	}
	return mappers.ToOrder(o), nil
}

// reread — карточка после изменения (ответ §6.2–§6.9).
func (s *Service) reread(ctx context.Context, id string) (*dto.Order, error) {
	o, err := s.repo.Get(ctx, id, "")
	if err != nil {
		return nil, mapNotFound(err)
	}
	return mappers.ToOrder(o), nil
}

func inputs(lines []validators.Line) []models.ItemInput {
	out := make([]models.ItemInput, len(lines))
	for i, l := range lines {
		out[i] = l.ItemInput
	}
	return out
}

// productErr — недоступные запчасти → 400 items[i].product_id: unavailable (DS-055).
func productErr(err error, lines []validators.Line) error {
	var pe *repository.ProductsUnavailableError
	if !errors.As(err, &pe) {
		return err
	}
	var f []errorspkg.Field
	for _, l := range lines {
		if l.ID == "" && pe.ProductIDs[l.ProductID] {
			f = append(f, errorspkg.Field{Name: validators.ItemField(l.Index, "product_id"), Message: validators.MsgUnavailable})
		}
	}
	return validators.Error(f)
}

// Create — черновик §6.2; автор — из сессии.
func (s *Service) Create(ctx context.Context, a Actor, req dto.CreateRequest) (*dto.Order, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	in, f := validators.ValidateCreate(req)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	id, err := s.repo.Create(ctx, a.ID, in.SupplierID, in.Note, inputs(in.Lines))
	if errors.Is(err, repository.ErrSupplierUnavailable) {
		return nil, validators.Error([]errorspkg.Field{{Name: "supplier_id", Message: validators.MsgUnavailable}})
	}
	if err != nil {
		return nil, productErr(err, in.Lines)
	}
	slog.InfoContext(ctx, "purchase order created", "id", id, "user_id", a.ID)
	return s.reread(ctx, id)
}

// own — executor трогает только свои заказы: чужой → 404, как будто заказа нет (DS-051).
func own(a Actor, h models.Head) error {
	if !a.isAdmin() && h.CreatedBy != a.ID {
		return errorspkg.ErrPurchaseOrderNotFound
	}
	return nil
}

func version(h models.Head, v int64) error {
	if h.Version != v {
		return errorspkg.ErrPurchaseOrderVersionConflict
	}
	return nil
}

// UpdateCheck — правило правки §6.4 (экспорт для unit-тестов матрицы).
func UpdateCheck(a Actor, v int64, lines []validators.Line) repository.UpdateCheck {
	return func(h models.Head, existing []models.ExistingItem) error {
		if err := own(a, h); err != nil {
			return err
		}
		switch h.Status {
		case constants.StatusReady:
			return errorspkg.ErrPurchaseOrderReadOnlyReady
		case constants.StatusPickedUp:
			return errorspkg.ErrPurchaseOrderReadOnlyClosed
		case constants.StatusCancelled:
			return errorspkg.ErrPurchaseOrderReadOnlyCancelled
		}
		if err := version(h, v); err != nil {
			return err
		}
		byID := make(map[string]string, len(existing))
		for _, e := range existing {
			byID[e.ID] = e.ProductID
		}
		var f []errorspkg.Field
		for _, l := range lines {
			if l.ID == "" {
				continue
			}
			if pid, ok := byID[l.ID]; !ok || pid != l.ProductID {
				f = append(f, errorspkg.Field{Name: validators.ItemField(l.Index, "id"), Message: validators.MsgInvalid})
			}
		}
		if len(f) > 0 {
			return validators.Error(f)
		}
		return nil
	}
}

// Update — §6.4.
func (s *Service) Update(ctx context.Context, a Actor, id string, req dto.UpdateRequest) (*dto.Order, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	in, f := validators.ValidateUpdate(req)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	err := s.repo.Update(ctx, id, in.Note, inputs(in.Lines), UpdateCheck(a, in.Version, in.Lines))
	if err != nil {
		return nil, productErr(mapNotFound(err), in.Lines)
	}
	slog.InfoContext(ctx, "purchase order updated", "id", id, "user_id", a.ID)
	return s.reread(ctx, id)
}

// SendCheck — draft → sent (§5, §6.6).
func SendCheck(a Actor, v int64) repository.HeadCheck {
	return func(h models.Head) error {
		if err := own(a, h); err != nil {
			return err
		}
		switch h.Status {
		case constants.StatusDraft:
		case constants.StatusSent:
			return errorspkg.ErrPurchaseOrderAlreadySent
		default:
			return errorspkg.ErrPurchaseOrderSendNotDraft
		}
		if h.IsBazaar() {
			return errorspkg.ErrPurchaseOrderNoSupplier
		}
		if err := version(h, v); err != nil {
			return err
		}
		if h.ItemsCount == 0 {
			return errorspkg.ErrPurchaseOrderEmpty
		}
		// Поставщик ушёл в архив/удалён — тот же отказ, что при создании (§6.2, DS-043).
		if !h.SupplierAvailable {
			return validators.Error([]errorspkg.Field{{Name: "supplier_id", Message: validators.MsgUnavailable}})
		}
		return nil
	}
}

// RecallCheck — sent → draft (§6.7).
func RecallCheck(a Actor, v int64) repository.HeadCheck {
	return func(h models.Head) error {
		if err := own(a, h); err != nil {
			return err
		}
		switch h.Status {
		case constants.StatusSent:
		case constants.StatusReady:
			return errorspkg.ErrPurchaseOrderRecallReady
		default:
			return errorspkg.ErrPurchaseOrderRecallNotSent
		}
		return version(h, v)
	}
}

// PickUpCheck — «Забрали» (§6.8): ready; Базар из draft; поставщик без активного входа из sent
// (DS-042, DS-059).
func PickUpCheck(a Actor, v int64) repository.HeadCheck {
	return func(h models.Head) error {
		if err := own(a, h); err != nil {
			return err
		}
		switch h.Status {
		case constants.StatusReady:
		case constants.StatusDraft:
			if !h.IsBazaar() {
				return errorspkg.ErrPurchaseOrderNotReadySendFirst
			}
		case constants.StatusSent:
			if h.SupplierHasLogin {
				return errorspkg.ErrPurchaseOrderNotReadyWait
			}
		case constants.StatusPickedUp:
			return errorspkg.ErrPurchaseOrderAlreadyClosed
		default:
			return errorspkg.ErrPurchaseOrderIsCancelled
		}
		if err := version(h, v); err != nil {
			return err
		}
		if h.ItemsCount == 0 {
			return errorspkg.ErrPurchaseOrderEmpty
		}
		return nil
	}
}

// CancelCheck — draft/sent/ready → cancelled (§6.9).
func CancelCheck(a Actor, v int64) repository.HeadCheck {
	return func(h models.Head) error {
		if err := own(a, h); err != nil {
			return err
		}
		switch h.Status {
		case constants.StatusPickedUp:
			return errorspkg.ErrPurchaseOrderCancelClosed
		case constants.StatusCancelled:
			return errorspkg.ErrPurchaseOrderAlreadyCancelled
		}
		return version(h, v)
	}
}

// DeleteCheck — мягкое удаление только черновика (§6.5).
func DeleteCheck(v int64) repository.HeadCheck {
	return func(h models.Head) error {
		if h.Status != constants.StatusDraft {
			return errorspkg.ErrPurchaseOrderDeleteNotDraft
		}
		return version(h, v)
	}
}

func (s *Service) versioned(ctx context.Context, a Actor, id string, v int64, action string,
	run func(id string) error) (*dto.Order, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	if f := validators.ValidateVersion(v); len(f) > 0 {
		return nil, validators.Error(f)
	}
	if err := run(id); err != nil {
		return nil, mapNotFound(err)
	}
	slog.InfoContext(ctx, "purchase order "+action, "id", id, "user_id", a.ID)
	return s.reread(ctx, id)
}

// Send — §6.6.
func (s *Service) Send(ctx context.Context, a Actor, id string, req dto.VersionRequest) (*dto.Order, error) {
	return s.versioned(ctx, a, id, req.Version, "sent", func(id string) error {
		return s.repo.Send(ctx, id, a.ID, SendCheck(a, req.Version))
	})
}

// Recall — §6.7.
func (s *Service) Recall(ctx context.Context, a Actor, id string, req dto.VersionRequest) (*dto.Order, error) {
	return s.versioned(ctx, a, id, req.Version, "recalled", func(id string) error {
		return s.repo.Recall(ctx, id, RecallCheck(a, req.Version))
	})
}

// PickUp — §6.8.
func (s *Service) PickUp(ctx context.Context, a Actor, id string, req dto.VersionRequest) (*dto.Order, error) {
	return s.versioned(ctx, a, id, req.Version, "picked up", func(id string) error {
		return s.repo.PickUp(ctx, id, a.ID, PickUpCheck(a, req.Version))
	})
}

// Cancel — §6.9: причина 1–500 (DS-036).
func (s *Service) Cancel(ctx context.Context, a Actor, id string, req dto.ReasonRequest) (*dto.Order, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	reason, f := validators.ValidateReason(req.Version, req.Reason)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	if err := s.repo.Cancel(ctx, id, a.ID, reason, CancelCheck(a, req.Version)); err != nil {
		return nil, mapNotFound(err)
	}
	slog.InfoContext(ctx, "purchase order cancelled", "id", id, "user_id", a.ID)
	return s.reread(ctx, id)
}

// Delete — §6.5: только admin, роль проверяется первой (до поиска записи).
func (s *Service) Delete(ctx context.Context, a Actor, id string, req dto.ReasonRequest) error {
	if !a.isAdmin() {
		return errorspkg.ErrPurchaseOrderDeleteAdmin
	}
	if uuid.Validate(id) != nil {
		return errorspkg.ErrPurchaseOrderNotFound
	}
	reason, f := validators.ValidateReason(req.Version, req.Reason)
	if len(f) > 0 {
		return validators.Error(f)
	}
	if err := s.repo.SoftDelete(ctx, id, a.ID, reason, DeleteCheck(req.Version)); err != nil {
		return mapNotFound(err)
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindPurchaseOrders), "id", id, "user_id", a.ID, "action", "delete")
	return nil
}

// Restore — POST /api/v1/purchase-orders/:id/restore (раздел «Удалённые», только admin, DS-050).
func (s *Service) Restore(ctx context.Context, a Actor, id string) error {
	if !a.isAdmin() {
		return errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return errorspkg.ErrPurchaseOrderNotFound
	}
	err := s.repo.Restore(ctx, id)
	switch {
	case errors.Is(err, repository.ErrNotDeleted):
		return softdelete.ErrNotDeleted
	case errors.Is(err, repository.ErrRestoreSupplierUnavailable):
		return errorspkg.ErrPurchaseOrderRestoreSupplierUnavailable
	case err != nil:
		return mapNotFound(err)
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindPurchaseOrders), "id", id, "user_id", a.ID, "action", "restore")
	return nil
}

// Purge — вид purchase-orders служебной уборки DELETE /api/v1/test/purge/:kind/:id
// (TEST_PURGE_ENABLED, soft-delete-core DS-027).
func (s *Service) Purge(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	if err := s.repo.Purge(ctx, id); errors.Is(err, repository.ErrNotFound) {
		return errorspkg.NotFound("Не найдено")
	} else if err != nil {
		return err
	}
	return nil
}

// SuggestionsQuery — сырые параметры §6.10.
type SuggestionsQuery struct {
	Reason, Limit, Offset string
}

// Suggestions — §6.10.
func (s *Service) Suggestions(ctx context.Context, a Actor, q SuggestionsQuery) (*dto.SuggestionsResponse, error) {
	if err := staffOnly(a); err != nil {
		return nil, err
	}
	p, f := validators.ValidatePage("reason", q.Reason, []string{constants.ReasonForOrders, constants.ReasonBelowMin},
		q.Limit, q.Offset, constants.SuggestionsDefaultLimit, constants.SuggestionsMaxLimit)
	if len(f) > 0 {
		e := validators.Error(f)
		if f[0].Name == "reason" {
			e.Message = "Неизвестный тип строки"
		}
		return nil, e
	}
	rows, total, err := s.repo.Suggestions(ctx, p.Filter, a.owner(), p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	resp := &dto.SuggestionsResponse{Items: make([]dto.Suggestion, 0, len(rows)), Total: total, Limit: p.Limit, Offset: p.Offset}
	for i := range rows {
		resp.Items = append(resp.Items, mappers.ToSuggestion(&rows[i]))
	}
	return resp, nil
}

// ---- кабинет поставщика ----

// SupplierList — §6.12.
func (s *Service) SupplierList(ctx context.Context, a Actor, q ListQuery) (*dto.SupplierListResponse, error) {
	if err := supplierOnly(a); err != nil {
		return nil, err
	}
	p, f := validators.ValidatePage("status", q.Status, constants.SupplierStatuses, q.Limit, q.Offset,
		constants.ListDefaultLimit, constants.ListMaxLimit)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	rows, total, err := s.repo.SupplierList(ctx, a.SupplierID, p.Filter, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	resp := &dto.SupplierListResponse{Items: make([]dto.SupplierListItem, 0, len(rows)), Total: total, Limit: p.Limit, Offset: p.Offset}
	for i := range rows {
		resp.Items = append(resp.Items, mappers.ToSupplierListItem(&rows[i]))
	}
	return resp, nil
}

// SupplierModel — заказ кабинета со строками (карточка §6.13 и PDF §6.16).
func (s *Service) SupplierModel(ctx context.Context, a Actor, id string) (*models.PurchaseOrder, error) {
	if err := supplierOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	o, err := s.repo.SupplierGet(ctx, a.SupplierID, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return o, nil
}

// SupplierGet — §6.13.
func (s *Service) SupplierGet(ctx context.Context, a Actor, id string) (*dto.SupplierOrder, error) {
	o, err := s.SupplierModel(ctx, a, id)
	if err != nil {
		return nil, err
	}
	return mappers.ToSupplierOrder(o, s.firm.Name, s.firm.Phone), nil
}

// AnswerCheck — ответ по строке только в sent; версия строки (§6.14, DS-020, DS-032).
func AnswerCheck(v int64) repository.AnswerCheck {
	return func(h models.Head, it models.AnswerState) error {
		switch h.Status {
		case constants.StatusSent:
		case constants.StatusReady:
			return errorspkg.ErrPurchaseAnswerReadOnlyReady
		case constants.StatusPickedUp:
			return errorspkg.ErrPurchaseAnswerReadOnlyPickedUp
		default:
			return errorspkg.ErrPurchaseAnswerReadOnlyRecalled
		}
		if it.Version != v {
			return errorspkg.ErrPurchaseItemVersionConflict
		}
		return nil
	}
}

// Answer — §6.14.
func (s *Service) Answer(ctx context.Context, a Actor, orderID, itemID string, req dto.AnswerRequest) (*dto.AnswerResponse, error) {
	if err := supplierOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(orderID) != nil || uuid.Validate(itemID) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	in, f := validators.ValidateAnswer(req)
	if len(f) > 0 {
		return nil, validators.Error(f)
	}
	it, total, err := s.repo.Answer(ctx, a.SupplierID, orderID, itemID, a.ID, in, AnswerCheck(in.Version))
	if err != nil {
		return nil, mapNotFound(err)
	}
	slog.InfoContext(ctx, "purchase order answered", "id", orderID, "item_id", itemID, "user_id", a.ID)
	return &dto.AnswerResponse{SupplierItem: mappers.ToSupplierItem(it), TotalAmount: total}, nil
}

func positive(s *string) bool {
	if s == nil {
		return false
	}
	r, ok := new(big.Rat).SetString(*s)
	return ok && r.Sign() > 0
}

// ReadyCheck — «Собрано» (§6.15, DS-038): все строки с «Соберу»; у строк с количеством > 0 —
// цена. Черновик (отозван) и отменённый — 404, как будто заказа нет в кабинете.
func ReadyCheck() repository.ReadyCheck {
	return func(h models.Head, items []models.AnswerState) error {
		switch h.Status {
		case constants.StatusSent:
		case constants.StatusReady:
			return errorspkg.ErrPurchaseOrderAlreadyReady
		case constants.StatusPickedUp:
			return errorspkg.ErrPurchaseOrderAlreadyPickedUp
		default:
			return errorspkg.ErrPurchaseOrderNotFound
		}
		var noQty, noPrice []string
		for _, it := range items {
			switch {
			case it.SupplierQty == nil:
				noQty = append(noQty, it.ItemID)
			case positive(it.SupplierQty) && it.SupplierPrice == nil:
				noPrice = append(noPrice, it.ItemID)
			}
		}
		if len(noQty) > 0 {
			return errorspkg.ErrPurchaseOrderItemsNoQty.WithDetail("item_ids", noQty)
		}
		if len(noPrice) > 0 {
			return errorspkg.ErrPurchaseOrderItemsNoPrice.WithDetail("item_ids", noPrice)
		}
		if len(items) == 0 {
			return errorspkg.ErrPurchaseOrderEmpty
		}
		return nil
	}
}

// Ready — §6.15.
func (s *Service) Ready(ctx context.Context, a Actor, id string) (*dto.SupplierOrder, error) {
	if err := supplierOnly(a); err != nil {
		return nil, err
	}
	if uuid.Validate(id) != nil {
		return nil, errorspkg.ErrPurchaseOrderNotFound
	}
	if err := s.repo.Ready(ctx, a.SupplierID, id, a.ID, ReadyCheck()); err != nil {
		return nil, mapNotFound(err)
	}
	slog.InfoContext(ctx, "purchase order ready", "id", id, "user_id", a.ID)
	return s.SupplierGet(ctx, a, id)
}

// ActiveSuppliers — получатели экрана «Новый заказ» (§8.2).
func (s *Service) ActiveSuppliers(ctx context.Context) ([]repository.SupplierOption, error) {
	return s.repo.ActiveSuppliers(ctx)
}
