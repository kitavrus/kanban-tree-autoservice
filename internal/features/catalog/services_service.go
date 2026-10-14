package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

var ErrServiceNotFoundErr = errorspkg.New("SERVICE_NOT_FOUND", http.StatusNotFound, "Услуга не найдена")

// ErrServiceInUse — услуга используется в заказах (subtasks.service_id, миграция 0005,
// ADR-005). Только служебная уборка автотестов (DS-027): продуктовое удаление мягкое, заказы
// ему не мешают (DS-014).
var ErrServiceInUse = errorspkg.New("SERVICE_IN_USE", http.StatusConflict,
	"Услуга используется в заказах — удалить нельзя, отправьте её в архив")

type ServiceService struct {
	repo *ServiceRepo
}

func NewServiceService(pool *pgxpool.Pool) *ServiceService {
	return &ServiceService{repo: NewServiceRepo(pool)}
}

type ServiceListQuery struct {
	CategoryID      string
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

const serviceLookupDefaultLimit = 50 // API §Лимиты task.md: default 50 / max 500

// List — !isAdmin форсирует IncludeArchived=false (исполнитель видит только активные —
// D19). Комплекты всех услуг страницы — ОДНИМ запросом (repo.ProductsBatch,
// WHERE service_id = ANY($1)) — code review Sec 18/Серьёзное 1: раньше toResponse дёргал
// repo.Products на КАЖДУЮ услугу отдельно (N+1: до 501 round-trip на limit=500).
func (s *ServiceService) List(ctx context.Context, q ServiceListQuery, isAdmin bool) (*ServiceListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = serviceLookupDefaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	includeArchived := q.IncludeArchived && isAdmin

	categoryID := q.CategoryID
	if categoryID != "" && uuid.Validate(categoryID) != nil {
		categoryID = ""
	}
	query := strings.TrimSpace(q.Q)
	if sqlutil.HasInvalidText(query) {
		query = ""
	}
	items, total, err := s.repo.List(ctx, ServiceListParams{
		CategoryID: categoryID, Q: query, IncludeArchived: includeArchived, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	for i, sv := range items {
		ids[i] = sv.ID
	}
	productsByService, err := s.repo.ProductsBatch(ctx, ids)
	if err != nil {
		return nil, err
	}
	resp := &ServiceListResponse{Items: make([]ServiceResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, sv := range items {
		resp.Items[i] = toServiceResponse(sv, productsByService[sv.ID])
	}
	return resp, nil
}

// Get — as-is: архив не фильтрует ни для кого (D19 «активные записи» — про List), по
// прецеденту equipmenttypes.Service.Get (code review LOW#5).
func (s *ServiceService) Get(ctx context.Context, id string) (*ServiceResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrServiceNotFoundErr
	}
	sv, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrServiceNotFound) {
		return nil, ErrServiceNotFoundErr
	}
	if err != nil {
		return nil, err
	}
	r, err := s.toResponse(ctx, *sv)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// prepareProducts — validate() построчно (индексы = исходные строки, форма/API-клиент
// видят именно их), затем mergeProducts() дедупликация суммой qty ОТДЕЛЬНО, только над
// уже валидными строками (code review Medium#3). in.Products НЕ заменяется мерджем —
// repo.Create/Update получает и исходный список (для checkProducts — существование/
// архивность проверяется по позиции в ИСХОДНОМ списке, не смерженном, code review
// fix-loop 2 Medium: раньше индекс errServiceProductInvalid считался по смерженному
// списку, а SSR-форма/API-клиент показывают исходные строки — [A,A,B(архивный)] после
// мерджа даёт [A(qty=сумма),B], и ошибка «products[1]» подсвечивала бы позицию B в
// СМЕРЖЕННОМ списке, что при рендере исходных строк указывает на вторую A, а не на B),
// и смерженный — для фактической записи в service_products.
func prepareProducts(in ServiceInput) (ServiceInput, []ServiceProductInput, error) {
	if f := in.validate(); len(f) > 0 {
		return in, nil, errorspkg.Validation(f...)
	}
	merged, mergeErrs := mergeProducts(in.Products)
	if len(mergeErrs) > 0 {
		return in, nil, errorspkg.Validation(mergeErrs...)
	}
	return in, merged, nil
}

func (s *ServiceService) Create(ctx context.Context, in ServiceInput) (*ServiceResponse, error) {
	in = in.normalize()
	in, merged, err := prepareProducts(in)
	if err != nil {
		return nil, err
	}
	sv, err := s.repo.Create(ctx, in, merged)
	if err != nil {
		return nil, mapServiceWriteErr(err)
	}
	r, err := s.toResponse(ctx, *sv)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Update — полная замена карточки+комплекта (last-write-wins, D28 — нет version).
// Товар, уже присутствовавший в ТЕКУЩЕМ комплекте этой услуги, допустим даже архивным
// (repo.Update передаёт id услуги дальше для чтения «старого» комплекта в транзакции) —
// code review Medium#2: task.md запрещает архивный «при добавлении», не при пересохранении
// уже существующей строки (иначе любое сохранение услуги, чей товар с тех пор
// заархивирован по прецеденту PRODUCT_IN_USE, стало бы невозможным).
func (s *ServiceService) Update(ctx context.Context, id string, in ServiceInput) (*ServiceResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrServiceNotFoundErr
	}
	in = in.normalize()
	in, merged, err := prepareProducts(in)
	if err != nil {
		return nil, err
	}
	sv, err := s.repo.Update(ctx, id, in, merged)
	if errors.Is(err, ErrServiceNotFound) {
		return nil, ErrServiceNotFoundErr
	}
	if err != nil {
		return nil, mapServiceWriteErr(err)
	}
	r, err := s.toResponse(ctx, *sv)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *ServiceService) SetActive(ctx context.Context, id string, active bool) error {
	if uuid.Validate(id) != nil {
		return ErrServiceNotFoundErr
	}
	if err := s.repo.SetActive(ctx, id, active); errors.Is(err, ErrServiceNotFound) {
		return ErrServiceNotFoundErr
	} else if err != nil {
		return err
	}
	return nil
}

// mapServiceWriteErr — категория/строка комплекта невалидны → 400 по конкретному полю
// (task.md: «архивный/несуществующий товар → 400 по полю строки»); 22003 — страховка,
// не должна встречаться после validateAmount() в validate()/mergeProducts() (code review
// Блокер 1).
func mapServiceWriteErr(err error) error {
	switch {
	case errors.Is(err, errServiceCategoryInvalid):
		return errorspkg.Validation(errorspkg.Field{Name: "category_id", Message: msgInvalid})
	case errors.Is(err, errCategoryDeleted):
		return errorspkg.Validation(errorspkg.Field{Name: "category_id", Message: msgNotFound})
	case errors.Is(err, errAmountOutOfRange):
		return ErrAmountOutOfRange
	}
	var pe *errServiceProductInvalid
	if errors.As(err, &pe) {
		msg := msgInvalid
		if pe.Deleted {
			msg = msgNotFound
		}
		return errorspkg.Validation(errorspkg.Field{Name: fmt.Sprintf("products[%d].product_id", pe.Index), Message: msg})
	}
	return err
}

// toResponse — комплект дозагружается отдельным запросом (re-fetch-after-write —
// репозиторий не возвращает JOIN-поля товаров из транзакции записи). Для списков —
// toServiceResponse напрямую с батч-загруженными rows (см. List выше, N+1).
func (s *ServiceService) toResponse(ctx context.Context, sv Service) (ServiceResponse, error) {
	rows, err := s.repo.Products(ctx, sv.ID)
	if err != nil {
		return ServiceResponse{}, err
	}
	return toServiceResponse(sv, rows), nil
}

func toServiceResponse(sv Service, rows []ServiceProductRow) ServiceResponse {
	resp := ServiceResponse{
		ID: sv.ID, CategoryID: sv.CategoryID, CategoryIsDeleted: sv.CategoryIsDeleted, Name: sv.Name, IsActive: sv.IsActive,
		Products: make([]ServiceProductResponse, len(rows)), CreatedAt: sv.CreatedAt, UpdatedAt: sv.UpdatedAt,
	}
	if sv.Price != nil {
		a := Amount(*sv.Price)
		resp.Price = &a
	}
	for i, row := range rows {
		resp.Products[i] = ServiceProductResponse{ProductID: row.ProductID, ProductIsDeleted: row.IsDeleted, Name: row.Name, Unit: row.Unit, Price: Amount(row.Price), Qty: Amount(row.Qty)}
	}
	return resp
}
