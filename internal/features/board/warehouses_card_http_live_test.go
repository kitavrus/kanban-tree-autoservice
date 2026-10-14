//go:build integration

package board_test

import (
	"context"
	"fmt"
	"net/http"
)

// Склады в карточке заказа (docs/features/warehouses/draft-spec.md DS-005, TS-004…TS-006):
// списание/возврат по паре товар×склад, смена склада у списанной строки, склад по
// умолчанию. Методы OrderCardLiveSuite — общие фикстуры карточки (order_card_http_live_test.go).

func (s *OrderCardLiveSuite) warehouseID(name string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT id FROM warehouses WHERE name = $1`, name).Scan(&id))
	return id
}

func (s *OrderCardLiveSuite) pairStock(productID, warehouseID string) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT coalesce((SELECT stock FROM product_stock WHERE product_id = $1 AND warehouse_id = $2), 0)::text`,
		productID, warehouseID).Scan(&v))
	return v
}

func (s *OrderCardLiveSuite) setPairStock(productID, warehouseID, stock string) {
	_, err := s.Pool.Exec(context.Background(), `
		INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, $3::numeric)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET stock = EXCLUDED.stock`, productID, warehouseID, stock)
	s.Require().NoError(err)
}

func (s *OrderCardLiveSuite) movements(taskID int64) map[string]int {
	rows, err := s.Pool.Query(context.Background(), `
		SELECT w.name || ':' || m.reason, count(*) FROM stock_movements m JOIN warehouses w ON w.id = m.warehouse_id
		WHERE m.task_id = $1 GROUP BY 1`, taskID)
	s.Require().NoError(err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		s.Require().NoError(rows.Scan(&k, &n))
		out[k] = n
	}
	s.Require().NoError(rows.Err())
	return out
}

func (s *OrderCardLiveSuite) moveTask(taskID int64, columnName string) {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/move", taskID), map[string]any{"column_id": s.columnID(columnName)})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

// TS-004: заказ с товаром на складе «Москва» переходит в consumes_stock-колонку → минус на
// Москве, Алматы не меняется; обратный перенос — возврат на Москву.
func (s *OrderCardLiveSuite) TestWarehouses_TS004_ConsumeAndReturnByWarehouse() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	taskID, _ := s.newTask(s.newClient("Клиент-Склад-Списание"), s.columnID("Новые"))
	svcID := s.newService("100")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	productID := s.newProduct("10", "20", "0") // Алматы: 20
	s.setPairStock(productID, moscow, "5")     // Москва: 5

	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products",
		map[string]any{"product_id": productID, "qty": 2, "warehouse_id": moscow})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Products, 1)
	s.Equal(moscow, m.Card.Subtasks[0].Products[0].WarehouseID)
	s.Equal("Москва", m.Card.Subtasks[0].Products[0].WarehouseName)
	s.Equal("5.000", s.pairStock(productID, moscow), "в не-складской колонке списания нет")

	s.moveTask(taskID, "В работе") // consumes_stock=true (сид D32)
	s.Equal("3.000", s.pairStock(productID, moscow), "списано с Москвы")
	s.Equal("20.000", s.pairStock(productID, almaty), "Алматы не тронута")
	s.Equal(map[string]int{"Москва:task_consume": 1}, s.movements(taskID))

	_, card, raw := s.getCard(s.admin, taskID)
	var reserved string
	for _, cs := range card.Stock {
		if cs.ProductID == productID && cs.WarehouseID == moscow {
			reserved = cs.TaskReserved.String()
		}
	}
	s.Equal("2.000", reserved, "card.stock — по паре товар×склад: %s", raw)

	s.moveTask(taskID, "Новые")
	s.Equal("5.000", s.pairStock(productID, moscow), "возврат на Москву")
	s.Equal("20.000", s.pairStock(productID, almaty))
	s.Equal(map[string]int{"Москва:task_consume": 1, "Москва:task_return": 1}, s.movements(taskID))
}

// TS-005: смена склада у списанной строки товара → возврат на старый склад, списание с
// нового (по движению на каждый склад; с исходным списанием — 2+2 в журнале после
// обратной смены).
func (s *OrderCardLiveSuite) TestWarehouses_TS005_ChangeWarehouseOfConsumedLine() {
	almaty, moscow := s.warehouseID("Алматы"), s.warehouseID("Москва")
	taskID, _ := s.newTask(s.newClient("Клиент-Склад-Смена"), s.columnID("В работе"))
	svcID := s.newService("100")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	subID := m.Card.Subtasks[0].ID

	productID := s.newProduct("10", "10", "0") // Алматы: 10
	s.setPairStock(productID, moscow, "10")    // Москва: 10

	st, m, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+subID+"/products",
		map[string]any{"product_id": productID, "qty": 3})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	rowID := m.Card.Subtasks[0].Products[0].ID
	s.Equal(almaty, m.Card.Subtasks[0].Products[0].WarehouseID, "без склада — первый активный (Алматы)")
	s.Equal("7.000", s.pairStock(productID, almaty))

	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+rowID,
		map[string]any{"qty": 3, "price": 10, "warehouse_id": moscow})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(moscow, m.Card.Subtasks[0].Products[0].WarehouseID)
	s.Equal("10.000", s.pairStock(productID, almaty), "возврат на старый склад")
	s.Equal("7.000", s.pairStock(productID, moscow), "списание с нового")
	s.Equal(map[string]int{"Алматы:task_consume": 1, "Алматы:task_return": 1, "Москва:task_consume": 1}, s.movements(taskID))

	// обратно на Алматы — ещё возврат+списание: 2+2 в журнале.
	st, _, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+rowID,
		map[string]any{"qty": 3, "price": 10, "warehouse_id": almaty})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal("7.000", s.pairStock(productID, almaty))
	s.Equal("10.000", s.pairStock(productID, moscow))
	s.Equal(map[string]int{"Алматы:task_consume": 2, "Алматы:task_return": 1, "Москва:task_consume": 1, "Москва:task_return": 1}, s.movements(taskID))

	// PUT без warehouse_id — склад строки не меняется.
	st, m, raw = s.doCard(s.admin, http.MethodPut, "/api/v1/subtask-products/"+rowID, map[string]any{"qty": 3, "price": 10})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(almaty, m.Card.Subtasks[0].Products[0].WarehouseID)

	// склад не существует / архивный / не uuid → 400 warehouse_id.
	_, err := s.Pool.Exec(context.Background(), `INSERT INTO warehouses (name, position, is_active) VALUES ('Архив-TS005', 99, false)`)
	s.Require().NoError(err)
	archived := s.warehouseID("Архив-TS005")
	for _, wh := range []string{"00000000-0000-0000-0000-000000000000", archived, "msk"} {
		st, raw := s.admin.Do(http.MethodPut, "/api/v1/subtask-products/"+rowID, map[string]any{"qty": 3, "price": 10, "warehouse_id": wh})
		s.Equal(http.StatusBadRequest, st, "%s → %s", wh, raw)
		s.Contains(s.fields(raw), "warehouse_id", string(raw))
	}
}

// TS-006 (подфаза A): склад по умолчанию строки товара и комплекта услуги — первый
// активный склад по position (склад оборудования услуги — подфаза C, DS-008).
func (s *OrderCardLiveSuite) TestWarehouses_TS006_DefaultWarehouse() {
	almaty := s.warehouseID("Алматы")
	taskID, _ := s.newTask(s.newClient("Клиент-Склад-Умолчание"), s.columnID("Новые"))
	productID := s.newProduct("10", "10", "0")
	svcID := s.newService("100")
	s.addServiceKit(svcID, productID, "1", 1)

	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svcID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().Len(m.Card.Subtasks[0].Products, 1, "комплект скопирован")
	s.Equal(almaty, m.Card.Subtasks[0].Products[0].WarehouseID, "комплект — со складом по умолчанию")

	// карточка отдаёт склады для select'а (активные по position).
	s.Require().GreaterOrEqual(len(m.Card.Warehouses), 2)
	s.Equal("Алматы", m.Card.Warehouses[0].Name)
	s.Equal("Москва", m.Card.Warehouses[1].Name)
}
