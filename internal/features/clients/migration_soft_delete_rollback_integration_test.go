//go:build integration

package clients_test

import (
	"context"
	"errors"

	"github.com/golang-migrate/migrate/v4"
)

// TS-012, откат 0013 на данных с удалёнными и одноимёнными записями: накат → удалённый клиент
// с единицей, ушедшей вместе с ним (deleted_with_client), его действующий тёзка, единица,
// удалённая отдельно у действующего клиента, и её действующая копия (тот же тип, модель,
// серийный номер) → откат → накат. Откат выполним (названия клиентов и серийные номера не
// уникальны): колонки уходят, все записи остаются и снова действующие — и ушедшие с клиентом,
// и удалённые отдельно; телефоны на месте.
func (s *ClientsSoftDeleteMigrationSuite) TestTS012_Migration0021_RollbackWithDeleted() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(12))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Require().NoError(mig.Migrate(13))

	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'csd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, csdAdminID)
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'csd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, csdSecondID)
	one := func(q string, args ...any) string {
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, q, args...).Scan(&id), q)
		return id
	}
	typeID := one(`INSERT INTO equipment_types (name) VALUES ('Тип отката 0013') RETURNING id`)
	gone := one(`INSERT INTO clients (company_name) VALUES ('Тёзка 0013') RETURNING id`)
	namesake := one(`INSERT INTO clients (company_name) VALUES ('Тёзка 0013') RETURNING id`)
	defer func() {
		// единицы уходят каскадом вместе с клиентами.
		s.exec(`DELETE FROM clients WHERE id = ANY($1::uuid[])`, []string{gone, namesake})
		s.exec(`DELETE FROM equipment_types WHERE id = $1`, typeID)
	}()
	unit := func(clientID string) string {
		return one(`INSERT INTO equipment (client_id, type_id, model, serial) VALUES ($1, $2, 'M-0013', 'SN-0013') RETURNING id`, clientID, typeID)
	}
	withClient, apart, twin := unit(gone), unit(namesake), unit(namesake)
	s.exec(`INSERT INTO client_phones (client_id, position, phone) VALUES ($1, 0, '111')`, gone)

	// клиент удалён вместе с единицей (те же «кто, когда, почему»); ещё одна — удалена отдельно.
	s.exec(`UPDATE clients SET deleted_at = now() - interval '1 day', deleted_by = $2, delete_reason = 'Закрылся' WHERE id = $1`, gone, csdSecondID)
	s.exec(`UPDATE equipment SET deleted_at = now() - interval '1 day', deleted_by = $2, delete_reason = 'Закрылся', deleted_with_client = true WHERE id = $1`,
		withClient, csdSecondID)
	s.exec(`UPDATE equipment SET deleted_at = now(), deleted_by = $2, delete_reason = 'Списана' WHERE id = $1`, apart, csdAdminID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, csdSecondID)
	s.Error(err, "удаление пользователя, удалившего клиента (RESTRICT)")

	// ---- down → 0012: колонки ушли, записи на месте ----
	s.Require().NoError(mig.Migrate(12))
	s.Empty(s.columns("clients"))
	s.Empty(s.columns("equipment"))
	for _, c := range []string{"clients_deleted_chk", "equipment_deleted_chk", "equipment_deleted_with_client_chk"} {
		s.False(s.exists("constraint", c), c)
	}
	for _, i := range []string{"clients_deleted_at_idx", "equipment_deleted_at_idx"} {
		s.False(s.exists("index", i), i)
	}
	s.Equal("2", s.scalar(`SELECT count(*)::text FROM clients WHERE id = ANY($1::uuid[]) AND company_name = 'Тёзка 0013'`, []string{gone, namesake}),
		"клиенты-тёзки остались, названия не тронуты")
	s.Equal("3", s.scalar(`SELECT count(*)::text FROM equipment WHERE id = ANY($1::uuid[]) AND serial = 'SN-0013'`,
		[]string{withClient, apart, twin}), "единицы остались у своих клиентов")
	s.Equal(gone, s.scalar(`SELECT client_id::text FROM equipment WHERE id = $1`, withClient))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM client_phones WHERE client_id = $1`, gone))
	s.exec(`DELETE FROM users WHERE id = $1`, csdSecondID) // FK ушёл вместе с колонкой

	// ---- повторный up: все записи действующие, различие «вместе с клиентом» потеряно ----
	s.Require().NoError(mig.Migrate(13))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM clients WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM equipment
		WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL OR deleted_with_client`))
	s.exec(`UPDATE equipment SET deleted_at = now(), deleted_by = $2, delete_reason = 'Снова', deleted_with_client = true WHERE id = $1`, withClient, csdAdminID)
}
