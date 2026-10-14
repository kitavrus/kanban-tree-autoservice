//go:build integration

package board_test

import (
	"context"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// EquipmentWarehousesMigrationSuite — миграция 0010 (warehouses DS-008, D55): существующие
// строки task_equipment → «Алматы»/at_warehouse; CHECK согласованности склад/маршрут;
// журнал task_equipment_movements; down снимает колонки и журнал, строки остаются.
type EquipmentWarehousesMigrationSuite struct {
	testutil.BaseSuite
}

func TestEquipmentWarehousesMigrationSuite(t *testing.T) {
	suite.Run(t, new(EquipmentWarehousesMigrationSuite))
}

const (
	ewmAdminID  = "aaaaaaaa-0010-0010-0010-000000000001"
	ewmClientID = "bbbbbbbb-0010-0010-0010-000000000001"
)

func (s *EquipmentWarehousesMigrationSuite) columnExists(table, column string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column).Scan(&ok))
	return ok
}

func (s *EquipmentWarehousesMigrationSuite) TestUpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrateRoot(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(9))
	defer func() { s.Require().NoError(mig.Up()) }()
	s.False(s.columnExists("task_equipment", "warehouse_id"))

	// фикстура до 0010: заказ + единица в заказе.
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'ewm-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, ewmAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'EWM Co') ON CONFLICT (id) DO NOTHING`, ewmClientID)
	s.Require().NoError(err)
	var typeID, equipmentID, columnID string
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO equipment_types (name) VALUES ('EWM ' || gen_random_uuid()) RETURNING id`).Scan(&typeID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO equipment (client_id, type_id, model) VALUES ($1, $2, 'M') RETURNING id`, ewmClientID, typeID).Scan(&equipmentID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns ORDER BY position LIMIT 1`).Scan(&columnID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, 'EWM', 1, $3) RETURNING id`,
		columnID, ewmClientID, ewmAdminID).Scan(&taskID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2)`, taskID, equipmentID)
	s.Require().NoError(err)

	s.Require().NoError(mig.Migrate(10))
	var status, whName string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT te.status, w.name FROM task_equipment te JOIN warehouses w ON w.id = te.warehouse_id
		WHERE te.task_id = $1`, taskID).Scan(&status, &whName))
	s.Equal("at_warehouse", status)
	s.Equal("Алматы", whName, "существующие строки → Алматы")

	var almaty, moscow string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Алматы'`).Scan(&almaty))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Москва'`).Scan(&moscow))

	// CHECK согласованности: в пути без маршрута / со складом / from = to — ошибка.
	for _, q := range []string{
		`UPDATE task_equipment SET status = 'in_transit' WHERE task_id = $1`,
		`UPDATE task_equipment SET status = 'in_transit', warehouse_id = NULL, transit_from_warehouse_id = $2, transit_to_warehouse_id = $2 WHERE task_id = $1`,
		`UPDATE task_equipment SET status = 'lost' WHERE task_id = $1`,
		`UPDATE task_equipment SET warehouse_id = NULL WHERE task_id = $1`,
	} {
		_, err := s.Pool.Exec(ctx, q, taskID, almaty)
		s.Error(err, q)
	}
	_, err = s.Pool.Exec(ctx, `UPDATE task_equipment SET status = 'in_transit', warehouse_id = NULL,
		transit_from_warehouse_id = $2, transit_to_warehouse_id = $3 WHERE task_id = $1`, taskID, almaty, moscow)
	s.Require().NoError(err, "корректный «в пути»")

	_, err = s.Pool.Exec(ctx, `INSERT INTO task_equipment_movements (task_id, equipment_id, action, from_warehouse_id, to_warehouse_id, user_id)
		VALUES ($1, $2, 'sent', $3, $4, $5)`, taskID, equipmentID, almaty, moscow, ewmAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO task_equipment_movements (task_id, equipment_id, action, user_id) VALUES ($1, $2, 'teleported', $3)`,
		taskID, equipmentID, ewmAdminID)
	s.Error(err, "action CHECK")

	// down → 0009: колонок и журнала нет, строка task_equipment цела.
	s.Require().NoError(mig.Migrate(9))
	s.False(s.columnExists("task_equipment", "warehouse_id"))
	s.False(s.columnExists("task_equipment", "status"))
	var journal bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT to_regclass('task_equipment_movements') IS NOT NULL`).Scan(&journal))
	s.False(journal)
	var cnt int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM task_equipment WHERE task_id = $1`, taskID).Scan(&cnt))
	s.Equal(1, cnt)

	_, err = s.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	s.Require().NoError(err)
}
