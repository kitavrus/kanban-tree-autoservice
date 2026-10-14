package transfers

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Коды ошибок полей (details.fields[].message) — клиент переводит их в текст.
const (
	msgRequired    = "required"
	msgInvalid     = "invalid"
	msgTooLong     = "too_long"
	msgInvalidText = "invalid_text"
	msgTooMany     = "too_many"
	// msgSameWarehouse — склад-получатель совпадает с источником (from = to → 400).
	msgSameWarehouse = "same_warehouse"
	// msgDuplicate — товар/единица уже есть в другой строке документа (D52: дубли не сливаются).
	msgDuplicate = "duplicate"
	// msgExceedsSent — факт приёмки больше отправленного (0 ≤ qty_received ≤ qty_sent; не
	// действует для прихода от поставщика, DS-015).
	msgExceedsSent = "exceeds_sent"
	// msgNotFound / msgArchived — склад, товар, единица не существует / в архиве.
	msgNotFound = "not_found"
	msgArchived = "archived"
	// msgNotAllowed — поле не относится к типу строки (qty_sent у единицы, equipment_id у
	// товара; факт приёмки по строке-единице) либо к направлению документа (точка маршрута,
	// тип строки, флаг receive, direction/task_id при правке).
	// msgNotInTask — единица не входит в оборудование заказа документа (DS-035).
	msgNotAllowed = "not_allowed"
	msgNotInTask  = "not_in_task"
	// msgBeforeFrom — «период по» раньше «периода с» (фильтр списка, DS-034).
	msgBeforeFrom = "before_from"
	// msgNotPositive — количество к отправке ≤ 0.
	msgNotPositive = "not_positive"
	// msgNegative — факт приёмки < 0; цена продажи < 0.
	msgNegative = "negative"
)

const (
	noteMax  = 1000
	itemsMax = 200
	// reasonMax — причина отмены проведения, символов после trim (transfers-revert DS-053).
	reasonMax = 500
)

// validateReason — причина отмены проведения: 1–500 символов после trim; возвращает
// нормализованное значение.
func validateReason(raw string) (string, []errorspkg.Field) {
	reason := strings.TrimSpace(raw)
	switch {
	case reason == "":
		return reason, []errorspkg.Field{{Name: "reason", Message: msgRequired}}
	case utf8.RuneCountInString(reason) > reasonMax:
		return reason, []errorspkg.Field{{Name: "reason", Message: msgTooLong}}
	case sqlutil.HasInvalidText(reason):
		return reason, []errorspkg.Field{{Name: "reason", Message: msgInvalidText}}
	}
	return reason, nil
}

// parseVersion — поле version тела удаления документа (сырой JSON): целое ≥ 1; нет / null →
// required, не целое число (строка, дробь, ноль, отрицательное) → invalid.
func parseVersion(raw json.RawMessage) (int, []errorspkg.Field) {
	v := strings.TrimSpace(string(raw))
	if v == "" || v == "null" {
		return 0, []errorspkg.Field{{Name: "version", Message: msgRequired}}
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, []errorspkg.Field{{Name: "version", Message: msgInvalid}}
	}
	return n, nil
}

// qtyScale — NUMERIC(12,3): 9 цифр целой части, 3 дробной.
var qtyScale = numeric.Scale{IntDigits: 9, FracDigits: 3}

// priceScale — NUMERIC(12,2): цена продажи 0–9999999999.99 (transfers-sales §6.1).
var priceScale = numeric.Scale{IntDigits: 10, FracDigits: 2}

// Input — нормализованные поля перемещения (создание/правка заявки). Direction и TaskID при
// правке берутся из документа (после создания не меняются, DS-002/DS-012).
type Input struct {
	Direction       string
	FromWarehouseID string
	ToWarehouseID   string
	TaskID          *int64
	SupplierID      string
	Note            string
	Items           []ItemInput
}

func normID(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (in Input) normalize() Input {
	in.Direction = strings.TrimSpace(in.Direction)
	in.FromWarehouseID = normID(in.FromWarehouseID)
	in.ToWarehouseID = normID(in.ToWarehouseID)
	in.SupplierID = normID(in.SupplierID)
	in.Note = strings.TrimSpace(in.Note)
	items := make([]ItemInput, len(in.Items))
	for i, it := range in.Items {
		items[i] = ItemInput{
			Kind:        strings.TrimSpace(it.Kind),
			ProductID:   normID(it.ProductID),
			EquipmentID: normID(it.EquipmentID),
			QtySent:     it.QtySent,
			Price:       it.Price,
		}
	}
	in.Items = items
	return in
}

// route — какие точки маршрута несёт направление (§4.1, CHECK transfers_route_check).
type route struct{ from, to, task, supplier bool }

var routes = map[string]route{
	DirClientToWarehouse:    {to: true, task: true},
	DirWarehouseToClient:    {from: true, task: true},
	DirWarehouseToWarehouse: {from: true, to: true},
	DirSupplierToWarehouse:  {to: true, supplier: true},
}

// kindAllowed — матрица «направление × тип строки» (DS-004): товар — склад → склад и приход
// от поставщика и накладная «склад → клиент» (товар с ценой, transfers-sales DS-011); единица —
// документы с клиентом и склад → склад.
func kindAllowed(direction, kind string) bool {
	switch kind {
	case KindProduct:
		return direction != DirClientToWarehouse
	case KindEquipment:
		return direction != DirSupplierToWarehouse
	}
	return false
}

func isDirection(v string) bool {
	_, ok := routes[v]
	return ok
}

// validate — ошибки полей после normalize() (существование/активность складов, товаров,
// единиц, заказа и поставщика проверяет repo). Набор точек маршрута — по направлению:
// недостающая → required, лишняя → not_allowed (§6.2).
func (in Input) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	rt, dirOK := routes[in.Direction]
	switch {
	case in.Direction == "":
		add("direction", msgRequired)
	case !dirOK:
		add("direction", msgInvalid)
	}
	if dirOK {
		fromOK := checkRoutePoint("from_warehouse_id", in.FromWarehouseID, rt.from, add)
		toOK := checkRoutePoint("to_warehouse_id", in.ToWarehouseID, rt.to, add)
		if rt.from && rt.to && fromOK && toOK && in.FromWarehouseID == in.ToWarehouseID {
			add("to_warehouse_id", msgSameWarehouse)
		}
		switch {
		case rt.task && in.TaskID == nil:
			add("task_id", msgRequired)
		case rt.task && *in.TaskID <= 0:
			add("task_id", msgInvalid)
		case !rt.task && in.TaskID != nil:
			add("task_id", msgNotAllowed)
		}
		checkRoutePoint("supplier_id", in.SupplierID, rt.supplier, add)
	}

	if utf8.RuneCountInString(in.Note) > noteMax {
		add("note", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Note) {
		add("note", msgInvalidText)
	}

	switch {
	case len(in.Items) == 0:
		add("items", msgRequired)
	case len(in.Items) > itemsMax:
		add("items", msgTooMany)
	}
	seenProduct, seenEquipment := map[string]bool{}, map[string]bool{}
	for i, it := range in.Items {
		kf := fmt.Sprintf("items[%d].kind", i)
		pf := fmt.Sprintf("items[%d].product_id", i)
		ef := fmt.Sprintf("items[%d].equipment_id", i)
		qf := fmt.Sprintf("items[%d].qty_sent", i)
		prf := fmt.Sprintf("items[%d].price", i)
		if dirOK && (it.Kind == KindProduct || it.Kind == KindEquipment) && !kindAllowed(in.Direction, it.Kind) {
			add(kf, msgNotAllowed)
			continue
		}
		switch it.Kind {
		case KindProduct:
			if checkUUID(pf, it.ProductID, add) {
				if seenProduct[it.ProductID] {
					add(pf, msgDuplicate)
				}
				seenProduct[it.ProductID] = true
			}
			if it.EquipmentID != "" {
				add(ef, msgNotAllowed)
			}
			if msg := positiveQty(it.QtySent); msg != "" {
				add(qf, msg)
			}
			if it.Price != nil {
				if !pricedDirection(in.Direction) {
					add(prf, msgNotAllowed)
				} else if msg := salePrice(*it.Price); msg != "" {
					add(prf, msg)
				}
			}
		case KindEquipment:
			if checkUUID(ef, it.EquipmentID, add) {
				if seenEquipment[it.EquipmentID] {
					add(ef, msgDuplicate)
				}
				seenEquipment[it.EquipmentID] = true
			}
			if it.ProductID != "" {
				add(pf, msgNotAllowed)
			}
			if it.QtySent != "" {
				add(qf, msgNotAllowed)
			}
			if it.Price != nil {
				add(prf, msgNotAllowed)
			}
		case "":
			add(kf, msgRequired)
		default:
			add(kf, msgInvalid)
		}
	}
	return f
}

// checkRoutePoint — uuid точки маршрута: нужна направлению → обязательна и корректна; не
// нужна, но передана → not_allowed. true — значение есть и корректно.
func checkRoutePoint(name, v string, needed bool, add func(name, msg string)) bool {
	if !needed {
		if v != "" {
			add(name, msgNotAllowed)
		}
		return false
	}
	return checkUUID(name, v, add)
}

// immutableGuards — direction и task_id в теле правки (§6.4): после создания не меняются,
// переданы → not_allowed.
func immutableGuards(direction *string, taskID *int64) []errorspkg.Field {
	var f []errorspkg.Field
	if direction != nil {
		f = append(f, errorspkg.Field{Name: "direction", Message: msgNotAllowed})
	}
	if taskID != nil {
		f = append(f, errorspkg.Field{Name: "task_id", Message: msgNotAllowed})
	}
	return f
}

// createFlags — флаг receive («создать и принять», DS-013) допустим только для приёма от
// клиента.
func createFlags(direction string, receive bool) []errorspkg.Field {
	if receive && isDirection(direction) && direction != DirClientToWarehouse {
		return []errorspkg.Field{{Name: "receive", Message: msgNotAllowed}}
	}
	return nil
}

func checkUUID(name, v string, add func(name, msg string)) bool {
	if v == "" {
		add(name, msgRequired)
		return false
	}
	// Только каноническая 36-символьная форма: urn:/{}-варианты uuid.Validate пропускает,
	// а приведение ::uuid в PG — не все (22P02 → 500).
	if len(v) != 36 || uuid.Validate(v) != nil {
		add(name, msgInvalid)
		return false
	}
	return true
}

// positiveQty — qty_sent: число NUMERIC(12,3), строго > 0.
func positiveQty(a numeric.Amount) string {
	if a == "" {
		return msgRequired
	}
	if msg := a.Validate(qtyScale, true); msg != "" {
		return msg
	}
	if ratOf(string(a)).Sign() <= 0 {
		return msgNotPositive
	}
	return ""
}

// pricedDirection — направление, товарные строки которого несут цену продажи: только
// накладная «склад → клиент» (transfers-sales DS-011).
func pricedDirection(direction string) bool { return direction == DirWarehouseToClient }

// salePrice — цена продажи NUMERIC(12,2), 0–9999999999.99: отрицательная → negative; не
// число, больше двух знаков после запятой, больше максимума → invalid (§6.1).
func salePrice(a numeric.Amount) string {
	switch a.Validate(priceScale, false) {
	case "":
		return ""
	case numeric.MsgInvalid:
		return msgNegative
	}
	return msgInvalid
}

func ratOf(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return new(big.Rat)
	}
	return r
}

// sentItem — строка документа для проверки приёмки (QtySent пуст у строки-единицы).
type sentItem struct {
	ID      string
	Kind    string
	QtySent string
}

// resolveReceive — факт по каждой ТОВАРНОЙ строке документа (DS-037): не указанная строка —
// факт = qty_sent; указанная — 0 ≤ qty_received ≤ qty_sent; строка-единица в запросе →
// not_allowed (единицы принимаются целиком). Возвращает id товарной строки → факт.
// direction: приход от поставщика — факт может превышать заказанное (DS-015, предел —
// NUMERIC(12,3)); документы с клиентом факта не имеют — любой непустой items → not_allowed.
func resolveReceive(direction string, items []sentItem, in []ReceiveItemInput) (map[string]string, []errorspkg.Field) {
	var f []errorspkg.Field
	if isClientDirection(direction) {
		// товар накладной «склад → клиент» списан при отправке — факт не вводится, qty_received
		// остаётся NULL и после приёмки (transfers-sales DS-038).
		if len(in) > 0 {
			return nil, []errorspkg.Field{{Name: "items", Message: msgNotAllowed}}
		}
		return map[string]string{}, nil
	}
	byID := make(map[string]sentItem, len(items))
	out := make(map[string]string, len(items))
	for _, it := range items {
		byID[it.ID] = it
		if it.Kind == KindProduct {
			out[it.ID] = it.QtySent
		}
	}
	seen := map[string]bool{}
	for i, r := range in {
		idf := fmt.Sprintf("items[%d].id", i)
		qf := fmt.Sprintf("items[%d].qty_received", i)
		id := normID(r.ID)
		it, ok := byID[id]
		switch {
		case id == "":
			f = append(f, errorspkg.Field{Name: idf, Message: msgRequired})
			continue
		case !ok:
			f = append(f, errorspkg.Field{Name: idf, Message: msgNotFound})
			continue
		case it.Kind != KindProduct:
			f = append(f, errorspkg.Field{Name: idf, Message: msgNotAllowed})
			continue
		case seen[id]:
			f = append(f, errorspkg.Field{Name: idf, Message: msgDuplicate})
			continue
		}
		seen[id] = true
		if r.QtyReceived == "" {
			f = append(f, errorspkg.Field{Name: qf, Message: msgRequired})
			continue
		}
		if msg := r.QtyReceived.Validate(qtyScale, true); msg != "" {
			f = append(f, errorspkg.Field{Name: qf, Message: msg})
			continue
		}
		fact := ratOf(string(r.QtyReceived))
		if fact.Sign() < 0 {
			f = append(f, errorspkg.Field{Name: qf, Message: msgNegative})
			continue
		}
		if direction != DirSupplierToWarehouse && fact.Cmp(ratOf(it.QtySent)) > 0 {
			f = append(f, errorspkg.Field{Name: qf, Message: msgExceedsSent})
			continue
		}
		out[id] = string(r.QtyReceived)
	}
	return out, f
}
