//go:build integration

package board_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// AttachmentsLiveSuite (§TD-8): вложения заказа (фаза 7, D21) — реальный бинарь + TCP +
// реальный логин; файлы пишутся в UPLOADS_DIR временного каталога сьюты (проверки диска).
type AttachmentsLiveSuite struct {
	testutil.LiveSuite
	uploads string
	admin   *testutil.Browser
	execA   *testutil.Browser
	execB   *testutil.Browser
	execAID string
}

func TestAttachmentsLiveSuite(t *testing.T) {
	dir := t.TempDir()
	suite.Run(t, &AttachmentsLiveSuite{
		uploads: dir,
		// UPLOADS_GC_INTERVAL=1s — сборщик сирот тикает часто (attachments_gc_http_live_test.go);
		// свежие файлы остальных тестов моложе UPLOADS_GC_MIN_AGE (1h) и им не трогаются.
		LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server", ExtraEnv: []string{"UPLOADS_DIR=" + dir, "UPLOADS_GC_INTERVAL=1s", "TEST_PURGE_ENABLED=true"}},
	})
}

func (s *AttachmentsLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("att-admin", "password1", "Вложения Админ", "admin", true)
	s.execAID = s.SeedUser("att-exec-a", "password1", "Исполнитель А", "executor", true)
	s.SeedUser("att-exec-b", "password1", "Исполнитель Б", "executor", true)
	s.admin = s.NewBrowser()
	s.admin.Login("att-admin", "password1")
	s.execA = s.NewBrowser()
	s.execA.Login("att-exec-a", "password1")
	s.execB = s.NewBrowser()
	s.execB.Login("att-exec-b", "password1")
}

// ---- wire-типы (1:1 с card_dto.go CardAttachment, §TD-6.2; карточка — частично) ----

type attachmentDTO struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	UserID       string `json:"user_id"`
	AuthorName   string `json:"author_name"`
	CreatedAt    string `json:"created_at"`
	URL          string `json:"url"`
	CanDelete    bool   `json:"can_delete"`
}

type attCardDTO struct {
	Task struct {
		ID      int64 `json:"id"`
		Version int64 `json:"version"`
	} `json:"task"`
	Attachments []attachmentDTO `json:"attachments"`
}

type attMutationDTO struct {
	Card     attCardDTO        `json:"card"`
	Warnings []json.RawMessage `json:"warnings"`
}

// ---- контент файлов: валидные сигнатуры + добивка до нужного размера ----

func fileOf(sig []byte, size int) []byte {
	b := make([]byte, size)
	copy(b, sig)
	for i := len(sig); i < size; i++ {
		b[i] = byte(i % 251)
	}
	return b
}

var (
	pngSig = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	mp4Sig = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	pdfSig = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
)

// ---- фикстуры/хелперы ----

func (s *AttachmentsLiveSuite) newTask() int64 {
	var clientID, colID string
	ctx := context.Background()
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('Вложения ' || gen_random_uuid()) RETURNING id`).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns WHERE name = 'Новые'`).Scan(&colID))
	st, raw := s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{"title": "Заказ с вложениями", "client_id": clientID, "column_id": colID})
	s.Require().Equal(http.StatusCreated, st, string(raw))
	var m struct {
		Task struct {
			ID int64 `json:"id"`
		} `json:"task"`
	}
	s.Require().NoError(json.Unmarshal(raw, &m))
	return m.Task.ID
}

func multipartBody(field, filename string, content []byte) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if field != "" {
		fw, _ := w.CreateFormFile(field, filename)
		_, _ = fw.Write(content)
	} else {
		_ = w.WriteField("note", "без файла")
	}
	_ = w.Close()
	return buf.Bytes(), w.FormDataContentType()
}

func (s *AttachmentsLiveSuite) uploadTo(b *testutil.Browser, path, filename string, content []byte) (int, attMutationDTO, []byte) {
	body, ct := multipartBody("file", filename, content)
	resp := b.Request(http.MethodPost, path, body, map[string]string{"Content-Type": ct})
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	var m attMutationDTO
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		s.Require().NoError(json.Unmarshal(raw, &m), string(raw))
	}
	return resp.StatusCode, m, raw
}

func (s *AttachmentsLiveSuite) upload(b *testutil.Browser, taskID int64, filename string, content []byte) (int, attMutationDTO, []byte) {
	return s.uploadTo(b, fmt.Sprintf("/api/v1/tasks/%d/attachments", taskID), filename, content)
}

func (s *AttachmentsLiveSuite) mustUpload(b *testutil.Browser, taskID int64, filename string, content []byte) attachmentDTO {
	st, m, raw := s.upload(b, taskID, filename, content)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.Require().NotEmpty(m.Card.Attachments)
	return m.Card.Attachments[len(m.Card.Attachments)-1]
}

func (s *AttachmentsLiveSuite) errCode(raw []byte) string {
	var e apiErrDTO
	s.Require().NoError(json.Unmarshal(raw, &e), string(raw))
	return e.Code
}

func (s *AttachmentsLiveSuite) storageKey(id string) string {
	var key string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT storage_key FROM task_attachments WHERE id = $1`, id).Scan(&key))
	return key
}

func (s *AttachmentsLiveSuite) countRows(taskID int64) int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM task_attachments WHERE task_id = $1`, taskID).Scan(&n))
	return n
}

// diskFiles — все файлы каталога UPLOADS_DIR (включая временные .upload-*).
func (s *AttachmentsLiveSuite) diskFiles() map[string]bool {
	es, err := os.ReadDir(s.uploads)
	s.Require().NoError(err)
	out := map[string]bool{}
	for _, e := range es {
		out[e.Name()] = true
	}
	return out
}

func (s *AttachmentsLiveSuite) get(b *testutil.Browser, path string, headers map[string]string) (*http.Response, []byte) {
	resp := b.Request(http.MethodGet, path, nil, headers)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp, raw
}

// rawRequest — запрос «руками» по TCP: заголовки с заявленным Content-Length без отправки
// всего тела (проверка раннего 413 без чтения тела). fasthttp в режиме StreamRequestBody
// ждёт первые 8 КБ тела (prefetch) до вызова хендлеров — префикс ровно 8 КБ (непрочитанного
// сервером остатка в сокете нет — закрытие соединения без RST).
func (s *AttachmentsLiveSuite) rawRequest(method, path string, contentLength int64, bodyPrefix []byte) (int, string) {
	u, err := url.Parse(s.BaseURL)
	s.Require().NoError(err)
	conn, err := net.Dial("tcp", u.Host)
	s.Require().NoError(err)
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n", method, path, u.Host, contentLength)
	s.Require().NoError(err)
	prefix := make([]byte, 8<<10)
	copy(prefix, bodyPrefix)
	_, _ = conn.Write(prefix)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	s.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// ---- Happy Path ----

// LV-201: загрузка фото/видео/PDF → 201 {card, warnings}; card.attachments с видом/типом/
// размером/автором/url; файл на диске под uuid-ключом, не под именем пользователя.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_Upload_AllKinds() {
	taskID := s.newTask()

	st, raw := s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st)
	s.Contains(string(raw), `"attachments":[]`, "пустой заказ — массив, не null")

	photo := fileOf(pngSig, 3000)
	st, m, raw := s.upload(s.admin, taskID, "фото 1.png", photo)
	s.Require().Equal(http.StatusCreated, st, string(raw))
	s.NotNil(m.Warnings)
	s.Require().Len(m.Card.Attachments, 1)
	a := m.Card.Attachments[0]
	s.Equal("photo", a.Kind)
	s.Equal("image/png", a.ContentType)
	s.Equal("фото 1.png", a.OriginalName)
	s.EqualValues(3000, a.Size)
	s.Equal("Вложения Админ", a.AuthorName)
	s.True(a.CanDelete)
	s.Equal("/api/v1/attachments/"+a.ID+"/file", a.URL)

	key := s.storageKey(a.ID)
	s.NoError(uuid.Validate(key), "ключ хранения — uuid")
	onDisk, err := os.ReadFile(filepath.Join(s.uploads, key))
	s.Require().NoError(err)
	s.Equal(photo, onDisk)
	s.False(s.diskFiles()["фото 1.png"])

	v := s.mustUpload(s.admin, taskID, "звук.mp4", fileOf(mp4Sig, 2048))
	s.Equal("video", v.Kind)
	s.Equal("video/mp4", v.ContentType)
	p := s.mustUpload(s.admin, taskID, "акт.pdf", fileOf(pdfSig, 1024))
	s.Equal("pdf", p.Kind)
	s.Equal("application/pdf", p.ContentType)

	// Путь в имени — только базовое имя; расширение не влияет на тип.
	odd := s.mustUpload(s.admin, taskID, "../../etc/passwd.txt", fileOf(pngSig, 100))
	s.Equal("passwd.txt", odd.OriginalName)
	s.Equal("image/png", odd.ContentType)

	st, raw = s.admin.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st)
	var card attCardDTO
	s.Require().NoError(json.Unmarshal(raw, &card))
	s.Require().Len(card.Attachments, 4)
	s.Equal([]string{"photo", "video", "pdf", "photo"}, []string{card.Attachments[0].Kind, card.Attachments[1].Kind, card.Attachments[2].Kind, card.Attachments[3].Kind})
}

// LV-202: отдача — Content-Type из БД, inline + filename*, nosniff, Cache-Control private.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_File_Headers() {
	taskID := s.newTask()
	content := fileOf(pdfSig, 5000)
	a := s.mustUpload(s.admin, taskID, `отчёт "итог".pdf`, content)

	resp, body := s.get(s.execB, a.URL, nil) // исполнитель видит вложения заказа (D19)
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	s.Equal(content, body)
	s.Equal("application/pdf", resp.Header.Get("Content-Type"))
	s.Equal("nosniff", resp.Header.Get("X-Content-Type-Options"))
	s.Contains(resp.Header.Get("Cache-Control"), "private")
	s.Equal("bytes", resp.Header.Get("Accept-Ranges"))
	s.Equal("5000", resp.Header.Get("Content-Length"))
	cd := resp.Header.Get("Content-Disposition")
	s.True(strings.HasPrefix(cd, `inline; filename="`), cd)
	s.Contains(cd, "filename*=UTF-8''%D0%BE%D1%82%D1%87%D1%91%D1%82%20%22")
	s.NotContains(cd, `"итог"`)
}

// LV-203: Range — 206 + Content-Range; неудовлетворимый — 416 bytes */size.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_File_Range() {
	taskID := s.newTask()
	content := fileOf(mp4Sig, 1000)
	a := s.mustUpload(s.execA, taskID, "clip.mp4", content)

	resp, body := s.get(s.admin, a.URL, map[string]string{"Range": "bytes=10-19"})
	s.Require().Equal(http.StatusPartialContent, resp.StatusCode)
	s.Equal("bytes 10-19/1000", resp.Header.Get("Content-Range"))
	s.Equal(content[10:20], body)
	s.Equal("video/mp4", resp.Header.Get("Content-Type"))

	resp, body = s.get(s.admin, a.URL, map[string]string{"Range": "bytes=-5"})
	s.Require().Equal(http.StatusPartialContent, resp.StatusCode)
	s.Equal(content[995:], body)

	resp, _ = s.get(s.admin, a.URL, map[string]string{"Range": "bytes=5000-"})
	s.Equal(http.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	s.Equal("bytes */1000", resp.Header.Get("Content-Range"))
}

// ---- Validation / MIME / Limits ----

// LV-204: MIME-подмена — HTML/SVG под .jpg/.png → 400 ATTACHMENT_TYPE_UNSUPPORTED; ни
// строки, ни файла.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_MIMESpoof400() {
	taskID := s.newTask()
	before := s.diskFiles()
	for name, content := range map[string][]byte{
		"evil.jpg": []byte("<!DOCTYPE html><html><body><script>alert(1)</script></body></html>"),
		"evil.png": []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"a.pdf":    []byte("PK\x03\x04 zip под видом pdf"),
	} {
		st, _, raw := s.upload(s.admin, taskID, name, content)
		s.Equal(http.StatusBadRequest, st, name)
		s.Equal("ATTACHMENT_TYPE_UNSUPPORTED", s.errCode(raw), name)
	}
	s.Equal(0, s.countRows(taskID))
	s.Equal(before, s.diskFiles())
}

// LV-205: лимит фото 20 МБ — ровно лимит принимается, +1 байт → 413 ATTACHMENT_TOO_LARGE;
// частичный файл не остаётся. Лимит PDF — то же значение.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_PhotoLimit413() {
	taskID := s.newTask()
	before := s.diskFiles()
	st, _, raw := s.upload(s.admin, taskID, "big.png", fileOf(pngSig, 20<<20+1))
	s.Require().Equal(http.StatusRequestEntityTooLarge, st, string(raw))
	s.Equal("ATTACHMENT_TOO_LARGE", s.errCode(raw))
	s.Contains(string(raw), "20 МБ")
	s.Equal(0, s.countRows(taskID))
	s.Equal(before, s.diskFiles())

	st, _, raw = s.upload(s.admin, taskID, "big.pdf", fileOf(pdfSig, 20<<20+1))
	s.Equal(http.StatusRequestEntityTooLarge, st, string(raw))
	s.Equal("ATTACHMENT_TOO_LARGE", s.errCode(raw))

	a := s.mustUpload(s.admin, taskID, "edge.png", fileOf(pngSig, 20<<20))
	s.EqualValues(20<<20, a.Size)
}

// LV-206: лимиты тела по роутам — upload: Content-Length > 501 МБ → 413 без чтения тела;
// прочие роуты — прежние 4 МБ (Content-Length и chunked); тело > 8 КБ (поток) по-прежнему
// читается обычными хендлерами.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_BodyLimits() {
	taskID := s.newTask()

	st, raw := s.rawRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/attachments", taskID), 600<<20, []byte("--x\r\n"))
	s.Equal(http.StatusRequestEntityTooLarge, st, raw)
	s.Contains(raw, "REQUEST_TOO_LARGE")

	st, raw = s.rawRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", taskID), 5<<20, []byte("{"))
	s.Equal(http.StatusRequestEntityTooLarge, st, raw)

	// chunked (длина неизвестна) > 4 МБ на обычном роуте → 413. Превышение небольшое: хвост
	// сервер дочитывает (overflowDrain), клиент успевает дописать тело и получить ответ.
	big := io.MultiReader(strings.NewReader(`{"title":"`), strings.NewReader(strings.Repeat("a", 4<<20+1024)), strings.NewReader(`"}`))
	req, err := http.NewRequest(http.MethodPost, s.BaseURL+"/api/v1/tasks", big)
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Csrf-Token", s.admin.CSRFToken())
	resp, err := s.admin.Client.Do(req)
	s.Require().NoError(err)
	_ = resp.Body.Close()
	s.Equal(http.StatusRequestEntityTooLarge, resp.StatusCode)

	// > 8 КБ (тело приходит потоком) — обычный JSON-хендлер работает как раньше
	var clientID, colID string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `INSERT INTO clients (company_name) VALUES ('Длинное описание') RETURNING id`).Scan(&clientID))
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT id FROM columns WHERE name = 'Новые'`).Scan(&colID))
	st2, raw2 := s.admin.Do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title": "Длинный", "client_id": clientID, "column_id": colID, "description": strings.Repeat("я", 9000),
	})
	s.Equal(http.StatusCreated, st2, string(raw2))
}

// LV-209: битые входы — нет файла/не multipart/пустой файл → 400, невалидные id → 404.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_InvalidInput() {
	taskID := s.newTask()
	path := fmt.Sprintf("/api/v1/tasks/%d/attachments", taskID)

	body, ct := multipartBody("", "", nil)
	resp := s.admin.Request(http.MethodPost, path, body, map[string]string{"Content-Type": ct})
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Equal(http.StatusBadRequest, resp.StatusCode, string(raw))
	s.Equal("VALIDATION", s.errCode(raw))
	s.Contains(string(raw), `"file"`)

	st, raw := s.admin.Do(http.MethodPost, path, map[string]string{"file": "x"})
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("BAD_REQUEST", s.errCode(raw))

	st, _, raw = s.upload(s.admin, taskID, "empty.png", nil)
	s.Equal(http.StatusBadRequest, st, string(raw))
	s.Equal("VALIDATION", s.errCode(raw))

	for _, p := range []string{"/api/v1/tasks/abc/attachments", "/api/v1/tasks/0/attachments", "/api/v1/tasks/999999999/attachments"} {
		st, _, raw = s.uploadTo(s.admin, p, "a.png", fileOf(pngSig, 64))
		s.Equal(http.StatusNotFound, st, p)
		s.Equal("TASK_NOT_FOUND", s.errCode(raw), p)
	}
	for _, id := range []string{"not-a-uuid", uuid.NewString()} {
		st, raw = s.admin.Do(http.MethodGet, "/api/v1/attachments/"+id+"/file", nil)
		s.Equal(http.StatusNotFound, st, id)
		s.Equal("ATTACHMENT_NOT_FOUND", s.errCode(raw))
		st, raw = s.admin.Do(http.MethodDelete, "/api/v1/attachments/"+id, nil)
		s.Equal(http.StatusNotFound, st, id)
		s.Equal("ATTACHMENT_NOT_FOUND", s.errCode(raw))
	}
	s.Equal(0, s.countRows(taskID))
}

// ---- Access Control ----

// LV-207: удаление — чужой executor 403 ATTACHMENT_FORBIDDEN (can_delete=false), автор —
// 200 + файл удалён с диска, повтор — 404; admin удаляет чужое.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_Delete_Permissions() {
	taskID := s.newTask()
	a := s.mustUpload(s.execA, taskID, "a.png", fileOf(pngSig, 200))
	s.Equal(s.execAID, a.UserID)
	b := s.mustUpload(s.execB, taskID, "b.png", fileOf(pngSig, 200))
	keyA := s.storageKey(a.ID)

	st, raw := s.execB.Do(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d/card", taskID), nil)
	s.Require().Equal(http.StatusOK, st)
	var card attCardDTO
	s.Require().NoError(json.Unmarshal(raw, &card))
	s.Require().Len(card.Attachments, 2)
	s.False(card.Attachments[0].CanDelete, "чужое вложение — без права удаления")
	s.True(card.Attachments[1].CanDelete)

	st, raw = s.execB.Do(http.MethodDelete, "/api/v1/attachments/"+a.ID, nil)
	s.Equal(http.StatusForbidden, st, string(raw))
	s.Equal("ATTACHMENT_FORBIDDEN", s.errCode(raw))
	s.True(s.diskFiles()[keyA])

	st, raw = s.execA.Do(http.MethodDelete, "/api/v1/attachments/"+a.ID, nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	var m attMutationDTO
	s.Require().NoError(json.Unmarshal(raw, &m))
	s.Require().Len(m.Card.Attachments, 1)
	s.Equal(b.ID, m.Card.Attachments[0].ID)
	s.False(s.diskFiles()[keyA], "файл удалён с диска")

	st, raw = s.execA.Do(http.MethodDelete, "/api/v1/attachments/"+a.ID, nil)
	s.Equal(http.StatusNotFound, st)
	s.Equal("ATTACHMENT_NOT_FOUND", s.errCode(raw))
	st, _ = s.execA.Do(http.MethodGet, a.URL, nil)
	s.Equal(http.StatusNotFound, st)

	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/attachments/"+b.ID, nil)
	s.Equal(http.StatusOK, st, string(raw))
	s.Equal(0, s.countRows(taskID))
}

// LV-208: без сессии — 401 на все роуты вложений; без CSRF-заголовка — 403 CSRF_INVALID.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_NoSession401_NoCSRF403() {
	taskID := s.newTask()
	a := s.mustUpload(s.admin, taskID, "a.png", fileOf(pngSig, 100))

	anon := s.NewBrowser()
	st, _, raw := s.upload(anon, taskID, "x.png", fileOf(pngSig, 100))
	s.Equal(http.StatusUnauthorized, st, string(raw))
	st, _ = anon.Do(http.MethodGet, a.URL, nil)
	s.Equal(http.StatusUnauthorized, st)
	st, _ = anon.Do(http.MethodDelete, "/api/v1/attachments/"+a.ID, nil)
	s.Equal(http.StatusUnauthorized, st)

	body, ct := multipartBody("file", "x.png", fileOf(pngSig, 100))
	resp := s.admin.Request(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/attachments", taskID), body,
		map[string]string{"Content-Type": ct, "X-Csrf-Token": ""})
	raw, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Equal(http.StatusForbidden, resp.StatusCode, string(raw))
	s.Equal("CSRF_INVALID", s.errCode(raw))
	s.Equal(1, s.countRows(taskID))
}

// ---- Cascade ----

// LV-210 → soft-delete-core TS-008 (DS-013): мягкое удаление заказа — строки вложений и файлы
// на диске остаются; файл по прямой ссылке не отдаётся и поштучно не удаляется (404), загрузка
// в удалённый заказ — 404 TASK_NOT_FOUND; после восстановления файл отдаётся снова.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_TaskSoftDelete_KeepsFiles() {
	taskID := s.newTask()
	a := s.mustUpload(s.admin, taskID, "a.png", fileOf(pngSig, 100))
	b := s.mustUpload(s.execA, taskID, "b.pdf", fileOf(pdfSig, 100))
	keys := []string{s.storageKey(a.ID), s.storageKey(b.ID)}

	st, raw := s.admin.Do(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", taskID), map[string]any{"reason": "Создан по ошибке"})
	s.Require().Equal(http.StatusOK, st, string(raw))
	s.Equal(2, s.countRows(taskID), "строки вложений удалённого заказа на месте")
	files := s.diskFiles()
	for _, k := range keys {
		s.True(files[k], "файл удалённого заказа на диске: %s", k)
	}
	for _, br := range []*testutil.Browser{s.admin, s.execA} {
		st, raw = br.Do(http.MethodGet, a.URL, nil)
		s.Equal(http.StatusNotFound, st, string(raw))
		s.Equal("ATTACHMENT_NOT_FOUND", s.errCode(raw))
	}
	st, raw = s.admin.Do(http.MethodDelete, "/api/v1/attachments/"+a.ID, nil)
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("ATTACHMENT_NOT_FOUND", s.errCode(raw))
	st, _, raw = s.upload(s.admin, taskID, "c.png", fileOf(pngSig, 100))
	s.Equal(http.StatusNotFound, st, string(raw))
	s.Equal("TASK_NOT_FOUND", s.errCode(raw))
	s.Equal(2, s.countRows(taskID), "отказы ничего не изменили")

	// после восстановления файл отдаётся.
	st, raw = s.admin.Do(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%d/restore", taskID), nil)
	s.Require().Equal(http.StatusOK, st, string(raw))
	st, _ = s.execA.Do(http.MethodGet, a.URL, nil)
	s.Equal(http.StatusOK, st)
	st, _ = s.execA.Do(http.MethodGet, b.URL, nil)
	s.Equal(http.StatusOK, st)
}

// ---- Request desync (code review фазы 7, HIGH) ----

// pipelined — сырой TCP: запрос с Content-Length, в теле которого после 8 КБ паддинга лежит
// второй («контрабандный») HTTP-запрос. Возвращает первый ответ и получен ли второй ответ
// на том же соединении (его быть не должно: тело либо дочитано целиком, либо соединение
// закрыто).
func (s *AttachmentsLiveSuite) pipelined(method, path, headers string) (*http.Response, bool) {
	u, err := url.Parse(s.BaseURL)
	s.Require().NoError(err)
	conn, err := net.Dial("tcp", u.Host)
	s.Require().NoError(err)
	defer conn.Close()

	smuggled := "GET /health HTTP/1.1\r\nHost: " + u.Host + "\r\n\r\n"
	body := strings.Repeat("x", 8<<10) + smuggled
	_, err = fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=zzz\r\n%sContent-Length: %d\r\n\r\n%s",
		method, path, u.Host, headers, len(body), body)
	s.Require().NoError(err)

	s.Require().NoError(conn.SetReadDeadline(time.Now().Add(3 * time.Second)))
	br := bufio.NewReader(conn)
	first, err := http.ReadResponse(br, nil)
	s.Require().NoError(err)
	_, _ = io.Copy(io.Discard, first.Body)
	_ = first.Body.Close()

	s.Require().NoError(conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)))
	second, err := http.ReadResponse(br, nil)
	if err != nil {
		return first, false
	}
	_ = second.Body.Close()
	return first, true
}

// LV-211: отказ ДО хендлера на роуте загрузки (403 CSRF, 401 без сессии) — соединение
// закрывается, недочитанный хвост тела не исполняется как следующий запрос.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_NoRequestDesync() {
	taskID := s.newTask()
	path := fmt.Sprintf("/api/v1/tasks/%d/attachments", taskID)

	// 403: без CSRF (аноним)
	first, smuggled := s.pipelined(http.MethodPost, path, "")
	s.Equal(http.StatusForbidden, first.StatusCode)
	s.True(first.Close, "Connection: close на роуте загрузки")
	s.False(smuggled, "контрабандный запрос из тела не обработан")

	// 401: валидный CSRF, но без сессии
	anon := s.NewBrowser()
	tok := anon.CSRFToken()
	first, smuggled = s.pipelined(http.MethodPost, path, "Cookie: kt_csrf="+tok+"\r\nX-Csrf-Token: "+tok+"\r\n")
	s.Equal(http.StatusUnauthorized, first.StatusCode)
	s.True(first.Close)
	s.False(smuggled)

	// обычный API-роут: тело читается целиком до CSRF — «запрос» внутри тела остаётся телом
	first, smuggled = s.pipelined(http.MethodPost, "/api/v1/tasks", "")
	s.Equal(http.StatusForbidden, first.StatusCode)
	s.False(smuggled)
}

// zeroReader — бесконечный поток байт 'a' (без аллокации всего тела в тесте).
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// LV-212: chunked-тело загрузки (Content-Length неизвестен) без поля file больше лимита
// роута → 413 REQUEST_TOO_LARGE, а не бесконечный приём.
func (s *AttachmentsLiveSuite) TestAttachmentsLiveSuite_ChunkedBodyCapped413() {
	taskID := s.newTask()
	const boundary = "capboundary"
	head := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"note\"\r\n\r\n"
	tail := "\r\n--" + boundary + "--\r\n"
	filler := int64(board.AttachmentUploadMaxBody) + 100<<10
	body := io.MultiReader(strings.NewReader(head), io.LimitReader(zeroReader{}, filler), strings.NewReader(tail))

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/tasks/%d/attachments", s.BaseURL, taskID), body)
	s.Require().NoError(err)
	req.ContentLength = -1 // длина неизвестна → Transfer-Encoding: chunked
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("X-Csrf-Token", s.admin.CSRFToken())
	resp, err := s.admin.Client.Do(req)
	s.Require().NoError(err)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	s.Equal(http.StatusRequestEntityTooLarge, resp.StatusCode, string(raw))
	s.Equal("REQUEST_TOO_LARGE", s.errCode(raw))
	s.Equal(0, s.countRows(taskID))
}
