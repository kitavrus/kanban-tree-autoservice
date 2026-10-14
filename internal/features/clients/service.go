package clients

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

var (
	ErrClientNotFound        = errorspkg.New("CLIENT_NOT_FOUND", http.StatusNotFound, "Клиент не найден")
	ErrEquipmentNotFound     = errorspkg.New("EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено")
	ErrClientVersionConflict = errorspkg.New("CLIENT_VERSION_CONFLICT", http.StatusConflict,
		"Карточку уже изменил другой пользователь — ваши правки не сохранены")
)

const (
	defaultLimit = 50
	maxLimit     = 200
	// minSearchDigits — ADR-003: строка поиска трактуется как телефон (сравнение по
	// цифрам), если после отбрасывания нецифр остаётся ≥3 цифры.
	minSearchDigits = 3

	// equipmentDefaultLimit/equipmentMaxLimit — оборудование одного клиента, список
	// не постраничный по UX, но лимит защищает от неограниченной выборки.
	equipmentDefaultLimit = 200
	equipmentMaxLimit     = 200
)

var (
	// phoneLikeRe — строка похожа на телефонный ввод (цифры/+()-/пробел), не текст.
	phoneLikeRe = regexp.MustCompile(`^[0-9+()\- ]+$`)
	nonDigitRe  = regexp.MustCompile(`\D`)
)

type Service struct {
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepo(pool)}
}

// ListQuery — параметры списка из HTTP-слоя (сырые).
type ListQuery struct {
	Q             string
	Limit, Offset int
}

func (s *Service) List(ctx context.Context, q ListQuery) (*ClientListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)

	query := strings.TrimSpace(q.Q)
	if sqlutil.HasInvalidText(query) {
		// NUL-байт / невалидный UTF-8 в q (напр. GET /api/v1/clients?q=%00, доступно и
		// исполнителю) — не 500: фильтр молча игнорируется, как будто q не задан (200 с
		// полным списком), а не ошибка формы поиска по адресной строке (code review §TD-4).
		query = ""
	}
	digits, digitsAlt := searchDigits(query)
	items, total, err := s.repo.ListClients(ctx, ListParams{
		Q: query, Digits: digits, DigitsAlt: digitsAlt, PlateLike: sqlutil.PlatePattern(query),
		Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	resp := &ClientListResponse{Items: make([]ClientResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, c := range items {
		resp.Items[i] = toResponse(c)
	}
	return resp, nil
}

// searchDigits — ADR-003: строка похожа на телефон и содержит ≥3 цифры → цифры для
// прямого сравнения с БД (телефон без нецифровых символов через regexp_replace, затем
// LIKE '%<digits>%'); иначе
// оба возврата — "". digitsAlt (непусто, только если digits начинается с 7 или 8) — та же
// строка БЕЗ ведущей цифры, для СИММЕТРИЧНОГО сравнения repo.go: у номера в БД точно так же
// отбрасывается ведущая 7/8 (`phoneAltDigitsExpr`), поэтому «8999123»/«+7999123» находят
// друг друга независимо от длины ввода (частичный ввод, не только полный 11-значный номер —
// code review, регрессия итерации 1: сравнение digits ПОЛНОЙ длины ≥11 ломало короткий
// частичный ввод с кодом страны, т.к. БД-сторона стриплась всегда, а запрос — только при
// ≥11 цифрах).
func searchDigits(q string) (digits, digitsAlt string) {
	if q == "" || !phoneLikeRe.MatchString(q) {
		return "", ""
	}
	digits = nonDigitRe.ReplaceAllString(q, "")
	if len(digits) < minSearchDigits {
		return "", ""
	}
	// alt без ведущей 7/8 — только если в нём остаётся ≥ minSearchDigits цифр,
	// иначе «755» искал бы по «55» и находил лишнее.
	if (digits[0] == '7' || digits[0] == '8') && len(digits)-1 >= minSearchDigits {
		digitsAlt = digits[1:]
	}
	return digits, digitsAlt
}

func (s *Service) Get(ctx context.Context, id string) (*ClientResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrClientNotFound
	}
	c, err := s.repo.GetClient(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toResponse(*c)
	return &resp, nil
}

func (s *Service) Create(ctx context.Context, in ClientInput) (*ClientResponse, error) {
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	c, err := s.repo.CreateClient(ctx, in)
	if err != nil {
		return nil, err
	}
	resp := toResponse(*c)
	return &resp, nil
}

// Update — полная замена карточки + набора телефонов; version nil → 400 (ADR-100),
// version не совпал с текущим → 409 CLIENT_VERSION_CONFLICT (не перезапись).
func (s *Service) Update(ctx context.Context, id string, version *int64, in ClientInput) (*ClientResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrClientNotFound
	}
	in = in.normalize()
	f := in.validate()
	if version == nil {
		f = append(f, errorspkg.Field{Name: "version", Message: msgRequired})
	}
	if len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	c, err := s.repo.UpdateClient(ctx, id, *version, in)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrClientNotFound
	case errors.Is(err, errVersionConflict):
		return nil, ErrClientVersionConflict
	case err != nil:
		return nil, err
	}
	resp := toResponse(*c)
	return &resp, nil
}

func toResponse(c Client) ClientResponse {
	phones := c.Phones
	if phones == nil {
		phones = []PhoneDTO{}
	}
	return ClientResponse{
		ID: c.ID, CompanyName: c.CompanyName,
		Phones: phones, EquipmentCount: c.EquipmentCount, Version: c.Version,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// ListEquipment — 404 CLIENT_NOT_FOUND, если клиента нет (проверка до запроса, а не
// пустой список — design-sql.md `## List Equipment By Client Query`).
func (s *Service) ListEquipment(ctx context.Context, clientID string, limit, offset int) (*EquipmentListResponse, error) {
	if uuid.Validate(clientID) != nil {
		return nil, ErrClientNotFound
	}
	exists, err := s.repo.ClientExists(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrClientNotFound
	}

	if limit <= 0 {
		limit = equipmentDefaultLimit
	}
	limit = min(limit, equipmentMaxLimit)
	offset = max(offset, 0)

	items, total, err := s.repo.ListEquipment(ctx, clientID, limit, offset)
	if err != nil {
		return nil, err
	}
	resp := &EquipmentListResponse{Items: make([]EquipmentResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, e := range items {
		resp.Items[i] = toEquipmentResponse(e)
	}
	return resp, nil
}

func (s *Service) GetEquipment(ctx context.Context, id string) (*EquipmentResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrEquipmentNotFound
	}
	e, err := s.repo.GetEquipment(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrEquipmentNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toEquipmentResponse(*e)
	return &resp, nil
}

// CreateEquipment — validate → repo.CreateEquipment (клиент удалён параллельно → 404
// CLIENT_NOT_FOUND, не 500).
func (s *Service) CreateEquipment(ctx context.Context, clientID string, in EquipmentInput) (*EquipmentResponse, error) {
	if uuid.Validate(clientID) != nil {
		return nil, ErrClientNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	e, err := s.repo.CreateEquipment(ctx, clientID, in)
	switch {
	case errors.Is(err, errClientGone):
		return nil, ErrClientNotFound
	case err != nil:
		return nil, err
	}
	resp := toEquipmentResponse(*e)
	return &resp, nil
}

// UpdateEquipment — LWW (без version, D28 — версия только у clients).
func (s *Service) UpdateEquipment(ctx context.Context, id string, in EquipmentInput) (*EquipmentResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrEquipmentNotFound
	}
	in = in.normalize()
	if f := in.validate(); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}

	e, err := s.repo.UpdateEquipment(ctx, id, in)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrEquipmentNotFound
	case err != nil:
		return nil, err
	}
	resp := toEquipmentResponse(*e)
	return &resp, nil
}

func toEquipmentResponse(e Equipment) EquipmentResponse {
	return EquipmentResponse{ID: e.ID, ClientID: e.ClientID, PlateNumber: e.PlateNumber, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}

// lookupDefaultLimit/lookupMaxLimit — поиск автомобиля по госномеру: совпадений единицы
// (перепроданный автомобиль у нескольких клиентов), лимит — защита от неограниченной выборки.
const (
	lookupDefaultLimit = 20
	lookupMaxLimit     = 50
)

// EquipmentLookupQuery — фильтры GET /api/v1/equipment (все — по AND).
type EquipmentLookupQuery struct {
	PlateNumber string // точное совпадение госномера (без учёта регистра/пробелов/дефисов)
	Q           string // фрагмент госномера (подстрока, без учёта регистра/пробелов/дефисов)
	ClientID    string // автомобили клиента
	Limit       int
	Offset      int
}

// LookupEquipment — GET /api/v1/equipment?plate_number=|q=|client_id=: только действующие
// единицы действующих клиентов (диалог «Новый заказ»: точный поиск по госномеру,
// автодополнение по фрагменту, автомобили выбранного клиента). Хотя бы один фильтр
// обязателен (список всего оборудования этот адрес не отдаёт) → иначе 400
// plate_number/q/client_id: required. При q — сначала совпадения с начала номера.
func (s *Service) LookupEquipment(ctx context.Context, in EquipmentLookupQuery) (*EquipmentLookupResponse, error) {
	plate := NormalizePlateNumber(in.PlateNumber)
	q := NormalizePlateNumber(in.Q)
	clientID := strings.TrimSpace(in.ClientID)
	var fields []errorspkg.Field
	if sqlutil.HasInvalidText(plate) {
		fields = append(fields, errorspkg.Field{Name: "plate_number", Message: msgInvalidText})
	}
	if sqlutil.HasInvalidText(q) {
		fields = append(fields, errorspkg.Field{Name: "q", Message: msgInvalidText})
	}
	if clientID != "" && uuid.Validate(clientID) != nil {
		fields = append(fields, errorspkg.Field{Name: "client_id", Message: msgInvalid})
	}
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	f := equipmentLookupFilter{plate: sqlutil.PlateCompact(plate), fragment: sqlutil.PlateCompact(q), clientID: clientID}
	if f.plate == "" && f.fragment == "" && f.clientID == "" {
		return nil, errorspkg.Validation(
			errorspkg.Field{Name: "plate_number", Message: msgRequired},
			errorspkg.Field{Name: "q", Message: msgRequired},
			errorspkg.Field{Name: "client_id", Message: msgRequired},
		)
	}
	limit, offset := in.Limit, max(in.Offset, 0)
	if limit <= 0 {
		limit = lookupDefaultLimit
	}
	limit = min(limit, lookupMaxLimit)
	items, total, err := s.repo.LookupEquipment(ctx, f, limit, offset)
	if err != nil {
		return nil, err
	}
	return &EquipmentLookupResponse{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}
