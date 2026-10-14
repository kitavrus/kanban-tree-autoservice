package board

import (
	"io"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Сигнатуры тест-файлов (минимальные валидные заголовки форматов).
var (
	sigPNG  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	sigJPEG = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	sigGIF  = []byte("GIF89a\x01\x00\x01\x00")
	sigWEBP = []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	sigMP4  = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")
	sigMOV  = []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00qt  ")
	sigWEBM = []byte("\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01")
	sigPDF  = []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
)

func TestSniffAttachment_Supported(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		kind string
		ct   string
	}{
		{"png", sigPNG, AttachmentKindPhoto, "image/png"},
		{"jpeg", sigJPEG, AttachmentKindPhoto, "image/jpeg"},
		{"gif", sigGIF, AttachmentKindPhoto, "image/gif"},
		{"webp", sigWEBP, AttachmentKindPhoto, "image/webp"},
		{"mp4", sigMP4, AttachmentKindVideo, "video/mp4"},
		{"mov", sigMOV, AttachmentKindVideo, "video/quicktime"},
		{"webm", sigWEBM, AttachmentKindVideo, "video/webm"},
		{"pdf", sigPDF, AttachmentKindPDF, "application/pdf"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kind, ct, ok := sniffAttachment(c.head)
			require.True(t, ok)
			assert.Equal(t, c.kind, kind)
			assert.Equal(t, c.ct, ct)
		})
	}
}

func TestSniffAttachment_Rejected(t *testing.T) {
	for name, head := range map[string][]byte{
		"html":  []byte("<!DOCTYPE html><html><script>alert(1)</script>"),
		"svg":   []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`),
		"zip":   []byte("PK\x03\x04\x14\x00\x00\x00"),
		"exe":   []byte("MZ\x90\x00\x03\x00\x00\x00"),
		"text":  []byte("просто текст, переименованный в .jpg"),
		"empty": {},
		"heic":  []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"),
	} {
		_, _, ok := sniffAttachment(head)
		assert.False(t, ok, name)
	}
}

func TestAttachmentLimit(t *testing.T) {
	assert.EqualValues(t, 20<<20, attachmentLimit(AttachmentKindPhoto))
	assert.EqualValues(t, 500<<20, attachmentLimit(AttachmentKindVideo))
	assert.EqualValues(t, 20<<20, attachmentLimit(AttachmentKindPDF))
	assert.Greater(t, AttachmentUploadMaxBody, AttachmentMaxVideo)
	for _, k := range []string{AttachmentKindPhoto, AttachmentKindVideo, AttachmentKindPDF} {
		assert.NotEmpty(t, attachmentLimitMessage(k))
	}
}

func TestSanitizeAttachmentName(t *testing.T) {
	assert.Equal(t, "фото.jpg", sanitizeAttachmentName("фото.jpg"))
	assert.Equal(t, "passwd", sanitizeAttachmentName("../../etc/passwd"))
	assert.Equal(t, "evil.png", sanitizeAttachmentName(`C:\Users\x\evil.png`))
	assert.Equal(t, "ab.png", sanitizeAttachmentName("a\x00\r\nb.png"))
	assert.Equal(t, "file", sanitizeAttachmentName("   "))
	assert.Equal(t, "file", sanitizeAttachmentName(".."))
	assert.Equal(t, "ok", sanitizeAttachmentName("o\xffk"))
	assert.Equal(t, "photogpj.exe", sanitizeAttachmentName("photo\u202Egpj.exe"), "RLO (bidi) убран")
	assert.Equal(t, "ab.png", sanitizeAttachmentName("a\u2066\u200Bb\uFEFF.png"), "isolate/zero-width/BOM убраны")
	long := sanitizeAttachmentName(strings.Repeat("я", 300))
	assert.Equal(t, 255, len([]rune(long)))
}

func TestContentDisposition(t *testing.T) {
	assert.Equal(t, `inline; filename="a.pdf"; filename*=UTF-8''a.pdf`, contentDisposition("a.pdf"))
	got := contentDisposition(`отчёт "1";.pdf`)
	assert.True(t, strings.HasPrefix(got, `inline; filename="_____ _1__.pdf"; filename*=UTF-8''`), got)
	assert.Contains(t, got, "%D0%BE%D1%82%D1%87%D1%91%D1%82%20%221%22%3B.pdf")
	assert.NotContains(t, got, "\n")
}

func TestParseRange(t *testing.T) {
	const size = 100
	cases := []struct {
		h      string
		ok     bool
		bad    bool
		start  int64
		length int64
	}{
		{h: ""},
		{h: "items=0-1"},
		{h: "bytes=0-1,5-6"}, // multi — целиком
		{h: "bytes=0-9", ok: true, start: 0, length: 10},
		{h: "bytes=90-", ok: true, start: 90, length: 10},
		{h: "bytes=95-1000", ok: true, start: 95, length: 5},
		{h: "bytes=-10", ok: true, start: 90, length: 10},
		{h: "bytes=-1000", ok: true, start: 0, length: 100},
		{h: "bytes=100-", bad: true},
		{h: "bytes=5-2", bad: true},
		{h: "bytes=abc", bad: true},
		{h: "bytes=-0", bad: true},
		{h: "bytes=x-5", bad: true},
	}
	for _, c := range cases {
		r, ok, err := parseRange(c.h, size)
		if c.bad {
			assert.ErrorIs(t, err, errInvalidRange, c.h)
			continue
		}
		require.NoError(t, err, c.h)
		assert.Equal(t, c.ok, ok, c.h)
		if c.ok {
			assert.Equal(t, byteRange{Start: c.start, Length: c.length}, r, c.h)
		}
	}
}

func TestIsAttachmentUpload(t *testing.T) {
	app := fiber.New()
	cases := []struct {
		method, path string
		want         bool
	}{
		{fiber.MethodPost, "/api/v1/tasks/12/attachments", true},
		{fiber.MethodPost, "/api/v1/tasks/12/attachments/", true},
		{fiber.MethodPost, "/API/V1/Tasks/12/Attachments", true},
		{fiber.MethodGet, "/api/v1/tasks/12/attachments", false},
		{fiber.MethodPost, "/api/v1/tasks/12", false},
		{fiber.MethodPost, "/api/v1/tasks//attachments", false},
		{fiber.MethodPost, "/api/v1/tasks/12/equipment", false},
		{fiber.MethodPost, "/api/v1/tasks/12/attachments/x", false},
		{fiber.MethodPost, "/api/v1/attachments", false},
	}
	for _, c := range cases {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(c.method)
		fctx.Request.SetRequestURI(c.path)
		ctx := app.AcquireCtx(fctx)
		assert.Equal(t, c.want, IsAttachmentUpload(ctx), c.method+" "+c.path)
		app.ReleaseCtx(ctx)
	}
}

func TestCapReader(t *testing.T) {
	exact := &capReader{r: strings.NewReader("12345"), left: 5}
	b, err := io.ReadAll(exact)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(b))
	assert.False(t, exact.exceeded, "ровно лимит — допустимо")

	over := &capReader{r: strings.NewReader("123456"), left: 5}
	b, err = io.ReadAll(over)
	assert.ErrorIs(t, err, errBodyCapExceeded)
	assert.Equal(t, "12345", string(b))
	assert.True(t, over.exceeded)
}
