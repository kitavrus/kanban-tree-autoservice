//go:build integration

package board_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Итог заказа с накладными клиенту (docs/features/transfers-sales/draft-spec.md §11, TS-014) —
// методы OrderCardLiveSuite (§TD-8): документы создаются и проводятся через API перемещений
// того же сервера.

type salesTransferDTO struct {
	ID      int64       `json:"id"`
	Status  string      `json:"status"`
	Version int         `json:"version"`
	Amount  json.Number `json:"amount"`
}

// invoice — накладная «склад → клиент» заказа со склада «Алматы»: товары [товар, количество,
// цена]; send — сразу отправить.
func (s *OrderCardLiveSuite) invoice(taskID int64, send bool, lines ...[3]any) salesTransferDTO {
	items := make([]map[string]any, len(lines))
	for i, l := range lines {
		items[i] = map[string]any{"kind": "product", "product_id": l[0], "qty_sent": l[1], "price": l[2]}
	}
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "warehouse_to_client", "from_warehouse_id": s.warehouseID("Алматы"), "task_id": taskID,
		"items": items, "send": send,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var t salesTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	return t
}

func (s *OrderCardLiveSuite) invoiceAction(t salesTransferDTO, action string) salesTransferDTO {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/%s", t.ID, action), map[string]any{"version": t.Version})
	s.Require().Equal(http.StatusOK, st, "%s → %s", action, raw)
	var out salesTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	return out
}

// totals — total_amount / transfers_amount / is_paid карточки и total_amount того же заказа в
// GET /tasks (одно выражение итога на проект — значения обязаны совпасть).
func (s *OrderCardLiveSuite) totals(taskID int64, clientID string) (total, sold string, paid bool, card orderCardDTO) {
	st, card, raw := s.getCard(s.admin, taskID)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, raw = s.admin.Do(http.MethodGet, "/api/v1/tasks?client_id="+clientID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var list struct {
		Items []struct {
			ID          int64       `json:"id"`
			TotalAmount json.Number `json:"total_amount"`
		} `json:"items"`
	}
	s.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	found := false
	for _, it := range list.Items {
		if it.ID == taskID {
			found = true
			s.Equal(card.Task.TotalAmount, it.TotalAmount, "GET /tasks total_amount = карточка")
		}
	}
	s.True(found, "заказ должен быть в списке")
	return card.Task.TotalAmount.String(), card.Task.TransfersAmount.String(), card.Task.IsPaid, card
}

// TS-014: total_amount = услуги + товары услуг + накладные в статусах «В пути» и «Получено»;
// заявка и отменённая накладная в сумму не входят; transfers_amount — в карточке, amount — у
// каждого документа блока «Перемещения заказа»; GET /tasks совпадает с карточкой. Негатив:
// накладная другого заказа не влияет; отмена накладной «в пути» убирает её сумму; is_paid
// заказа при отправке накладной не меняется. Печатная форма «Выдача» — строка «Продано по
// накладным» и тот же итог.
func (s *OrderCardLiveSuite) TestTS014_OrderTotalIncludesSentInvoices() {
	ctx := context.Background()
	c := s.newClient("Клиент-Накладные")
	col := s.columnID("Новые")
	taskID, _ := s.newTask(c, col)

	// услуги и товары услуг: 1000 + 2 × 150.50 = 1301.00.
	svc := s.newService("1000")
	st, m, raw := s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": svc})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	p1 := s.newProduct("150.5", "100", "1")
	st, _, raw = s.doCard(s.admin, http.MethodPost, "/api/v1/subtasks/"+m.Card.Subtasks[0].ID+"/products", map[string]any{"product_id": p1, "qty": 2})
	s.Require().Equal(http.StatusCreated, st, string(raw))

	total, sold, paid, card := s.totals(taskID, c)
	s.Equal("1301.00", total)
	s.Equal("0.00", sold, "накладных нет")
	s.False(paid)
	s.Empty(card.Transfers)

	// заявка: сумма у документа есть, в итог заказа не входит.
	goods := s.newProduct("100.10", "100", "1")
	small := s.newProduct("0.99", "100", "1")
	inv := s.invoice(taskID, false, [3]any{goods, 3, 100.10}, [3]any{small, 1, 0.99})
	s.Equal("301.29", inv.Amount.String(), "300.30 + 0.99")
	total, sold, _, card = s.totals(taskID, c)
	s.Equal("1301.00", total, "заявка в итог не входит")
	s.Equal("0.00", sold)
	s.Require().Len(card.Transfers, 1)
	s.Equal("301.29", card.Transfers[0].Amount.String())
	s.Equal("requested", card.Transfers[0].Status)

	// отправка: накладная «в пути» входит в итог; оплата заказа не меняется.
	inv = s.invoiceAction(inv, "send")
	total, sold, paid, _ = s.totals(taskID, c)
	s.Equal("1602.29", total, "1301.00 + 301.29")
	s.Equal("301.29", sold)
	s.False(paid, "отправка накладной оплату заказа не трогает")

	// оплаченный заказ: вторая накладная отправлена и получена — is_paid остаётся true.
	_, err := s.Pool.Exec(ctx, `UPDATE tasks SET is_paid = true, paid_at = now() WHERE id = $1`, taskID)
	s.Require().NoError(err)
	second := s.invoice(taskID, true, [3]any{goods, 2, 50})
	total, sold, paid, _ = s.totals(taskID, c)
	s.Equal("1702.29", total)
	s.Equal("401.29", sold)
	s.True(paid, "отправка накладной оплату заказа не трогает")
	s.invoiceAction(second, "receive")
	inv = s.invoiceAction(inv, "receive")
	total, sold, paid, card = s.totals(taskID, c)
	s.Equal("1702.29", total, "полученные накладные остаются в итоге")
	s.Equal("401.29", sold)
	s.True(paid)

	// накладная «в пути» → отмена: её сумма уходит из итога, у документа остаётся.
	third := s.invoice(taskID, true, [3]any{small, 10, 7.77})
	total, sold, _, _ = s.totals(taskID, c)
	s.Equal("1779.99", total, "1702.29 + 77.70")
	s.Equal("478.99", sold, "401.29 + 77.70")
	third = s.invoiceAction(third, "cancel")
	s.Equal("cancelled", third.Status)
	total, sold, _, card = s.totals(taskID, c)
	s.Equal("1702.29", total, "отменённая накладная в итог не входит")
	s.Equal("401.29", sold)
	amounts := map[int64]string{}
	for _, t := range card.Transfers {
		amounts[t.ID] = t.Status + ":" + t.Amount.String()
	}
	s.Equal(map[int64]string{inv.ID: "received:301.29", second.ID: "received:100.00", third.ID: "cancelled:77.70"}, amounts)

	// приём от клиента (без товаров) — amount 0.00, итог не меняется.
	unit := s.newEquipment(c)
	_, err = s.Pool.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2)`, taskID, unit)
	s.Require().NoError(err)
	st, raw = s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "client_to_warehouse", "to_warehouse_id": s.warehouseID("Алматы"), "task_id": taskID,
		"items": []map[string]any{{"kind": "equipment", "equipment_id": unit}}, "send": true,
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var intake salesTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &intake), string(raw))
	total, sold, _, card = s.totals(taskID, c)
	s.Equal("1702.29", total)
	s.Equal("401.29", sold)
	for _, t := range card.Transfers {
		if t.ID == intake.ID {
			s.Equal("0.00", t.Amount.String(), "приём от клиента суммы не несёт")
		}
	}

	// негатив (abuse case 4): накладная другого заказа того же клиента на итог не влияет.
	otherID, _ := s.newTask(c, col)
	s.invoice(otherID, true, [3]any{goods, 1, 999})
	total, sold, _, _ = s.totals(taskID, c)
	s.Equal("1702.29", total, "сумма считается по task_id")
	s.Equal("401.29", sold)
	total, sold, _, _ = s.totals(otherID, c)
	s.Equal("999.00", total, "заказ без услуг: итог = накладные")
	s.Equal("999.00", sold)

	// округление на «половине» (half-up): 0.15 × 1.5 = 0.225 → 0.23, 0.10 × 0.35 = 0.035 → 0.04 —
	// итог заказа складывает уже округлённые суммы строк.
	halfID, _ := s.newTask(c, col)
	half := s.invoice(halfID, true, [3]any{goods, 1.5, 0.15}, [3]any{small, 0.35, 0.10})
	s.Equal("0.27", half.Amount.String())
	total, sold, _, card = s.totals(halfID, c)
	s.Equal("0.27", total)
	s.Equal("0.27", sold)
	s.Require().Len(card.Transfers, 1)
	s.Equal("0.27", card.Transfers[0].Amount.String())

	// row-мутация карточки отдаёт тот же итог (card в ответе — re-fetch).
	st, m, raw = s.doCard(s.admin, http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/subtasks", taskID), map[string]any{"service_id": s.newService("10")})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Equal("1712.29", m.Card.Task.TotalAmount.String())
	s.Equal("401.29", m.Card.Task.TransfersAmount.String())

	// печатная форма «Выдача»: строка «Продано по накладным» перед итогом, итог — total_amount.
	resp := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover", taskID), nil, nil)
	defer resp.Body.Close()
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	body := string(b)
	s.Contains(body, "Продано по накладным: 401.29")
	s.Contains(body, "Итого: 1712.29")

	// акт заказа, у которого накладная только в заявке, строки не содержит.
	draftID, _ := s.newTask(c, col)
	s.invoice(draftID, false, [3]any{goods, 1, 5})
	respDraft := s.admin.Request(http.MethodGet, fmt.Sprintf("/tasks/%d/print/handover", draftID), nil, nil)
	defer respDraft.Body.Close()
	s.Require().Equal(http.StatusOK, respDraft.StatusCode)
	b, err = io.ReadAll(respDraft.Body)
	s.Require().NoError(err)
	s.NotContains(string(b), "Продано по накладным")
	s.Contains(string(b), "Итого: 0.00")
}
