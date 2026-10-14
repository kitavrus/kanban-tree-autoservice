//go:build integration

package users_test

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

// UserTimezoneMigrationSuite — миграция 0010 (docs/features/user-timezone/draft-spec.md
// §4.1, DS-033; часть TS-022 — «существующие пользователи получили Asia/Almaty»):
// down → колонки нет; up → колонка NOT NULL с умолчанием, существующие строки заполнены,
// CHECK длины 1..64; повторный down/up чистый.
type UserTimezoneMigrationSuite struct {
	testutil.BaseSuite
}

func TestUserTimezoneMigrationSuite(t *testing.T) {
	suite.Run(t, new(UserTimezoneMigrationSuite))
}

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *UserTimezoneMigrationSuite) columnExists() bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns
		               WHERE table_name = 'users' AND column_name = 'timezone')`).Scan(&ok))
	return ok
}

func (s *UserTimezoneMigrationSuite) pgCode(err error) string {
	var pgErr *pgconn.PgError
	s.Require().True(errors.As(err, &pgErr), "ожидалась ошибка PostgreSQL: %v", err)
	return pgErr.Code
}

func (s *UserTimezoneMigrationSuite) TestTS022_Migration0013_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(9))
	// Вернуть схему на последнюю версию; пока 0010 — последняя, Up() отвечает ErrNoChange.
	defer func() {
		if err := mig.Up(); !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.False(s.columnExists(), "до 0010 колонки users.timezone нет")

	// «Существующий» пользователь — заведён ДО миграции.
	var oldID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ('tz-mig-old', 'x', 'До миграции', 'executor')
		 RETURNING id::text`).Scan(&oldID))
	defer func() { _, _ = s.Pool.Exec(ctx, `DELETE FROM users WHERE login LIKE 'tz-mig-%'`) }()

	// --- up 0010 ---
	s.Require().NoError(mig.Migrate(10))
	s.True(s.columnExists())

	var typ, nullable, def string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT data_type, is_nullable, column_default FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = 'timezone'`).Scan(&typ, &nullable, &def))
	s.Equal("text", typ)
	s.Equal("NO", nullable)
	s.Contains(def, "Asia/Almaty")

	// Существующие пользователи получили Алматы (DS-033) — все, без NULL и пустых.
	var tz string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1::uuid`, oldID).Scan(&tz))
	s.Equal("Asia/Almaty", tz)
	var notAlmaty int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE timezone <> 'Asia/Almaty'`).Scan(&notAlmaty))
	s.Zero(notAlmaty, "сразу после миграции у всех — умолчание")

	// Новый пользователь без поля → умолчание.
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ('tz-mig-new', 'x', 'После миграции', 'executor')
		 RETURNING timezone`).Scan(&tz))
	s.Equal("Asia/Almaty", tz)

	// CHECK длины 1..64: пустое и 65 символов не проходят мимо сервиса; границы валидны.
	_, err = s.Pool.Exec(ctx, `UPDATE users SET timezone = '' WHERE id = $1::uuid`, oldID)
	s.Equal("23514", s.pgCode(err))
	_, err = s.Pool.Exec(ctx, `UPDATE users SET timezone = $2 WHERE id = $1::uuid`, oldID, strings.Repeat("a", 65))
	s.Equal("23514", s.pgCode(err))
	_, err = s.Pool.Exec(ctx, `UPDATE users SET timezone = NULL WHERE id = $1::uuid`, oldID)
	s.Equal("23502", s.pgCode(err), "NOT NULL")
	_, err = s.Pool.Exec(ctx, `UPDATE users SET timezone = $2 WHERE id = $1::uuid`, oldID, strings.Repeat("a", 64))
	s.Require().NoError(err, "64 символа — граница валидна")
	_, err = s.Pool.Exec(ctx, `UPDATE users SET timezone = 'Europe/Moscow' WHERE id = $1::uuid`, oldID)
	s.Require().NoError(err)

	// --- down → 0009: колонка (и CHECK) удалены, пользователи на месте; up поднимается чисто ---
	s.Require().NoError(mig.Migrate(9))
	s.False(s.columnExists())
	var kept int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE login LIKE 'tz-mig-%'`).Scan(&kept))
	s.Equal(2, kept, "откат не удаляет пользователей")

	s.Require().NoError(mig.Migrate(10))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1::uuid`, oldID).Scan(&tz))
	s.Equal("Asia/Almaty", tz, "после отката и повторного наката — снова умолчание")
}
