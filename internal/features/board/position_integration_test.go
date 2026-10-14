//go:build integration

package board

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// PositionSuite — ADR-001 (design-sql.md §Position Query): позиция между соседями,
// добавление в пустую/конец колонки, ребалансировка при переполнении масштаба дроби.
// Пакет `board` (не board_test) — nextPosition/queryPosition неэкспортированы.
type PositionSuite struct {
	testutil.BaseSuite
}

func TestPositionSuite(t *testing.T) {
	suite.Run(t, new(PositionSuite))
}

const (
	posAdminID  = "77777777-7777-7777-7777-777777777777"
	posClientID = "66666666-6666-6666-6666-666666666666"
	posColumnID = "55555555-5555-5555-5555-555555555555"
)

func (s *PositionSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES
		($1, 'position-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, posAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'Position Co')
		ON CONFLICT (id) DO NOTHING`, posClientID)
	s.Require().NoError(err)
}

func (s *PositionSuite) SetupTest() {
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `TRUNCATE TABLE columns RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO columns (id, name, color, is_final, consumes_stock, position)
		VALUES ($1, 'Приёмка', '#94A3B8', false, false, 1)`, posColumnID)
	s.Require().NoError(err)
}

func (s *PositionSuite) createTask(position string) int64 {
	ctx := context.Background()
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO tasks (column_id, client_id, title, position, created_by)
		VALUES ($1, $2, 'T', $3::numeric, $4) RETURNING id`, posColumnID, posClientID, position, posAdminID).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *PositionSuite) TestNextPosition_EmptyColumn_AppendsDefault() {
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(ctx) //nolint:errcheck

	pos, err := nextPosition(ctx, tx, posColumnID, 0, nil, nil)
	s.Require().NoError(err)
	s.Equal("1024", pos)
	s.Require().NoError(tx.Commit(ctx))
}

func (s *PositionSuite) TestNextPosition_Between_ReturnsMidpoint() {
	t1 := s.createTask("1024")
	t2 := s.createTask("2048")

	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(ctx) //nolint:errcheck

	pos, err := nextPosition(ctx, tx, posColumnID, 0, &t1, &t2)
	s.Require().NoError(err)
	s.Equal("1536", pos)
	s.Require().NoError(tx.Commit(ctx))
}

// TestNextPosition_DeepScale_TriggersRebalance — прев/некст с 23-значной дробной частью
// (итог многократных бисекций) → scale(pos) > 20 → колонка перенумеровывается (1024*rn)
// один раз, позиция считается заново по уже чистым значениям.
func (s *PositionSuite) TestNextPosition_DeepScale_TriggersRebalance() {
	t1 := s.createTask("1")
	t2 := s.createTask("1.00000000000000000000002")

	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(ctx) //nolint:errcheck

	pos, err := nextPosition(ctx, tx, posColumnID, 0, &t1, &t2)
	s.Require().NoError(err)
	s.Equal("1536", pos)
	s.Require().NoError(tx.Commit(ctx))

	var p1, p2 string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT position::text FROM tasks WHERE id=$1`, t1).Scan(&p1))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT position::text FROM tasks WHERE id=$1`, t2).Scan(&p2))
	s.Equal("1024", p1)
	s.Equal("2048", p2)
}
