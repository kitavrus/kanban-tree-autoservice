//go:build integration

package suppliers_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SuppliersMigrationSuite — миграция 0009 (docs/features/suppliers/draft-spec.md §4.1,
// DS-007; часть TS-020 — «уникальность без регистра и с пробелами по краям, включая
// архивные» держит индекс suppliers_name_lower_uq). down → таблицы нет; up → таблица,
// дефолты, уникальный индекс, CHECK'и длины; повторный down/up чистый.
type SuppliersMigrationSuite struct {
	testutil.BaseSuite
}

func TestSuppliersMigrationSuite(t *testing.T) {
	suite.Run(t, new(SuppliersMigrationSuite))
}

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *SuppliersMigrationSuite) tableExists() bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT to_regclass('suppliers') IS NOT NULL`).Scan(&ok))
	return ok
}

func (s *SuppliersMigrationSuite) pgCode(err error) string {
	var pgErr *pgconn.PgError
	s.Require().True(errors.As(err, &pgErr), "ожидалась ошибка PostgreSQL: %v", err)
	return pgErr.Code
}

func (s *SuppliersMigrationSuite) TestTS020_Migration0012_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(8))
	// Вернуть схему на последнюю версию; пока 0009 — последняя, Up() отвечает ErrNoChange.
	defer func() {
		if err := mig.Up(); !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.False(s.tableExists(), "до 0009 таблицы suppliers нет")

	// --- up 0009 ---
	s.Require().NoError(mig.Migrate(9))
	s.True(s.tableExists())

	// колонки и типы — §4.1.
	rows, err := s.Pool.Query(ctx, `
		SELECT column_name, data_type, is_nullable FROM information_schema.columns
		WHERE table_name = 'suppliers' ORDER BY ordinal_position`)
	s.Require().NoError(err)
	type col struct{ name, typ, nullable string }
	var cols []col
	for rows.Next() {
		var c col
		s.Require().NoError(rows.Scan(&c.name, &c.typ, &c.nullable))
		cols = append(cols, c)
	}
	rows.Close()
	s.Require().NoError(rows.Err())
	s.Equal([]col{
		{"id", "uuid", "NO"},
		{"name", "text", "NO"},
		{"contact_name", "text", "NO"},
		{"phone", "text", "NO"},
		{"address", "text", "NO"},
		{"note", "text", "NO"},
		{"is_active", "boolean", "NO"},
		{"created_at", "timestamp with time zone", "NO"},
		{"updated_at", "timestamp with time zone", "NO"},
	}, cols)

	// дефолты: только название — остальное заполняет БД.
	var id, contact, phone, address, note string
	var active, stamped bool
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO suppliers (name) VALUES ('ООО Миграция')
		 RETURNING id::text, contact_name, phone, address, note, is_active, created_at IS NOT NULL AND updated_at IS NOT NULL`).
		Scan(&id, &contact, &phone, &address, &note, &active, &stamped))
	s.Len(id, 36)
	s.Equal("", contact)
	s.Equal("", phone)
	s.Equal("", address)
	s.Equal("", note)
	s.True(active)
	s.True(stamped)

	// уникальность названия: регистр + пробелы по краям, включая архивные.
	_, err = s.Pool.Exec(ctx, `INSERT INTO suppliers (name) VALUES ('  ооо МИГРАЦИЯ ')`)
	s.Equal("23505", s.pgCode(err))
	_, err = s.Pool.Exec(ctx, `UPDATE suppliers SET is_active = false WHERE id = $1::uuid`, id)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO suppliers (name) VALUES ('ооо миграция')`)
	s.Equal("23505", s.pgCode(err), "архивный поставщик тоже занимает название")
	var idx string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT indexdef FROM pg_indexes WHERE tablename = 'suppliers' AND indexname = 'suppliers_name_lower_uq'`).Scan(&idx))
	s.Contains(idx, "UNIQUE")

	// CHECK'и: пустое название и превышение длины не проходят мимо сервиса.
	for _, q := range []struct {
		sql string
		arg string
	}{
		{`INSERT INTO suppliers (name) VALUES ($1)`, "   "},
		{`INSERT INTO suppliers (name) VALUES ($1)`, strings.Repeat("я", 201)},
		{`INSERT INTO suppliers (name, contact_name) VALUES ('CHK-1', $1)`, strings.Repeat("я", 201)},
		{`INSERT INTO suppliers (name, phone) VALUES ('CHK-2', $1)`, strings.Repeat("1", 51)},
		{`INSERT INTO suppliers (name, address) VALUES ('CHK-3', $1)`, strings.Repeat("я", 501)},
		{`INSERT INTO suppliers (name, note) VALUES ('CHK-4', $1)`, strings.Repeat("я", 1001)},
	} {
		_, err = s.Pool.Exec(ctx, q.sql, q.arg)
		s.Equal("23514", s.pgCode(err), q.sql)
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO suppliers (name, contact_name, phone, address, note) VALUES ($1, $2, $3, $4, $5)`,
		strings.Repeat("я", 200), strings.Repeat("я", 200), strings.Repeat("1", 50), strings.Repeat("я", 500), strings.Repeat("я", 1000))
	s.Require().NoError(err, "границы длины валидны")

	// --- down → 0008: таблица удалена; up снова поднимается чисто (пустой) ---
	s.Require().NoError(mig.Migrate(8))
	s.False(s.tableExists())
	s.Require().NoError(mig.Migrate(9))
	var cnt int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM suppliers`).Scan(&cnt))
	s.Equal(0, cnt)
}
