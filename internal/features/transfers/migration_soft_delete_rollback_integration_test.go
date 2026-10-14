//go:build integration

package transfers_test

import (
	"context"
	"errors"

	"github.com/golang-migrate/migrate/v4"
)

// TS-012, откат 0019 на данных с удалёнными документами: накат → удалённые «Заявка» (её единица
// оборудования освобождена, как делает удаление) и «Отменён», действующие заявка-двойник с тем
// же маршрутом и проведённые документы → откат → накат. Откат выполним: колонки уходят,
// документы остаются в прежних статусах и снова действующие; единицу ожившей заявки откат
// обратно НЕ занимает (active_transfer_id остаётся NULL — см. комментарий down-файла).
func (s *TransfersSoftDeleteMigrationSuite) TestTS012_Migration0019_RollbackWithDeleted() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(18))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Require().NoError(mig.Migrate(19))

	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, tsdAdminID)
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tsd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, tsdSecondID)
	almaty := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	moscow := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	var ids []int64
	doc := func(status string) int64 {
		var id int64
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, created_by)
			VALUES ('warehouse_to_warehouse', $1, $2, $3, $4) RETURNING id`, almaty, moscow, status, tsdAdminID).Scan(&id))
		ids = append(ids, id)
		return id
	}
	var clientID, typeID, unitID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('Клиент отката 0019') RETURNING id`).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO equipment_types (name) VALUES ('Тип отката 0019') RETURNING id`).Scan(&typeID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO equipment (client_id, type_id, model, location_status, warehouse_id)
		VALUES ($1, $2, 'M-0019', 'at_warehouse', $3) RETURNING id`, clientID, typeID, almaty).Scan(&unitID))
	defer func() {
		// порядок — от ссылающихся к справочникам (FK RESTRICT); единица уходит с клиентом.
		s.exec(`DELETE FROM transfers WHERE id = ANY($1)`, ids)
		s.exec(`DELETE FROM clients WHERE id = $1`, clientID)
		s.exec(`DELETE FROM equipment_types WHERE id = $1`, typeID)
	}()

	requested, cancelled := doc("requested"), doc("cancelled")
	twin, inTransit, received := doc("requested"), doc("in_transit"), doc("received")
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 1)`, requested, unitID)
	// удаление заявки освобождает её единицы (active_transfer_id = NULL) — состояние «после удаления».
	s.exec(`UPDATE transfers SET deleted_at = now() - interval '1 day', deleted_by = $2, delete_reason = 'Создана по ошибке' WHERE id = $1`, requested, tsdSecondID)
	s.exec(`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = 'Дубль' WHERE id = $1`, cancelled, tsdAdminID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, tsdSecondID)
	s.Error(err, "удаление пользователя, удалившего документ (RESTRICT)")

	// ---- down → 0018: колонки ушли, документы на месте в прежних статусах ----
	s.Require().NoError(mig.Migrate(18))
	s.Empty(s.columns())
	s.False(s.exists("constraint", "transfers_deleted_chk"))
	s.False(s.exists("index", "transfers_deleted_at_idx"))
	s.Equal("requested,cancelled,requested,in_transit,received", s.scalar(`
		SELECT string_agg(status, ',' ORDER BY id) FROM transfers WHERE id = ANY($1)`, ids))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = $1`, requested), "состав ожившей заявки на месте")
	s.Equal("true", s.scalar(`SELECT (active_transfer_id IS NULL AND location_status = 'at_warehouse')::text FROM equipment WHERE id = $1`, unitID),
		"единицу ожившей заявки откат не занимает")
	s.exec(`DELETE FROM users WHERE id = $1`, tsdSecondID) // FK ушёл вместе с колонкой

	// ---- повторный up: документы действующие, ограничения снова работают ----
	s.Require().NoError(mig.Migrate(19))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM transfers WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`))
	s.failsCheck("удаление проведённого документа после повторного наката",
		`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = 'x' WHERE id = ANY($1)`, []int64{inTransit, received}, tsdAdminID)
	s.exec(`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = 'Снова' WHERE id = ANY($1)`, []int64{requested, twin}, tsdAdminID)
}
