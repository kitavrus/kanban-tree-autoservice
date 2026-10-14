package board

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// attachmentService — узкий интерфейс AttachmentsHandler (эталон cardService).
type attachmentService interface {
	UploadAttachment(ctx context.Context, a Actor, taskID, name string, body io.Reader) (*OrderCardMutationResponse, error)
	DeleteAttachment(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
	OpenAttachment(ctx context.Context, a Actor, id string) (*AttachmentFile, error)
}

// AttachmentsHandler — вложения заказа (фаза 7, D21). D19 — доступно и executor.
type AttachmentsHandler struct {
	svc attachmentService
}

func NewAttachmentsHandler(svc attachmentService) *AttachmentsHandler {
	return &AttachmentsHandler{svc: svc}
}

// IsAttachmentUpload — предикат роута загрузки для middleware.BodyLimit (лимит тела
// AttachmentUploadMaxBody вместо общего 4 МБ, тело читается хендлером потоком). Сверка
// без регистра и с хвостовым «/» — как роутер Fiber по умолчанию (CaseSensitive/
// StrictRouting = false). Промах предиката безопасен: запрос получит общий лимит.
func IsAttachmentUpload(c fiber.Ctx) bool {
	if c.Method() != fiber.MethodPost {
		return false
	}
	p := strings.TrimSuffix(strings.ToLower(c.Path()), "/")
	rest, ok := strings.CutPrefix(p, "/api/v1/tasks/")
	if !ok {
		return false
	}
	id, tail, ok := strings.Cut(rest, "/")
	return ok && id != "" && tail == "attachments"
}

// Upload — POST /api/v1/tasks/:id/attachments, multipart/form-data, поле `file`. Тело
// читается потоком (fiber StreamRequestBody): файл пишется в FileStore, в память целиком
// не попадает. Остаток тела после ответа дочитывается (в пределах лимита роута), чтобы
// браузер получил ответ (413/400), а не обрыв соединения посреди отправки.
//
// Лимит роута — инвариант и для chunked-тела (Content-Length неизвестен, middleware его не
// видит): поток читается через capReader, сверх AttachmentUploadMaxBody → 413
// REQUEST_TOO_LARGE. Соединение этого роута всегда закрывается после ответа
// (middleware.BodyLimit) — недочитанный хвост не станет следующим запросом.
func (h *AttachmentsHandler) Upload(c fiber.Ctx) (err error) {
	raw := requestBodyReader(c)
	body := &capReader{r: raw, left: AttachmentUploadMaxBody}
	defer func() {
		if body.exceeded {
			err = errorspkg.ErrRequestTooLarge
			_, _ = io.Copy(io.Discard, io.LimitReader(raw, uploadOverflowDrain))
			return
		}
		drainBody(body)
	}()

	mt, params, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if err != nil || mt != fiber.MIMEMultipartForm || params["boundary"] == "" {
		return errorspkg.BadRequest("Ожидается multipart/form-data с файлом в поле file")
	}
	mr := multipart.NewReader(body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return errorspkg.Validation(errorspkg.Field{Name: "file", Message: "Выберите файл"})
		}
		if err != nil {
			return errorspkg.BadRequest("Некорректное тело запроса")
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		resp, err := h.svc.UploadAttachment(c.Context(), currentActor(c), c.Params("id"), part.FileName(), part)
		_ = part.Close()
		if err != nil {
			return err
		}
		return c.Status(fiber.StatusCreated).JSON(resp)
	}
}

// Delete — DELETE /api/v1/attachments/:id.
func (h *AttachmentsHandler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.DeleteAttachment(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// File — GET /api/v1/attachments/:id/file: Content-Type из БД (не из расширения),
// inline + безопасный filename*, nosniff, приватный кэш; Range (один диапазон) — для
// перемотки видео, семантика http.ServeContent (206/416/Accept-Ranges). Тело отдаётся
// потоком из FileStore (SetBodyStream), без буферизации в памяти.
func (h *AttachmentsHandler) File(c fiber.Ctx) error {
	af, err := h.svc.OpenAttachment(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	size := af.Info.Size

	c.Set(fiber.HeaderContentType, af.ContentType)
	c.Set(fiber.HeaderContentDisposition, contentDisposition(af.OriginalName))
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderCacheControl, "private, max-age=3600")
	c.Set(fiber.HeaderAcceptRanges, "bytes")
	c.Set(fiber.HeaderLastModified, af.Info.ModTime.UTC().Format(http.TimeFormat))

	rng, partial, err := parseRange(c.Get(fiber.HeaderRange), size)
	if err != nil {
		_ = af.File.Close()
		c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes */%d", size))
		return c.Status(fiber.StatusRequestedRangeNotSatisfiable).Send(nil)
	}
	start, length := int64(0), size
	if partial {
		if _, err := af.File.Seek(rng.Start, io.SeekStart); err != nil {
			_ = af.File.Close()
			return err
		}
		start, length = rng.Start, rng.Length
		c.Status(fiber.StatusPartialContent)
		c.Set(fiber.HeaderContentRange, fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, size))
	}
	c.RequestCtx().SetBodyStream(readCloser{Reader: io.LimitReader(af.File, length), Closer: af.File}, int(length))
	return nil
}

// readCloser — fasthttp закрывает поток тела ответа (io.Closer) после отправки.
type readCloser struct {
	io.Reader
	io.Closer
}

// capReader — не более left байт; попытка прочитать больше → errBodyCapExceeded и
// exceeded=true (Upload отвечает 413 независимо от того, на каком шаге разбора это
// всплыло — NextPart, Close части или запись файла).
type capReader struct {
	r        io.Reader
	left     int64
	exceeded bool
}

var errBodyCapExceeded = errors.New("board: upload body exceeds route limit")

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		// проверка «есть ли ещё байт»: ровно лимит — допустимо
		var one [1]byte
		n, err := c.r.Read(one[:])
		if n > 0 {
			c.exceeded = true
			return 0, errBodyCapExceeded
		}
		return 0, err
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

// uploadOverflowDrain — после превышения лимита дочитываем небольшой хвост, чтобы
// клиент, дописывающий тело, получил 413, а не обрыв (соединение всё равно закрывается).
const uploadOverflowDrain = 1 << 20

// requestBodyReader — поток тела (StreamRequestBody) либо уже прочитанное тело.
func requestBodyReader(c fiber.Ctx) io.Reader {
	if req := c.Request(); req.IsBodyStream() {
		if r := req.BodyStream(); r != nil {
			return r
		}
	}
	return bytes.NewReader(c.Body())
}

// drainBody — дочитать остаток тела (не больше лимита роута; больше — отрезает
// middleware.BodyLimit по Content-Length ещё до хендлера).
func drainBody(r io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(r, AttachmentUploadMaxBody))
}
