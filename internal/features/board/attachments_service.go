package board

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Вложения заказа (фаза 7, D21). Доступ — по правам карточки фазы 6 (D19: любой
// авторизованный — admin и executor); удаление — автор или admin. Ответ мутаций — тот же
// конверт {card, warnings}, что у row-мутаций карточки (re-fetch-after-write).

// attachmentFileURL — отдача файла (GET, проверка сессии).
func attachmentFileURL(id string) string { return "/api/v1/attachments/" + id + "/file" }

func toCardAttachments(a Actor, rows []attachmentRow) []CardAttachment {
	out := make([]CardAttachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, CardAttachment{
			ID: r.ID, Kind: r.Kind, OriginalName: r.OriginalName, ContentType: r.ContentType, Size: r.Size,
			UserID: r.UserID, AuthorName: r.AuthorName, CreatedAt: r.CreatedAt,
			URL:       attachmentFileURL(r.ID),
			CanDelete: a.IsAdmin || r.UserID == a.ID,
		})
	}
	return out
}

var errFilesUnavailable = errors.New("board: attachment file store is not configured")

// UploadAttachment — POST /api/v1/tasks/:id/attachments (поток файла из multipart). Порядок:
// заказ есть → сигнатура (первые 512 байт) → поток в FileStore под uuid-ключом с лимитом
// по виду → строка в БД; любой сбой после записи файла — файл удаляется.
func (s *CardService) UploadAttachment(ctx context.Context, a Actor, taskID, name string, body io.Reader) (*OrderCardMutationResponse, error) {
	id, ok := parseTaskID(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if s.files == nil {
		return nil, errFilesUnavailable
	}

	ctx, span := otel.Tracer("board").Start(ctx, "attachments.upload")
	defer span.End()
	span.SetAttributes(attribute.Int64("task_id", id))

	exists, err := s.attachments.TaskExists(ctx, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrTaskNotFound
	}

	head := make([]byte, attachmentSniffLen)
	n, err := io.ReadFull(body, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, errorspkg.BadRequest("Не удалось прочитать файл")
	}
	head = head[:n]
	if n == 0 {
		return nil, errorspkg.Validation(errorspkg.Field{Name: "file", Message: "Файл пустой"})
	}
	kind, contentType, ok := sniffAttachment(head)
	if !ok {
		return nil, ErrAttachmentTypeUnsupported
	}
	span.SetAttributes(attribute.String("kind", kind))

	key := filestore.NewKey()
	size, err := s.files.Put(ctx, key, io.MultiReader(bytes.NewReader(head), body), attachmentLimit(kind))
	if errors.Is(err, filestore.ErrTooLarge) {
		return nil, attachmentTooLarge(kind)
	}
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int64("size", size))

	_, err = s.attachments.Insert(ctx, attachmentRow{
		TaskID: id, Kind: kind, OriginalName: sanitizeAttachmentName(name), ContentType: contentType,
		Size: size, StorageKey: key, UserID: a.ID,
	})
	if err != nil {
		s.removeFile(ctx, key)
		if errors.Is(err, errNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return s.cardMutation(ctx, a, id, ReconcileResult{Warnings: []StockWarning{}})
}

// DeleteAttachment — DELETE /api/v1/attachments/:id: 404 → 403 (не автор и не admin) →
// строка удаляется → файл удаляется ПОСЛЕ (ошибка файла логируется, не валит операцию).
func (s *CardService) DeleteAttachment(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrAttachmentNotFound
	}

	ctx, span := otel.Tracer("board").Start(ctx, "attachments.delete")
	defer span.End()

	// Проверка «заказ не удалён», права и DELETE строки — одна транзакция под блокировкой
	// заказа (soft-delete-core DS-013): вложение заказа, удаляемого параллельно, не стирается.
	row, err := s.attachments.Delete(ctx, id, func(row attachmentRow) error {
		if !a.IsAdmin && row.UserID != a.ID {
			return ErrAttachmentForbidden
		}
		return nil
	})
	if errors.Is(err, errNotFound) {
		return nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int64("task_id", row.TaskID))
	// Файл — только после успешного commit удаления строки.
	s.removeFile(ctx, row.StorageKey)

	resp, err := s.cardMutation(ctx, a, row.TaskID, ReconcileResult{Warnings: []StockWarning{}})
	if errors.Is(err, ErrTaskNotFound) {
		// заказ удалён параллельно — вложение всё равно удалено; сущность пути — вложение
		return nil, ErrAttachmentNotFound
	}
	return resp, err
}

// AttachmentFile — метаданные + открытый файл для GET /api/v1/attachments/:id/file.
// Вызывающий закрывает File.
type AttachmentFile struct {
	Kind         string
	OriginalName string
	ContentType  string
	File         io.ReadSeekCloser
	Info         filestore.Info
}

// OpenAttachment — доступ по правам карточки (любой авторизованный, D19). Строка есть, а
// файла на диске нет (удалён руками/потерян) → 404 + warn в лог.
func (s *CardService) OpenAttachment(ctx context.Context, _ Actor, id string) (*AttachmentFile, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrAttachmentNotFound
	}
	if s.files == nil {
		return nil, errFilesUnavailable
	}
	row, err := s.attachments.Get(ctx, id)
	if errors.Is(err, errNotFound) {
		return nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}
	f, info, err := s.files.Open(ctx, row.StorageKey)
	if errors.Is(err, filestore.ErrNotFound) {
		slog.WarnContext(ctx, "attachment file missing on storage", "attachment_id", id, "storage_key", row.StorageKey)
		return nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &AttachmentFile{Kind: row.Kind, OriginalName: row.OriginalName, ContentType: row.ContentType, File: f, Info: info}, nil
}

// removeFile — удаление файла вне транзакции; ошибка только логируется (строка уже
// удалена/не создана — осиротевший файл безопасен, операция пользователя успешна).
func (s *CardService) removeFile(ctx context.Context, key string) {
	removeAttachmentFiles(ctx, s.files, []string{key})
}

func removeAttachmentFiles(ctx context.Context, fs filestore.FileStore, keys []string) {
	if fs == nil {
		return
	}
	// Операция пользователя уже завершена: отмена запроса не должна оставлять файлы.
	ctx = context.WithoutCancel(ctx)
	for _, k := range keys {
		if err := fs.Delete(ctx, k); err != nil {
			slog.ErrorContext(ctx, "attachment file delete failed", "storage_key", k, "err", err)
		}
	}
}
