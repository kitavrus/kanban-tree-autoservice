//go:build integration

package warehouses_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Запрет архивирования склада (docs/features/transfers-core/draft-spec.md DS-026, TS-019):
// пока на складе числится оборудование клиентов либо есть незавершённый документ
// «Перемещение» (заявка / в пути) с этим складом — 409 WAREHOUSE_IN_USE. Методы
// WarehousesLiveSuite.

// whFixture — единицы и документы теста; cleanup возвращает БД в состояние «склады никем не
// заняты», чтобы остальные тесты сьюты (архив всех складов, D47) не зависели от порядка.
type whFixture struct {
	clientID, typeID string
	equipment        []string
}

func (s *WarehousesLiveSuite) newFixture() *whFixture {
	ctx := context.Background()
	f := &whFixture{}
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO clients (company_name) VALUES ('WH in use ' || gen_random_uuid()) RETURNING id::text`).Scan(&f.clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO equipment_types (name) VALUES ('WH in use ' || gen_random_uuid()) RETURNING id::text`).Scan(&f.typeID))
	return f
}

// unitAt — единица оборудования, числящаяся на складе (SQL: фикстура местоположения).
func (s *WarehousesLiveSuite) unitAt(f *whFixture, warehouseID string) string {
	var id string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		INSERT INTO equipment (client_id, type_id, model, location_status, warehouse_id)
		VALUES ($1, $2, 'WH-unit', 'at_warehouse', $3) RETURNING id::text`, f.clientID, f.typeID, warehouseID).Scan(&id))
	f.equipment = append(f.equipment, id)
	return id
}

func (s *WarehousesLiveSuite) cleanup(f *whFixture) {
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM transfers t WHERE EXISTS (
		    SELECT 1 FROM transfer_items i WHERE i.transfer_id = t.id AND i.equipment_id = ANY($1::uuid[]))`, f.equipment)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `DELETE FROM equipment WHERE id = ANY($1::uuid[])`, f.equipment)
	s.Require().NoError(err)
}

type whTransferDTO struct {
	ID      int64 `json:"id"`
	Version int   `json:"version"`
}

func (s *WarehousesLiveSuite) transfer(from, to, equipmentID string, send bool) whTransferDTO {
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/transfers", map[string]any{
		"direction": "warehouse_to_warehouse", "from_warehouse_id": from, "to_warehouse_id": to, "send": send,
		"items": []map[string]any{{"kind": "equipment", "equipment_id": equipmentID}},
	})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var t whTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &t), string(raw))
	return t
}

func (s *WarehousesLiveSuite) transferAction(t whTransferDTO, action string) whTransferDTO {
	st, raw := s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/transfers/%d/%s", t.ID, action), map[string]any{"version": t.Version})
	s.Require().Equal(http.StatusOK, st, "%s: %s", action, raw)
	var out whTransferDTO
	s.Require().NoError(json.Unmarshal(raw, &out), string(raw))
	return out
}

// archive — PUT is_active=false; возвращает статус и код ошибки.
func (s *WarehousesLiveSuite) archive(w warehouseDTO) (int, string) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": false})
	var e struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if st >= 400 {
		s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
		if e.Code == "WAREHOUSE_IN_USE" {
			s.Equal("На складе есть оборудование или незавершённые перемещения", e.Message)
		}
	}
	return st, e.Code
}

func (s *WarehousesLiveSuite) restore(w warehouseDTO) {
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+w.ID, map[string]any{"name": w.Name, "is_active": true})
	s.Require().Equal(http.StatusOK, st, string(raw))
}

func (s *WarehousesLiveSuite) isActive(id string) bool {
	var active bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT is_active FROM warehouses WHERE id = $1`, id).Scan(&active))
	return active
}

// TS-019: на складе числится единица → 409 WAREHOUSE_IN_USE; есть заявка / документ «в пути»
// с этим складом (отправитель или получатель) → 409; склад только с завершёнными
// документами и без оборудования архивируется (негатив).
func (s *WarehousesLiveSuite) TestTS019_ArchiveBlockedWhileInUse() {
	f := s.newFixture()
	defer s.cleanup(f)
	src, dst, idle := s.create("TS019 Источник"), s.create("TS019 Получатель"), s.create("TS019 Свободный")
	defer func() { // вернуть склады в актив для остальных тестов сьюты
		for _, w := range []warehouseDTO{src, dst, idle} {
			if !s.isActive(w.ID) {
				s.restore(w)
			}
		}
	}()

	// на складе числится оборудование.
	unit := s.unitAt(f, src.ID)
	st, code := s.archive(src)
	s.Equal(http.StatusConflict, st)
	s.Equal("WAREHOUSE_IN_USE", code)
	s.True(s.isActive(src.ID), "склад остался активным")

	// заявка: оба склада документа заняты (получатель — без единого оборудования на нём).
	doc := s.transfer(src.ID, dst.ID, unit, false)
	for name, w := range map[string]warehouseDTO{"отправитель заявки": src, "получатель заявки": dst} {
		st, code = s.archive(w)
		s.Equal(http.StatusConflict, st, name)
		s.Equal("WAREHOUSE_IN_USE", code, name)
	}

	// в пути: на отправителе оборудования уже нет (единица «в пути»), но документ не завершён.
	doc = s.transferAction(doc, "send")
	var onSrc int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM equipment WHERE warehouse_id = $1`, src.ID).Scan(&onSrc))
	s.Zero(onSrc, "единица в пути — на складе отправления не числится")
	for name, w := range map[string]warehouseDTO{"отправитель в пути": src, "получатель в пути": dst} {
		st, code = s.archive(w)
		s.Equal(http.StatusConflict, st, name)
		s.Equal("WAREHOUSE_IN_USE", code, name)
	}

	// SSR-архивирование — редирект на список с сообщением, склад активен.
	resp := s.admin.Request(http.MethodPost, "/warehouses/"+src.ID+"/archive", url.Values{}, nil)
	_ = resp.Body.Close()
	s.Equal(http.StatusSeeOther, resp.StatusCode)
	s.Contains(resp.Header.Get("Location"), "error=in_use")
	resp = s.admin.Request(http.MethodGet, "/warehouses?error=in_use", nil, nil)
	page, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Require().NoError(err)
	s.Contains(string(page), "На складе есть оборудование или незавершённые перемещения")
	s.True(s.isActive(src.ID))

	// не связанный склад архивируется как раньше.
	st, _ = s.archive(idle)
	s.Equal(http.StatusOK, st)
	s.restore(idle)

	// приёмка: документ завершён. Отправитель — только завершённые документы и без
	// оборудования → архивируется (негатив TS-019); получатель держит единицу → 409.
	s.transferAction(doc, "receive")
	st, code = s.archive(dst)
	s.Equal(http.StatusConflict, st, "на получателе числится принятая единица")
	s.Equal("WAREHOUSE_IN_USE", code)
	st, _ = s.archive(src)
	s.Equal(http.StatusOK, st, "склад только с завершёнными документами и без оборудования архивируется")
	s.False(s.isActive(src.ID))

	// правка уже архивного склада (is_active=false, переименование) — не архивирование, без 409.
	st, raw := s.admin.Do(http.MethodPut, "/api/v1/warehouses/"+src.ID, map[string]any{"name": "TS019 Источник (архив)", "is_active": false})
	s.Equal(http.StatusOK, st, string(raw))
	src.Name = "TS019 Источник (архив)"

	// отменённый документ тоже завершённый: заявка dst → idle отменена → idle архивируется.
	back := s.transfer(dst.ID, idle.ID, unit, false)
	st, code = s.archive(idle)
	s.Equal(http.StatusConflict, st)
	s.Equal("WAREHOUSE_IN_USE", code)
	s.transferAction(back, "cancel")
	st, _ = s.archive(idle)
	s.Equal(http.StatusOK, st, "после отмены заявки склад свободен")
}

// TS-019: «иначе — как сейчас» — WAREHOUSE_LAST_ACTIVE проверяется раньше WAREHOUSE_IN_USE:
// последний активный склад с оборудованием отвечает прежним кодом.
func (s *WarehousesLiveSuite) TestTS019_LastActiveStillWins() {
	f := s.newFixture()
	defer s.cleanup(f)
	l := s.list("")
	s.Require().NotEmpty(l.Items)
	for i, w := range l.Items {
		if i == 0 {
			continue
		}
		st, code := s.archive(w)
		s.Require().Equal(http.StatusOK, st, "%s: %s", w.Name, code)
		defer s.restore(w)
	}
	last := l.Items[0]
	s.unitAt(f, last.ID)
	st, code := s.archive(last)
	s.Equal(http.StatusConflict, st)
	s.Equal("WAREHOUSE_LAST_ACTIVE", code)
}
