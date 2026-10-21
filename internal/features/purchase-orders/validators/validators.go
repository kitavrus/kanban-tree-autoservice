// Package validators — проверка входа API закупок (§6.2, §6.4, §6.5, §6.9, §6.14, DS-047).
// Чистые функции без БД: формат, диапазоны, длины, сведение дублей запчастей. Проверки,
// требующие БД (запчасть/поставщик активны, id строки из этого заказа), — в service.
// Ошибки — коды полей details.fields[].message (как во всём проекте); текст пользователю —
// Message.
package validators

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей.
const (
	MsgRequired    = "required"
	MsgTooLong     = "too_long"
	MsgTooMany     = "too_many"
	MsgInvalid     = "invalid"
	MsgInvalidText = "invalid_text"
	MsgUnavailable = "unavailable"
)

var (
	qtyScale   = numeric.Scale{IntDigits: constants.QtyIntDigits, FracDigits: constants.QtyFracDigits}
	priceScale = numeric.Scale{IntDigits: constants.PriceIntDigits, FracDigits: constants.PriceFracDigits}
)

// ItemField — имя поля строки items[i].<name> в details.fields[].
func ItemField(i int, name string) string { return fmt.Sprintf("items[%d].%s", i, name) }

// Line — сведённая строка + индекс её первого вхождения в запросе (для ошибок полей).
type Line struct {
	models.ItemInput
	Index int
}

// Create — результат ValidateCreate.
type Create struct {
	SupplierID *string
	Note       string
	Lines      []Line
}

// ValidateCreate — §6.2.
func ValidateCreate(req dto.CreateRequest) (Create, []errorspkg.Field) {
	var out Create
	var f []errorspkg.Field
	if req.SupplierID != nil {
		id := strings.TrimSpace(*req.SupplierID)
		if uuid.Validate(id) != nil {
			f = append(f, errorspkg.Field{Name: "supplier_id", Message: MsgUnavailable})
		}
		out.SupplierID = &id
	}
	note, nf := validateNote(req.Note)
	f = append(f, nf...)
	out.Note = note
	lines, lf := validateItems(req.Items, false)
	f = append(f, lf...)
	out.Lines = lines
	return out, f
}

// Update — результат ValidateUpdate.
type Update struct {
	Version int64
	Note    string
	Lines   []Line
}

// ValidateUpdate — §6.4: items[].id — существующая строка (формат uuid; принадлежность заказу
// проверяет сервис).
func ValidateUpdate(req dto.UpdateRequest) (Update, []errorspkg.Field) {
	out := Update{Version: req.Version}
	var f []errorspkg.Field
	f = append(f, validateVersion(req.Version)...)
	note, nf := validateNote(req.Note)
	f = append(f, nf...)
	out.Note = note
	lines, lf := validateItems(req.Items, true)
	f = append(f, lf...)
	out.Lines = lines
	return out, f
}

// ValidateVersion — send/recall/pick-up (§6.6–§6.8).
func ValidateVersion(v int64) []errorspkg.Field { return validateVersion(v) }

func validateVersion(v int64) []errorspkg.Field {
	if v < 1 {
		return []errorspkg.Field{{Name: "version", Message: MsgRequired}}
	}
	return nil
}

// ValidateReason — причина отмены (§6.9) и удаления (§6.5): 1–500 символов после trim
// (общая проверка раздела «Удалённые», DS-072).
func ValidateReason(v int64, raw *string) (string, []errorspkg.Field) {
	f := validateVersion(v)
	reason, rf := softdelete.ValidateReason(raw)
	return reason, append(f, rf...)
}

func validateNote(note string) (string, []errorspkg.Field) {
	switch {
	case utf8.RuneCountInString(note) > constants.NoteMax:
		return note, []errorspkg.Field{{Name: "note", Message: MsgTooLong}}
	case sqlutil.HasInvalidText(note):
		return note, []errorspkg.Field{{Name: "note", Message: MsgInvalidText}}
	}
	return note, nil
}

// validateItems — 1–200 строк; qty > 0 и ≤ 99999.999 (3 знака); reason из enum (пусто —
// manual); повтор product_id — количества складываются в строку первого вхождения (DS-047).
func validateItems(items []dto.ItemRequest, allowIDs bool) ([]Line, []errorspkg.Field) {
	switch {
	case len(items) == 0:
		return nil, []errorspkg.Field{{Name: "items", Message: MsgRequired}}
	case len(items) > constants.MaxItems:
		return nil, []errorspkg.Field{{Name: "items", Message: MsgTooMany}}
	}
	var f []errorspkg.Field
	var lines []Line
	byProduct := map[string]int{} // product_id → индекс в lines
	sums := map[string]*big.Rat{}
	for i, it := range items {
		bad := false
		id := ""
		if it.ID != nil {
			id = strings.TrimSpace(*it.ID)
			if !allowIDs || uuid.Validate(id) != nil {
				f = append(f, errorspkg.Field{Name: ItemField(i, "id"), Message: MsgInvalid})
				bad = true
			}
		}
		pid := strings.TrimSpace(it.ProductID)
		if pid == "" {
			f = append(f, errorspkg.Field{Name: ItemField(i, "product_id"), Message: MsgRequired})
			bad = true
		} else if uuid.Validate(pid) != nil {
			f = append(f, errorspkg.Field{Name: ItemField(i, "product_id"), Message: MsgUnavailable})
			bad = true
		}
		qty, ok := ParseQty(string(it.Qty))
		if !ok {
			f = append(f, errorspkg.Field{Name: ItemField(i, "qty"), Message: MsgInvalid})
			bad = true
		}
		reason := strings.TrimSpace(it.Reason)
		switch {
		case reason == "" && id == "":
			// Новая строка без источника — ручная (§4.2); у существующей пустой reason —
			// «не менять» (repository сохраняет прежний).
			reason = constants.ReasonManual
		case reason != "" && !constants.In(reason, constants.Reasons):
			f = append(f, errorspkg.Field{Name: ItemField(i, "reason"), Message: MsgInvalid})
			bad = true
		}
		if bad {
			continue
		}
		pid = strings.ToLower(pid)
		if j, dup := byProduct[pid]; dup {
			sums[pid].Add(sums[pid], qty)
			if lines[j].ID == "" {
				lines[j].ID = strings.ToLower(id)
			}
			continue
		}
		byProduct[pid] = len(lines)
		sums[pid] = qty
		lines = append(lines, Line{ItemInput: models.ItemInput{ID: strings.ToLower(id), ProductID: pid, Reason: reason}, Index: i})
	}
	limit := new(big.Rat).SetFrac64(99999999, 1000)
	for j := range lines {
		s := sums[lines[j].ProductID]
		if s.Cmp(limit) > 0 {
			f = append(f, errorspkg.Field{Name: ItemField(lines[j].Index, "qty"), Message: MsgInvalid})
			continue
		}
		lines[j].Qty = s.FloatString(constants.QtyFracDigits)
	}
	return lines, f
}

// ParseQty — «Просим»: > 0 и ≤ 99999.999, не больше 3 знаков после точки.
func ParseQty(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || numeric.Amount(s).Validate(qtyScale, false) != "" {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 {
		return nil, false
	}
	return r, true
}

// ValidateAnswer — §6.14: supplier_qty 0–99999.999 (3 знака), supplier_price 0–9999999.99
// (2 знака), supplier_note ≤ 500.
func ValidateAnswer(req dto.AnswerRequest) (models.SupplierAnswer, []errorspkg.Field) {
	out := models.SupplierAnswer{Version: req.Version, SupplierNote: req.SupplierNote}
	f := validateVersion(req.Version)
	if req.SupplierQty != nil {
		s := strings.TrimSpace(string(*req.SupplierQty))
		if s == "" {
			out.SupplierQty = nil
		} else if numeric.Amount(s).Validate(qtyScale, false) != "" {
			f = append(f, errorspkg.Field{Name: "supplier_qty", Message: MsgInvalid})
		} else {
			out.SupplierQty = &s
		}
	}
	if req.SupplierPrice != nil {
		s := strings.TrimSpace(string(*req.SupplierPrice))
		if s == "" {
			out.SupplierPrice = nil
		} else if numeric.Amount(s).Validate(priceScale, false) != "" {
			f = append(f, errorspkg.Field{Name: "supplier_price", Message: MsgInvalid})
		} else {
			out.SupplierPrice = &s
		}
	}
	switch {
	case utf8.RuneCountInString(req.SupplierNote) > constants.SupplierNoteMax:
		f = append(f, errorspkg.Field{Name: "supplier_note", Message: MsgTooLong})
	case sqlutil.HasInvalidText(req.SupplierNote):
		f = append(f, errorspkg.Field{Name: "supplier_note", Message: MsgInvalidText})
	}
	return out, f
}

// Page — проверенные параметры списка.
type Page struct {
	Filter string // статус/тип; "" — без фильтра
	Limit  int
	Offset int
}

// ValidatePage — фильтр filterName из allowed (пусто — все), limit 1–maxLimit (не передан —
// defLimit), offset ≥ 0 (§6.1, §6.10, §6.12).
func ValidatePage(filterName, filter string, allowed []string, rawLimit, rawOffset string, defLimit, maxLimit int) (Page, []errorspkg.Field) {
	p := Page{Filter: strings.TrimSpace(filter), Limit: defLimit}
	var f []errorspkg.Field
	if p.Filter != "" && !constants.In(p.Filter, allowed) {
		f = append(f, errorspkg.Field{Name: filterName, Message: MsgInvalid})
	}
	if rawLimit != "" {
		n, err := strconv.Atoi(rawLimit)
		if err != nil || n < 1 || n > maxLimit {
			f = append(f, errorspkg.Field{Name: "limit", Message: MsgInvalid})
		} else {
			p.Limit = n
		}
	}
	if rawOffset != "" {
		n, err := strconv.Atoi(rawOffset)
		if err != nil || n < 0 {
			f = append(f, errorspkg.Field{Name: "offset", Message: MsgInvalid})
		} else {
			p.Offset = n
		}
	}
	return p, f
}

// Error — 400 VALIDATION с текстом первой ошибки поля (§6, «Сообщение пользователю»).
func Error(fields []errorspkg.Field) *errorspkg.AppError {
	e := errorspkg.Validation(fields...)
	if len(fields) > 0 {
		e.Message = Message(fields[0].Name, fields[0].Message)
	}
	return e
}

// Message — текст ошибки поля для пользователя (тексты §6 спеки).
func Message(field, code string) string {
	name := field
	if i := strings.LastIndex(field, "."); i >= 0 && strings.HasPrefix(field, "items[") {
		name = "items." + field[i+1:]
	}
	switch name {
	case "items":
		if code == MsgTooMany {
			return "Не больше 200 строк в заказе"
		}
		return "Добавьте хотя бы одну строку"
	case "items.qty":
		return "Количество от 0.001 до 99 999.999"
	case "items.product_id":
		return "Запчасть недоступна"
	case "items.reason":
		return "Неизвестный источник строки"
	case "items.id":
		return "Строка не из этого заказа"
	case "supplier_id":
		return "Поставщик недоступен"
	case "note":
		if code == MsgTooLong {
			return "Примечание — не больше 1000 символов"
		}
		return "Недопустимые символы в примечании"
	case "reason":
		return "Причина — от 1 до 500 символов"
	case "version":
		return "Не указана версия заказа"
	case "status":
		return "Неизвестный статус"
	case "limit", "offset":
		return "Некорректная пагинация"
	case "supplier_qty":
		return "Количество от 0 до 99 999.999"
	case "supplier_price":
		return "Цена от 0 до 9 999 999.99"
	case "supplier_note":
		if code == MsgTooLong {
			return "Комментарий — не больше 500 символов"
		}
		return "Недопустимые символы в комментарии"
	}
	return "Недопустимое значение"
}
