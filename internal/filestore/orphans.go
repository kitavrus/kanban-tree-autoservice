package filestore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Сборщик файлов-сирот. Файл вложения пишется на диск ДО INSERT строки, а при удалении
// стирается ПОСЛЕ commit — аварийное падение процесса между этими шагами оставляет файл
// без строки в БД. Такой файл никто не прочитает, его место не освобождается никогда.
//
// Сиротой считается объект каталога, у которого:
//   - mtime старше minAge (Put пишет поток во временный файл, mtime = время последней
//     записи; rename сохраняет его, а строка вставляется сразу после Put — минимальный
//     возраст с запасом отделяет идущую загрузку от брошенной);
//   - имя — uuid-ключ, которого нет в KeyIndex, ИЛИ временный файл Put (".upload-*"):
//     успешный Put его переименовывает, неуспешный удаляет — старый .upload-* остаётся
//     только после падения процесса.
//
// Прочие файлы и каталоги (не ключ и не temp) не трогаются: каталог мог быть указан на
// чужие данные — сборщик удаляет только то, что мог создать сам LocalFileStore.

// tempPrefix — префикс временных файлов Put (os.CreateTemp(dir, tempPrefix+"*")).
const tempPrefix = ".upload-"

// orphanBatch — ключей на один запрос KeyIndex (ограничение размера массива параметра).
const orphanBatch = 500

// KeyIndex — источник истины «какие ключи живы» (строки task_attachments). Возвращает
// подмножество keys, для которых строка существует.
type KeyIndex interface {
	ExistingKeys(ctx context.Context, keys []string) (map[string]struct{}, error)
}

// massGuardMin — предохранитель от массового удаления: столько и больше старых ключей, и
// НИ ОДНОГО нет в БД → проход прерывается без удаления ключей (ErrSweepSuspicious). Так
// выглядит не авария, а чужая/пустая БД (не тот DATABASE_URL, восстановленный старый бэкап,
// staging на prod-томе) — стереть весь каталог по её ответу было бы необратимо.
const massGuardMin = 20

// ErrSweepSuspicious — сработал предохранитель massGuardMin; каталог разбирает человек.
var ErrSweepSuspicious = errors.New("filestore: sweep aborted — no stored key is known to the index")

// SweepResult — итог одного прохода (лог/тесты).
type SweepResult struct {
	Scanned int // объектов каталога просмотрено
	Young   int // пропущено как моложе minAge
	Removed int // удалено сирот (ключи + временные файлы)
	Failed  int // сироты, которые не удалось удалить (ошибка ФС)
}

// SweepOrphans — один проход сборщика по каталогу хранилища. now — «текущее время»
// (параметр для детерминированных тестов). Ключи сверяются с KeyIndex ЦЕЛИКОМ до первого
// удаления: ошибка индекса или срабатывание предохранителя не удаляют ни одного ключевого
// файла (без ответа БД живой файл не отличить от сироты).
func (s *LocalFileStore) SweepOrphans(ctx context.Context, idx KeyIndex, minAge time.Duration, now time.Time, logger *slog.Logger) (SweepResult, error) {
	var res SweepResult
	if minAge <= 0 {
		return res, fmt.Errorf("filestore: sweep min age must be positive, got %s", minAge)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return res, fmt.Errorf("filestore: read dir: %w", err)
	}

	cutoff := now.Add(-minAge)
	var keys []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		isTemp := strings.HasPrefix(name, tempPrefix)
		if !isTemp && !isKey(name) {
			continue
		}
		res.Scanned++
		info, err := e.Info()
		if errors.Is(err, os.ErrNotExist) {
			continue // удалён параллельно (Delete / соседний инстанс)
		}
		if err != nil {
			return res, fmt.Errorf("filestore: stat %s: %w", name, err)
		}
		if info.ModTime().After(cutoff) {
			res.Young++
			continue
		}
		if isTemp {
			s.removeOrphan(ctx, logger, name, &res)
			continue
		}
		keys = append(keys, name)
	}

	var orphans []string
	for start := 0; start < len(keys); start += orphanBatch {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		batch := keys[start:min(start+orphanBatch, len(keys))]
		alive, err := idx.ExistingKeys(ctx, batch)
		if err != nil {
			return res, fmt.Errorf("filestore: key index: %w", err)
		}
		for _, k := range batch {
			if _, ok := alive[k]; !ok {
				orphans = append(orphans, k)
			}
		}
	}
	if len(keys) >= massGuardMin && len(orphans) == len(keys) {
		return res, fmt.Errorf("%w (%d old keys)", ErrSweepSuspicious, len(keys))
	}
	for _, k := range orphans {
		s.removeOrphan(ctx, logger, k, &res)
	}
	return res, nil
}

// isKey — имя в канонической форме NewKey (lowercase uuid с дефисами). uuid.Validate мало:
// он принимает и 32 hex без дефисов, {uuid}, urn:uuid:, верхний регистр — чужие файлы с
// такими именами (md5 и т.п.) сборщик не трогает.
func isKey(name string) bool {
	u, err := uuid.Parse(name)
	return err == nil && u.String() == name
}

func (s *LocalFileStore) removeOrphan(ctx context.Context, logger *slog.Logger, name string, res *SweepResult) {
	err := os.Remove(filepath.Join(s.dir, name))
	switch {
	case err == nil:
		res.Removed++
		logger.InfoContext(ctx, "orphan file removed", "name", name)
	case errors.Is(err, os.ErrNotExist):
		// уже удалён параллельно — цель достигнута, в счётчики не идёт
	default:
		res.Failed++
		logger.ErrorContext(ctx, "orphan file remove failed", "name", name, "err", err)
	}
}

// RunOrphanCollector — фоновый сборщик: первый проход сразу при старте (сироты появляются
// именно при падении процесса, т.е. к моменту следующего старта), дальше каждые interval.
// Возвращается по отмене ctx. Ошибка прохода логируется и не останавливает сборщик.
func RunOrphanCollector(ctx context.Context, s *LocalFileStore, idx KeyIndex, interval, minAge time.Duration, logger *slog.Logger) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		res, err := s.SweepOrphans(ctx, idx, minAge, time.Now(), logger)
		switch {
		case err != nil && ctx.Err() == nil:
			logger.ErrorContext(ctx, "orphan sweep failed", "err", err, "removed", res.Removed)
		case err == nil:
			logger.InfoContext(ctx, "orphan sweep done", "scanned", res.Scanned, "young", res.Young,
				"removed", res.Removed, "failed", res.Failed)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
