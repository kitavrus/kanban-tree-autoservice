//go:build integration

package board

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// OrphanSweepSuite — AttachmentRepo.ExistingKeys как filestore.KeyIndex + проход
// SweepOrphans на реальной БД (без HTTP; бинарь с тикером — attachments_gc_http_live_test.go).
type OrphanSweepSuite struct {
	testutil.BaseSuite

	repo     *AttachmentRepo
	userID   string
	clientID string
	colID    string
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestOrphanSweepSuite(t *testing.T) {
	suite.Run(t, new(OrphanSweepSuite))
}

func (s *OrphanSweepSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	s.repo = NewAttachmentRepo(s.Pool)
	ctx := context.Background()
	s.userID = s.SeedUser("gc-suite-user", "password1", "Сборщик", "admin", true)
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO clients (company_name) VALUES ('Клиент сборщика') RETURNING id`).Scan(&s.clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns WHERE name = 'Приёмка'`).Scan(&s.colID))
}

func (s *OrphanSweepSuite) newTask() int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, 'Заказ сборщика', 1024, $3)
		RETURNING id`, s.colID, s.clientID, s.userID).Scan(&id))
	return id
}

// stored — файл в хранилище (+ строка task_attachments при withRow), состаренный на age.
func (s *OrphanSweepSuite) stored(fs *filestore.LocalFileStore, taskID int64, withRow bool, age time.Duration) string {
	ctx := context.Background()
	key := filestore.NewKey()
	_, err := fs.Put(ctx, key, strings.NewReader("x"), 10)
	s.Require().NoError(err)
	if withRow {
		_, err = s.repo.Insert(ctx, attachmentRow{TaskID: taskID, Kind: "pdf", OriginalName: "a.pdf",
			ContentType: "application/pdf", Size: 1, StorageKey: key, UserID: s.userID})
		s.Require().NoError(err)
	}
	mt := time.Now().Add(-age)
	s.Require().NoError(os.Chtimes(filepath.Join(fs.Dir(), key), mt, mt))
	return key
}

func (s *OrphanSweepSuite) TestExistingKeys() {
	ctx := context.Background()
	fs, err := filestore.NewLocal(s.T().TempDir())
	s.Require().NoError(err)
	taskID := s.newTask()
	a := s.stored(fs, taskID, true, 0)
	b := s.stored(fs, taskID, true, 0)
	missing := filestore.NewKey()

	got, err := s.repo.ExistingKeys(ctx, []string{a, missing, b})
	s.Require().NoError(err)
	s.Equal(map[string]struct{}{a: {}, b: {}}, got)

	got, err = s.repo.ExistingKeys(ctx, []string{})
	s.Require().NoError(err)
	s.Empty(got)
}

// TestSweep_RealDB — сироты всех трёх происхождений удаляются, живые/свежие — нет:
// нет строки (падение между Put и INSERT), строка удалена (падение после commit DELETE),
// строка ушла каскадом при удалении заказа (падение до удаления файлов заказа).
func (s *OrphanSweepSuite) TestSweep_RealDB() {
	ctx := context.Background()
	fs, err := filestore.NewLocal(s.T().TempDir())
	s.Require().NoError(err)
	taskID, doomedTask := s.newTask(), s.newTask()

	alive := s.stored(fs, taskID, true, 2*time.Hour)
	noRow := s.stored(fs, taskID, false, 2*time.Hour)
	youngNoRow := s.stored(fs, taskID, false, time.Minute)
	deletedRow := s.stored(fs, taskID, true, 2*time.Hour)
	cascaded := s.stored(fs, doomedTask, true, 2*time.Hour)
	_, err = s.Pool.Exec(ctx, `DELETE FROM task_attachments WHERE storage_key = $1`, deletedRow)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, doomedTask)
	s.Require().NoError(err)

	res, err := fs.SweepOrphans(ctx, s.repo, time.Hour, time.Now(), quiet)
	s.Require().NoError(err)
	s.Equal(filestore.SweepResult{Scanned: 5, Young: 1, Removed: 3}, res)

	for key, want := range map[string]bool{alive: true, youngNoRow: true, noRow: false, deletedRow: false, cascaded: false} {
		_, err := os.Stat(filepath.Join(fs.Dir(), key))
		s.Equal(want, err == nil, "ключ %s: ожидалось наличие=%v", key, want)
	}

	// повторный проход идемпотентен
	res, err = fs.SweepOrphans(ctx, s.repo, time.Hour, time.Now(), quiet)
	s.Require().NoError(err)
	s.Zero(res.Removed)
}

// TestSweep_DBUnavailableKeepsFiles — БД недоступна → ошибка прохода, ни один ключевой
// файл не удалён (без ответа БД живой файл не отличить от сироты).
func (s *OrphanSweepSuite) TestSweep_DBUnavailableKeepsFiles() {
	ctx := context.Background()
	fs, err := filestore.NewLocal(s.T().TempDir())
	s.Require().NoError(err)
	key := s.stored(fs, s.newTask(), false, 2*time.Hour)

	pool, err := pgxpool.New(ctx, s.DSN)
	s.Require().NoError(err)
	pool.Close() // запрос в закрытый пул — ошибка, как при недоступной БД
	closed := NewAttachmentRepo(pool)
	_, err = fs.SweepOrphans(ctx, closed, time.Hour, time.Now(), quiet)
	s.Require().Error(err)
	_, statErr := os.Stat(filepath.Join(fs.Dir(), key))
	s.NoError(statErr, "файл на месте")
}
