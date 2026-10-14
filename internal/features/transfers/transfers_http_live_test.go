//go:build integration

package transfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// TransfersLiveSuite (§TD-8): документ «Перемещение» склад → склад со смешанными строками
// (docs/features/transfers-core/draft-spec.md) — товары (as-is: заявка → правка → отправка →
// приёмка с недостачей, отмены, «В пути» на остатках) и единицы оборудования клиентов:
// TS-005 (занятие единицы документом), TS-006 (отправка только со склада отправления),
// TS-009 (version до статуса, свои сообщения переходов, двойной receive), TS-028 (мусорное
// тело — без 5xx), SSR. Направления с клиентом и поставщиком, права исполнителя, фильтры
// списка — transfers_partners_http_live_test.go (transfers-partners). Тела создания этого
// файла без direction получают «склад → склад» (хелпер do).
type TransfersLiveSuite struct {
	testutil.LiveSuite
	admin   *testutil.Browser
	adminID string
	// exec — исполнитель (transfers-partners), создаётся лениво хелпером executor().
	exec   *testutil.Browser
	almaty string
	moscow string
	catID  string
	// clientID/typeID — владелец и тип единиц оборудования фикстуры.
	clientID string
	typeID   string
	seq      int
}

func TestTransfersLiveSuite(t *testing.T) {
	// TEST_PURGE_ENABLED=true — служебная уборка документов (soft-delete-transfers TS-051); сьюта
	// без неё (адреса нет, 404) — BoardLiveSuite.
	suite.Run(t, &TransfersLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"TEST_PURGE_ENABLED=true"}}})
}

func (s *TransfersLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("st-admin", "password1", "Перемещения Админ", "admin", true)
	s.admin = s.NewBrowser()
	s.admin.Login("st-admin", "password1")
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM users WHERE login = 'st-admin'`).Scan(&s.adminID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM warehouses WHERE name = 'Алматы'`).Scan(&s.almaty))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM warehouses WHERE name = 'Москва'`).Scan(&s.moscow))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'ST товары') RETURNING id::text`).Scan(&s.catID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO clients (company_name) VALUES ('ТОО Перемещаемый') RETURNING id::text`).Scan(&s.clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO equipment_types (name) VALUES ('Лазерный станок ST') RETURNING id::text`).Scan(&s.typeID))
}

// ---- wire-типы ----

// itemDTO — строка документа: у товара заполнены product_*/qty_*, у единицы — equipment_*
// (остальное в JSON — null, здесь — нулевое значение).
type itemDTO struct {
	ID                  string       `json:"id"`
	Kind                string       `json:"kind"`
	ProductID           string       `json:"product_id"`
	ProductName         string       `json:"product_name"`
	EquipmentID         string       `json:"equipment_id"`
	EquipmentName       string       `json:"equipment_name"`
	EquipmentSerial     string       `json:"equipment_serial"`
	EquipmentClientName string       `json:"equipment_client_name"`
	QtySent             *json.Number `json:"qty_sent"`
	QtyReceived         *json.Number `json:"qty_received"`
	// Price / LineTotal — только товарная строка накладной «склад → клиент» (transfers-sales).
	Price     *json.Number `json:"price"`
	LineTotal *json.Number `json:"line_total"`
}

type warningDTO struct {
	ProductID   string      `json:"product_id"`
	WarehouseID string      `json:"warehouse_id"`
	Stock       json.Number `json:"stock"`
	Kind        string      `json:"kind"`
}

// transferDTO — точки маршрута, не положенные направлению, в JSON — null (здесь — нулевое
// значение; TaskID — указатель: «нет заказа» отличим от 0).
type transferDTO struct {
	ID                int64   `json:"id"`
	Direction         string  `json:"direction"`
	FromWarehouseID   string  `json:"from_warehouse_id"`
	FromWarehouseName string  `json:"from_warehouse_name"`
	ToWarehouseID     string  `json:"to_warehouse_id"`
	ToWarehouseName   string  `json:"to_warehouse_name"`
	TaskID            *int64  `json:"task_id"`
	TaskTitle         string  `json:"task_title"`
	TaskIsDeleted     bool    `json:"task_is_deleted"`
	ClientID          string  `json:"client_id"`
	ClientName        string  `json:"client_name"`
	SupplierID        string  `json:"supplier_id"`
	SupplierName      string  `json:"supplier_name"`
	Status            string  `json:"status"`
	Note              string  `json:"note"`
	CreatedByName     string  `json:"created_by_name"`
	SentAt            *string `json:"sent_at"`
	ReceivedAt        *string `json:"received_at"`
	CancelledAt       *string `json:"cancelled_at"`
	// сведения о последней отмене проведения (transfers-revert DS-044).
	ReceivedBy       *string      `json:"received_by"`
	UnreceivedBy     *string      `json:"unreceived_by"`
	UnreceivedByName *string      `json:"unreceived_by_name"`
	UnreceivedAt     *string      `json:"unreceived_at"`
	UnreceiveReason  string       `json:"unreceive_reason"`
	Version          int          `json:"version"`
	Amount           json.Number  `json:"amount"`
	Items            []itemDTO    `json:"items"`
	Warnings         []warningDTO `json:"warnings"`
}

type listDTO struct {
	Items []struct {
		ID           int64       `json:"id"`
		Direction    string      `json:"direction"`
		Status       string      `json:"status"`
		ItemsCount   int         `json:"items_count"`
		TaskID       *int64      `json:"task_id"`
		TaskDeleted  bool        `json:"task_is_deleted"`
		ClientName   string      `json:"client_name"`
		SupplierID   string      `json:"supplier_id"`
		SupplierName string      `json:"supplier_name"`
		Amount       json.Number `json:"amount"`
	} `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		Fields      []struct{ Name, Message string } `json:"fields"`
		TransferID  int64                            `json:"transfer_id"`
		EquipmentID string                           `json:"equipment_id"`
	} `json:"details"`
}

// ---- хелперы ----

func (s *TransfersLiveSuite) errOf(raw []byte) apiErr {
	var e apiErr
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e
}

func (s *TransfersLiveSuite) fields(raw []byte) map[string]string {
	e := s.errOf(raw)
	s.Require().NotNil(e.Details, string(raw))
	m := map[string]string{}
	for _, f := range e.Details.Fields {
		m[f.Name] = f.Message
	}
	return m
}

// newProduct — товар с остатком stock на «Алматы» (SQL: фикстура, не предмет теста).
func (s *TransfersLiveSuite) newProduct(stock string) string {
	s.seq++
	ctx := context.Background()
	var id string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (category_id, name) VALUES ($1, $2) RETURNING id::text`,
		s.catID, fmt.Sprintf("ST товар %d", s.seq)).Scan(&id))
	_, err := s.Pool.Exec(ctx, `INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, $3::numeric)`,
		id, s.almaty, stock)
	s.Require().NoError(err)
	return id
}

func (s *TransfersLiveSuite) pairStock(productID, warehouseID string) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT coalesce((SELECT stock FROM product_stock WHERE product_id = $1 AND warehouse_id = $2), 0)::text`,
		productID, warehouseID).Scan(&v))
	return v
}

// movements — reason:склад:delta по перемещению (id движений — uuid, порядок не задан).
func (s *TransfersLiveSuite) movements(transferID int64) []string {
	rows, err := s.Pool.Query(context.Background(), `
		SELECT m.reason || ':' || w.name || ':' || m.qty_delta::text FROM stock_movements m
		JOIN warehouses w ON w.id = m.warehouse_id WHERE m.transfer_id = $1`, transferID)
	s.Require().NoError(err)
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		s.Require().NoError(rows.Scan(&v))
		out = append(out, v)
	}
	s.Require().NoError(rows.Err())
	return out
}

// do — запрос администратора. Тело создания (POST /api/v1/transfers) без direction —
// документ «склад → склад»: сценарии ядра направление не указывают.
func (s *TransfersLiveSuite) do(method, path string, body any) (int, transferDTO, []byte) {
	if m, ok := body.(map[string]any); ok && method == http.MethodPost && path == "/api/v1/transfers" {
		if _, has := m["direction"]; !has {
			withDirection := map[string]any{"direction": "warehouse_to_warehouse"}
			for k, v := range m {
				withDirection[k] = v
			}
			body = withDirection
		}
	}
	st, raw := s.admin.Do(method, path, body)
	var t transferDTO
	if st < 300 {
		s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	}
	return st, t, raw
}

func (s *TransfersLiveSuite) create(body map[string]any) transferDTO {
	st, t, raw := s.do(http.MethodPost, "/api/v1/transfers", body)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	return t
}

func (s *TransfersLiveSuite) action(t transferDTO, act string, body map[string]any) (int, transferDTO, []byte) {
	if body == nil {
		body = map[string]any{}
	}
	if _, ok := body["version"]; !ok {
		body["version"] = t.Version
	}
	return s.do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/%s", t.ID, act), body)
}

func (s *TransfersLiveSuite) inTransit(productID, warehouseID string) string {
	q := "?scope=all&limit=1000"
	if warehouseID != "" {
		q += "&warehouse_id=" + warehouseID
	}
	st, raw := s.admin.Do(http.MethodGet, "/api/v1/stock"+q, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l struct {
		Items []struct {
			ProductID string      `json:"product_id"`
			InTransit json.Number `json:"in_transit"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &l), string(raw))
	for _, it := range l.Items {
		if it.ProductID == productID {
			return it.InTransit.String()
		}
	}
	s.FailNow("товар не найден на экране остатков", productID)
	return ""
}

// ---- товарные строки (as-is) ----

// Заявка → правка строк → отправка (минус на источнике, «в пути») → приёмка по
// факту с недостачей (приход + transfer_loss) — итоговые остатки верны.
func (s *TransfersLiveSuite) TestProducts_RequestEditSendReceiveWithLoss() {
	p1, p2, p3 := s.newProduct("10"), s.newProduct("3"), s.newProduct("7")

	t := s.create(map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "note": " под заказ ",
		"items": []map[string]any{{"kind": "product", "product_id": p1, "qty_sent": 4}, {"kind": "product", "product_id": p3, "qty_sent": 1}},
	})
	s.Equal("requested", t.Status)
	s.Equal("под заказ", t.Note)
	s.Equal(1, t.Version)
	s.Equal("Перемещения Админ", t.CreatedByName)
	s.Len(t.Items, 2)
	s.Nil(t.SentAt)
	s.Equal("10.000", s.pairStock(p1, s.almaty), "заявка не двигает остатки")
	s.Empty(s.movements(t.ID))

	// правка строк заявки: p3 убрана, p2 добавлена, количество p1 изменено.
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "note": "",
		"items":   []map[string]any{{"kind": "product", "product_id": p1, "qty_sent": 6}, {"kind": "product", "product_id": p2, "qty_sent": 5}},
		"version": t.Version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, upd.Version)
	s.Require().Len(upd.Items, 2)
	s.Equal(p1, upd.Items[0].ProductID)
	s.Require().NotNil(upd.Items[0].QtySent)
	s.Equal("6.000", upd.Items[0].QtySent.String())
	s.Equal("product", upd.Items[0].Kind)
	s.Equal(p2, upd.Items[1].ProductID)

	// список: конверт §API-1, фильтр статуса.
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/transfers?status=requested", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var l listDTO
	s.Require().NoError(json.Unmarshal(raw, &l))
	s.Equal(50, l.Limit)
	s.Equal(0, l.Offset)
	found := false
	for _, it := range l.Items {
		s.Equal("requested", it.Status)
		if it.ID == t.ID {
			found = true
			s.Equal(2, it.ItemsCount)
		}
	}
	s.True(found)

	// отправка: −qty_sent с источника; p2 уходит в минус (3−5) — разрешено с предупреждением.
	st, sent, raw := s.action(upd, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("in_transit", sent.Status)
	s.NotNil(sent.SentAt)
	s.Equal("4.000", s.pairStock(p1, s.almaty))
	s.Equal("-2.000", s.pairStock(p2, s.almaty))
	s.Require().Len(sent.Warnings, 1)
	s.Equal(p2, sent.Warnings[0].ProductID)
	s.Equal("negative", sent.Warnings[0].Kind)
	s.Equal(s.almaty, sent.Warnings[0].WarehouseID)

	// «В пути» (TS-007): по всем складам и при фильтре склада-получателя; у источника — 0.
	s.Equal("6.000", s.inTransit(p1, ""))
	s.Equal("6.000", s.inTransit(p1, s.moscow))
	s.Equal("0", s.inTransit(p1, s.almaty))

	// приёмка по факту: p1 — 5 из 6 (недостача 1), p2 — без записи → факт = отправлено.
	st, rec, raw := s.action(sent, "receive", map[string]any{
		"items": []map[string]any{{"id": sent.Items[0].ID, "qty_received": 5}},
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("received", rec.Status)
	s.NotNil(rec.ReceivedAt)
	s.Require().NotNil(rec.Items[0].QtyReceived)
	s.Equal("5.000", rec.Items[0].QtyReceived.String())
	s.Equal("5.000", rec.Items[1].QtyReceived.String())
	s.Equal("5.000", s.pairStock(p1, s.moscow))
	s.Equal("5.000", s.pairStock(p2, s.moscow))
	s.Equal("4.000", s.pairStock(p1, s.almaty), "приёмка не трогает источник")
	s.Equal("0", s.inTransit(p1, ""), "после приёмки — не в пути")

	s.ElementsMatch([]string{
		"transfer_out:Алматы:-6.000", "transfer_out:Алматы:-5.000",
		"transfer_in:Москва:6.000", "transfer_in:Москва:5.000",
		"transfer_loss:Москва:-1.000",
	}, s.movements(t.ID))
}

// Создание сразу «Отправить» (send=true) — in_transit одной операцией.
func (s *TransfersLiveSuite) TestCreateAndSend() {
	p := s.newProduct("2")
	t := s.create(map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "send": true,
		"items": []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 2}},
	})
	s.Equal("in_transit", t.Status)
	s.Equal("0.000", s.pairStock(p, s.almaty))
	s.Require().Len(t.Warnings, 1, "остаток 0 при минимуме 0 — на минимуме")
	s.Equal("below_min", t.Warnings[0].Kind)
	s.ElementsMatch([]string{"transfer_out:Алматы:-2.000"}, s.movements(t.ID))
}

// Отмена in_transit → возврат на источник; отмена requested → без движений;
// правка received → 409; from = to → 400.
func (s *TransfersLiveSuite) TestProducts_CancelFinalSameWarehouse() {
	p := s.newProduct("10")
	items := []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 3}}

	// отмена requested — без движений.
	req := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items})
	st, c1, raw := s.action(req, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", c1.Status)
	s.NotNil(c1.CancelledAt)
	s.Empty(s.movements(req.ID))
	s.Equal("10.000", s.pairStock(p, s.almaty))

	// отмена in_transit — возврат на источник.
	tr := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "send": true})
	s.Equal("7.000", s.pairStock(p, s.almaty))
	st, c2, raw := s.action(tr, "cancel", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("cancelled", c2.Status)
	s.Equal("10.000", s.pairStock(p, s.almaty))
	s.Equal("0", s.pairStock(p, s.moscow), "получатель не затронут")
	s.ElementsMatch([]string{"transfer_out:Алматы:-3.000", "transfer_return:Алматы:3.000"}, s.movements(tr.ID))

	// финальные: правка/отправка/приёмка/отмена cancelled → 409.
	for _, act := range []string{"send", "receive", "cancel"} {
		st, _, raw := s.action(c2, act, nil)
		s.Equal(http.StatusConflict, st, "%s → %s", act, raw)
		s.Equal("TRANSFER_STATUS_CONFLICT", s.errOf(raw).Code)
	}

	// правка received → 409.
	rc := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "send": true})
	st, rcv, raw := s.action(rc, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", rc.ID), map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "version": rcv.Version,
	})
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TRANSFER_STATUS_CONFLICT", s.errOf(raw).Code)
	st, _, raw = s.action(rcv, "cancel", nil)
	s.Equal(http.StatusConflict, st, "received нельзя отменить: %s", raw)

	// правка строк in_transit тоже запрещена (строки редактируемы только в заявке).
	it := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "send": true})
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", it.ID), map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "version": it.Version,
	})
	s.Equal(http.StatusConflict, st, string(raw))

	// from = to → 400.
	st, _, raw = s.do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.almaty, "items": items,
	})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"to_warehouse_id": "same_warehouse"}, s.fields(raw))
}

// Валидация тела: строки, склады/товары (не существует, архив), приёмка вне границ, мусор.
func (s *TransfersLiveSuite) TestValidation() {
	p := s.newProduct("1")
	ok := []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 1}}
	cases := []struct {
		body any
		want map[string]string
	}{
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []any{}}, map[string]string{"items": "required"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 0}}},
			map[string]string{"items[0].qty_sent": "not_positive"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 1}, {"kind": "product", "product_id": p, "qty_sent": 2}}},
			map[string]string{"items[1].product_id": "duplicate"}},
		{map[string]any{"from_warehouse_id": "00000000-0000-0000-0000-000000000000", "to_warehouse_id": s.moscow, "items": ok},
			map[string]string{"from_warehouse_id": "not_found"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "product", "product_id": "00000000-0000-0000-0000-000000000000", "qty_sent": 1}}},
			map[string]string{"items[0].product_id": "not_found"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"product_id": p, "qty_sent": 1}}},
			map[string]string{"items[0].kind": "required"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "equipment", "equipment_id": "00000000-0000-0000-0000-000000000000"}}},
			map[string]string{"items[0].equipment_id": "not_found"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "equipment", "equipment_id": "00000000-0000-0000-0000-000000000000", "qty_sent": 1, "product_id": p}}},
			map[string]string{"items[0].qty_sent": "not_allowed", "items[0].product_id": "not_allowed"}},
		{map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 1, "equipment_id": "00000000-0000-0000-0000-000000000000"}}},
			map[string]string{"items[0].equipment_id": "not_allowed"}},
	}
	for _, c := range cases {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", c.body)
		s.Equal(http.StatusBadRequest, st, "%v → %s", c.body, raw)
		s.Equal(c.want, s.fields(raw), "%v", c.body)
	}
	for _, body := range []any{`{`, `null`, `{"items":"x"}`, map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": []map[string]any{{"kind": "product", "product_id": p, "qty_sent": "1"}}}} {
		st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", body)
		s.Equal(http.StatusBadRequest, st, "%v → %s", body, raw)
	}

	// архивный товар — 400; в существующей заявке — допустим при пересохранении.
	t := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": ok})
	_, err := s.Pool.Exec(context.Background(), `UPDATE products SET is_active = false WHERE id = $1`, p)
	s.Require().NoError(err)
	st, _, raw := s.do(http.MethodPost, "/api/v1/transfers", map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": ok})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].product_id": "archived"}, s.fields(raw))
	st, _, raw = s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": ok, "version": t.Version,
	})
	s.Equal(http.StatusOK, st, "архивный товар уже в заявке: %s", raw)

	// version обязателен; приёмка: факт > отправленного, чужая строка.
	st, _, raw = s.do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/send", t.ID), map[string]any{})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"version": "required"}, s.fields(raw))

	p2 := s.newProduct("5")
	tr := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "send": true,
		"items": []map[string]any{{"kind": "product", "product_id": p2, "qty_sent": 2}}})
	st, _, raw = s.action(tr, "receive", map[string]any{"items": []map[string]any{{"id": tr.Items[0].ID, "qty_received": 2.5}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].qty_received": "exceeds_sent"}, s.fields(raw))
	st, _, raw = s.action(tr, "receive", map[string]any{"items": []map[string]any{{"id": "00000000-0000-0000-0000-000000000000", "qty_received": 1}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].id": "not_found"}, s.fields(raw))
	st, _, raw = s.action(tr, "receive", map[string]any{"items": []map[string]any{{"id": tr.Items[0].ID, "qty_received": -1}}})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal(map[string]string{"items[0].qty_received": "negative"}, s.fields(raw))
	s.Equal("3.000", s.pairStock(p2, s.almaty), "неудачная приёмка ничего не меняет")
	s.Equal("0", s.pairStock(p2, s.moscow))

	// фильтры списка: мусор → 400; 404 на чужой/битый id.
	for _, q := range []string{"?status=done", "?warehouse_id=x"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/transfers"+q, nil)
		s.Equal(http.StatusBadRequest, st, "%s → %s", q, raw)
	}
	for _, id := range []string{"abc", "0", "999999999"} {
		st, raw := s.admin.Do(http.MethodGet, "/api/v1/transfers/"+id, nil)
		s.Equal(http.StatusNotFound, st, "%s → %s", id, raw)
	}
}

// Товарный документ: устаревший version → 409; двойной receive → 409; параллельные receive —
// один успех, остаток получателя изменён ровно один раз.
func (s *TransfersLiveSuite) TestProducts_VersionAndDoubleReceive() {
	p := s.newProduct("10")
	items := []map[string]any{{"kind": "product", "product_id": p, "qty_sent": 2}}
	t := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items})

	// устаревший version на правке и на отправке.
	st, upd, raw := s.do(http.MethodPut, fmt.Sprintf("/api/v1/transfers/%d", t.ID), map[string]any{
		"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "version": t.Version,
	})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.action(t, "send", nil) // t.Version = 1, актуальная — 2
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("TRANSFER_VERSION_CONFLICT", s.errOf(raw).Code)
	s.Equal("10.000", s.pairStock(p, s.almaty))

	st, sent, raw := s.action(upd, "send", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.action(sent, "receive", nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _, raw = s.action(sent, "receive", nil) // тот же version — повтор
	s.Equal(http.StatusConflict, st, string(raw))
	s.Equal("2.000", s.pairStock(p, s.moscow), "двойной receive не приходует дважды")

	// параллельные receive одного перемещения.
	t2 := s.create(map[string]any{"from_warehouse_id": s.almaty, "to_warehouse_id": s.moscow, "items": items, "send": true})
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/receive", t2.ID), map[string]any{"version": t2.Version})
		}(i)
	}
	wg.Wait()
	okCount := 0
	for _, c := range codes {
		if c == http.StatusOK {
			okCount++
		} else {
			s.Equal(http.StatusConflict, c)
		}
	}
	s.Equal(1, okCount, "ровно одна приёмка")
	s.Equal("4.000", s.pairStock(p, s.moscow))
}

// Без сессии — 401 на API (права исполнителя — TestTS015_*, transfers-partners).
func (s *TransfersLiveSuite) TestAnonymousUnauthorized() {
	anon := s.NewBrowser()
	for _, cs := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/transfers", nil},
		{http.MethodPost, "/api/v1/transfers", map[string]any{}},
		{http.MethodGet, "/api/v1/transfers/1", nil},
		{http.MethodPut, "/api/v1/transfers/1", map[string]any{}},
		{http.MethodPost, "/api/v1/transfers/1/send", map[string]any{"version": 1}},
		{http.MethodPost, "/api/v1/transfers/1/receive", map[string]any{"version": 1}},
		{http.MethodPost, "/api/v1/transfers/1/cancel", map[string]any{"version": 1}},
	} {
		st, raw := anon.Do(cs.method, cs.path, cs.body)
		s.Equal(http.StatusUnauthorized, st, "%s %s → %s", cs.method, cs.path, raw)
	}
}

// SSR: страница admin, пункт сайдбара, скрипт экрана.
func (s *TransfersLiveSuite) TestSSR_Page() {
	resp := s.admin.Request(http.MethodGet, "/transfers", nil, nil)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), `data-testid="st-root"`)
	s.Contains(string(body), `href="/transfers"`, "пункт «Перемещения» в сайдбаре admin")
	s.Contains(string(body), `/assets/ssr-shared/js/transfers.js`)
	s.NotContains(string(body), "stock-transfers", "старых адресов и скрипта нет (DS-030)")
	// §8.1: фильтр «Склад» — SSR-списком складов; кнопки «Новое перемещение» и «Сбросить».
	s.Contains(string(body), `value="`+s.almaty+`" data-active="true" data-name="Алматы">Алматы</option>`)
	s.Contains(string(body), `value="`+s.moscow+`" data-active="true" data-name="Москва">Москва</option>`)
	s.Contains(string(body), `data-testid="st-new" data-bs-toggle="dropdown" aria-expanded="false">Новое перемещение</button>`)
	s.Contains(string(body), `data-testid="st-filter-reset"`)
	s.Contains(string(body), `<option value="received">Получено</option>`)
	s.Contains(string(body), `<option value="cancelled">Отменён</option>`)
}
