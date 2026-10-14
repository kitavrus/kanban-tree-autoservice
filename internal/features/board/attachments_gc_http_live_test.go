//go:build integration

package board_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// age — состарить файл каталога вложений (mtime) на d: имитация файла, оставшегося от
// процесса, упавшего d назад.
func (s *AttachmentsLiveSuite) age(name string, d time.Duration) {
	mt := time.Now().Add(-d)
	s.Require().NoError(os.Chtimes(filepath.Join(s.uploads, name), mt, mt))
}

// TestAttachments_OrphanCollector_RemovesOnlyOrphans — фоновый сборщик сирот в бинаре
// сервера (UPLOADS_GC_INTERVAL=1s, MIN_AGE по умолчанию 1h) на реальной БД:
//   - старый файл со строкой task_attachments (загружен через API) — остаётся;
//   - старый файл без строки (падение между Put и INSERT) — удаляется;
//   - старый файл, строку которого удалили, а файл — нет (падение после commit DELETE) —
//     удаляется;
//   - брошенный temp .upload-* — удаляется;
//   - свежий файл без строки (идущая загрузка) и чужой файл (не uuid) — остаются.
func (s *AttachmentsLiveSuite) TestAttachments_OrphanCollector_RemovesOnlyOrphans() {
	taskID := s.newTask()
	alive := s.mustUpload(s.admin, taskID, "живой.png", fileOf(pngSig, 64))
	aliveKey := s.storageKey(alive.ID)
	gone := s.mustUpload(s.admin, taskID, "без-строки.pdf", fileOf(pdfSig, 64))
	goneKey := s.storageKey(gone.ID)
	_, err := s.Pool.Exec(context.Background(), `DELETE FROM task_attachments WHERE id = $1`, gone.ID)
	s.Require().NoError(err)

	orphanKey, youngKey := uuid.NewString(), uuid.NewString()
	for _, n := range []string{orphanKey, youngKey, ".upload-crashed", "не-ключ.txt"} {
		s.Require().NoError(os.WriteFile(filepath.Join(s.uploads, n), []byte("x"), 0o600))
	}
	for _, n := range []string{aliveKey, goneKey, orphanKey, ".upload-crashed", "не-ключ.txt"} {
		s.age(n, 2*time.Hour)
	}

	s.Eventually(func() bool {
		f := s.diskFiles()
		return !f[orphanKey] && !f[goneKey] && !f[".upload-crashed"]
	}, 10*time.Second, 100*time.Millisecond, "сироты удалены сборщиком")

	f := s.diskFiles()
	s.True(f[aliveKey], "файл со строкой в БД не тронут")
	s.True(f[youngKey], "свежий файл без строки (идущая загрузка) не тронут")
	s.True(f["не-ключ.txt"], "чужой файл (не uuid-ключ) не тронут")
	s.Require().NoError(os.Remove(filepath.Join(s.uploads, youngKey)))
	s.Require().NoError(os.Remove(filepath.Join(s.uploads, "не-ключ.txt")))
}

// TS-008 (soft-delete-core DS-013): файлы вложений мягко удалённого заказа — не сироты: их
// строки task_attachments остаются, и проход сборщика (UPLOADS_GC_INTERVAL=1s) старые файлы
// удалённого заказа не трогает, пока файл-сирота рядом удаляется (негатив: файл без строки
// по-прежнему убирается). После восстановления заказа файл отдаётся.
func (s *AttachmentsLiveSuite) TestTS008_OrphanCollector_KeepsFilesOfDeletedTask() {
	taskID := s.newTask()
	kept := s.mustUpload(s.admin, taskID, "удалённый-заказ.png", fileOf(pngSig, 64))
	keptKey := s.storageKey(kept.ID)

	st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{"reason": "Создан по ошибке"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _ = s.admin.Do(http.MethodGet, kept.URL, nil)
	s.Equal(http.StatusNotFound, st, "файл удалённого заказа по прямой ссылке не отдаётся")

	orphanKey := uuid.NewString()
	s.Require().NoError(os.WriteFile(filepath.Join(s.uploads, orphanKey), []byte("x"), 0o600))
	s.age(keptKey, 2*time.Hour)
	s.age(orphanKey, 2*time.Hour)

	// сирота исчез — значит, проход сборщика после удаления заказа состоялся.
	s.Eventually(func() bool { return !s.diskFiles()[orphanKey] }, 10*time.Second, 100*time.Millisecond, "сирота удалён сборщиком")
	s.True(s.diskFiles()[keptKey], "файл мягко удалённого заказа сборщик не тронул")
	s.Equal(1, s.countRows(taskID), "строка вложения на месте")

	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _ = s.admin.Do(http.MethodGet, kept.URL, nil)
	s.Equal(http.StatusOK, st, "после восстановления файл отдаётся")
}

// TS-013 (soft-delete-core DS-027, DS-036; прежний LV-210): служебная уборка заказа — строки
// вложений уходят каскадом FK, файлы удаляются с диска после commit; так же — для мягко
// удалённого заказа. Сьюта запущена с TEST_PURGE_ENABLED=true.
func (s *AttachmentsLiveSuite) TestTS013_Purge_RemovesAttachmentFiles() {
	for _, soft := range []bool{false, true} {
		taskID := s.newTask()
		a := s.mustUpload(s.admin, taskID, "a.png", fileOf(pngSig, 100))
		b := s.mustUpload(s.execA, taskID, "b.pdf", fileOf(pdfSig, 100))
		keys := []string{s.storageKey(a.ID), s.storageKey(b.ID)}
		if soft {
			st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{"reason": "Создан по ошибке"})
			s.Require().Equal(http.StatusOK, st, string(raw))
		}

		st, raw := s.execA.Do(http.MethodDelete, fmt.Sprintf("/api/v1/test/purge/tasks/%d", taskID), nil)
		s.Equal(http.StatusForbidden, st, "исполнитель: %s", raw)
		s.Equal(2, s.countRows(taskID))

		st, raw = s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/test/purge/tasks/%d", taskID), nil)
		s.Require().Equal(http.StatusOK, st, "soft=%v: %s", soft, raw)
		s.Equal(0, s.countRows(taskID), "soft=%v", soft)
		files := s.diskFiles()
		for _, k := range keys {
			s.False(files[k], "soft=%v: файл %s удалён с диска", soft, k)
		}
		st, _ = s.admin.Do(http.MethodGet, a.URL, nil)
		s.Equal(http.StatusNotFound, st)
	}
}
