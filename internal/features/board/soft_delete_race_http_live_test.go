//go:build integration

package board_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Гонка «мягкое удаление заказа ↔ запись части заказа» (soft-delete-core DS-026, DS-013;
// code review, fix-loop 2) — методы AttachmentsLiveSuite (каталог вложений на диске).
// Журнал работ и вложения не берут заказ под FOR UPDATE, поэтому их запись блокирует строку
// заказа FOR KEY SHARE с условием «не удалён». Гонка воспроизводится детерминированно, без
// sleep: тест сам держит блокировку строки заказа в открытой транзакции, дожидается по
// pg_stat_activity, что запросы сервера встали в ожидание именно этой блокировки, и только
// потом фиксирует транзакцию.

// lockWaiters — число серверных соединений, ждущих блокировку строки (wait_event_type Lock).
func (s *AttachmentsLiveSuite) lockWaiters() int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND pid <> pg_backend_pid()`).Scan(&n))
	return n
}

// raceSubtask — услуга заказа с одной записью журнала (SQL: фикстура); возвращает услугу и запись.
func (s *AttachmentsLiveSuite) raceSubtask(taskID int64) (subtaskID, logID string) {
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		WITH cat AS (INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Race Svc ' || gen_random_uuid()) RETURNING id),
		     svc AS (INSERT INTO services (category_id, name, price) SELECT id, 'Услуга гонки', 100 FROM cat RETURNING id)
		INSERT INTO subtasks (task_id, service_id, status, position, price) SELECT $1, id, 'new', 1, 100 FROM svc RETURNING id`,
		taskID).Scan(&subtaskID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO subtask_logs (subtask_id, user_id, text)
		SELECT $1, id, 'Запись до удаления' FROM users WHERE login = 'att-admin' RETURNING id`, subtaskID).Scan(&logID))
	return subtaskID, logID
}

func (s *AttachmentsLiveSuite) raceBrowser() *testutil.Browser {
	b := s.NewBrowser()
	b.Login("att-admin", "password1")
	return b
}

// Удаление заказа идёт (строка заказа под FOR UPDATE, отметка удаления ещё не зафиксирована) —
// в это окно приходят правки частей заказа. Каждая обязана дождаться удаления и получить 404
// своего вида: запись журнала не добавлена, не изменена и не удалена; вложение не удалено и
// его файл на диске цел; новое вложение не загружено (его файл с диска убран).
func (s *AttachmentsLiveSuite) TestRace_PartWritesWaitForTaskSoftDelete() {
	ctx := context.Background()
	taskID := s.newTask()
	subtaskID, logID := s.raceSubtask(taskID)
	kept := s.mustUpload(s.admin, taskID, "остаётся.png", fileOf(pngSig, 100))
	keptKey := s.storageKey(kept.ID)
	filesBefore := len(s.diskFiles())

	// «Удаление в полёте»: те же блокировка и UPDATE, что делает TaskService.Delete.
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT id FROM tasks WHERE id = $1 FOR UPDATE`, taskID)
	s.Require().NoError(err)
	_, err = tx.Exec(ctx, `
		UPDATE tasks SET deleted_at = now(), delete_reason = 'гонка', version = version + 1,
		       deleted_by = (SELECT id FROM users WHERE login = 'att-admin')
		WHERE id = $1`, taskID)
	s.Require().NoError(err)

	type result struct {
		status int
		raw    []byte
	}
	requests := []struct {
		name, code string
		do         func(b *testutil.Browser) (int, []byte)
	}{
		{"добавление записи", "SUBTASK_NOT_FOUND", func(b *testutil.Browser) (int, []byte) {
			return b.Do(http.MethodPost, "/api/v1/subtasks/"+subtaskID+"/logs", map[string]any{"text": "Запись в удалённый заказ"})
		}},
		{"правка записи", "SUBTASK_LOG_NOT_FOUND", func(b *testutil.Browser) (int, []byte) {
			return b.Do(http.MethodPut, "/api/v1/subtask-logs/"+logID, map[string]any{"text": "Исправлено после удаления"})
		}},
		{"удаление записи", "SUBTASK_LOG_NOT_FOUND", func(b *testutil.Browser) (int, []byte) {
			return b.Do(http.MethodDelete, "/api/v1/subtask-logs/"+logID, nil)
		}},
		{"удаление вложения", "ATTACHMENT_NOT_FOUND", func(b *testutil.Browser) (int, []byte) {
			return b.Do(http.MethodDelete, "/api/v1/attachments/"+kept.ID, nil)
		}},
		{"загрузка вложения", "TASK_NOT_FOUND", func(b *testutil.Browser) (int, []byte) {
			st, _, raw := s.upload(b, taskID, "в-удалённый.png", fileOf(pngSig, 100))
			return st, raw
		}},
	}
	browsers := make([]*testutil.Browser, len(requests))
	for i := range requests {
		browsers[i] = s.raceBrowser() // вход — до старта гонки
	}
	results := make([]result, len(requests))
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st, raw := requests[i].do(browsers[i])
			results[i] = result{st, raw}
		}(i)
	}

	// все пять запросов стоят в ожидании блокировки строки заказа — ни один не прошёл мимо.
	s.Eventually(func() bool { return s.lockWaiters() == len(requests) }, 15*time.Second, 20*time.Millisecond,
		"запросы частей заказа ждут блокировку заказа (ждут: %d из %d)", s.lockWaiters(), len(requests))
	s.Require().NoError(tx.Commit(ctx))
	wg.Wait()

	for i, rq := range requests {
		s.Equal(http.StatusNotFound, results[i].status, "%s: %s", rq.name, results[i].raw)
		s.Equal(rq.code, s.errCode(results[i].raw), "%s: %s", rq.name, results[i].raw)
	}

	var logs int
	var text string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT count(*), coalesce(min(text), '') FROM subtask_logs WHERE subtask_id = $1`, subtaskID).Scan(&logs, &text))
	s.Equal(1, logs, "запись в удалённом заказе не появилась и не удалена")
	s.Equal("Запись до удаления", text, "запись удалённого заказа не изменена")
	s.Equal(1, s.countRows(taskID), "строка вложения удалённого заказа на месте, новая не добавлена")
	files := s.diskFiles()
	s.True(files[keptKey], "файл вложения удалённого заказа цел")
	s.Len(files, filesBefore, "файл отклонённой загрузки убран с диска")
}

// Обратный порядок: запись части заказа идёт (строка заказа под FOR KEY SHARE) — удаление
// заказа обязано дождаться её конца, а не пройти параллельно: FOR UPDATE удаления с KEY SHARE
// несовместим. После освобождения блокировки заказ удаляется (200).
func (s *AttachmentsLiveSuite) TestRace_TaskSoftDeleteWaitsForPartWrite() {
	ctx := context.Background()
	taskID := s.newTask()

	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM tasks WHERE id = $1 AND deleted_at IS NULL FOR KEY SHARE`, taskID)
	s.Require().NoError(err)

	var st int
	var raw []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{"reason": "Создан по ошибке"})
	}()

	s.Eventually(func() bool { return s.lockWaiters() == 1 }, 15*time.Second, 20*time.Millisecond, "удаление заказа ждёт запись части")
	var deleted bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM tasks WHERE id = $1`, taskID).Scan(&deleted))
	s.False(deleted, "пока запись части не завершена, заказ не удалён")

	s.Require().NoError(tx.Commit(ctx))
	<-done
	s.Equal(http.StatusOK, st, string(raw))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM tasks WHERE id = $1`, taskID).Scan(&deleted))
	s.True(deleted)
}
