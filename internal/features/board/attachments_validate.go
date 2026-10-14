package board

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Вложения заказа (фаза 7, D21): тип определяется ТОЛЬКО по содержимому (сигнатуре), не по
// расширению/заголовку клиента; лимиты — по виду.
const (
	AttachmentKindPhoto = "photo"
	AttachmentKindVideo = "video"
	AttachmentKindPDF   = "pdf"

	mb = int64(1) << 20
	// AttachmentMaxPhoto/Video/PDF — лимиты размера файла по виду (D21).
	AttachmentMaxPhoto = 20 * mb
	AttachmentMaxVideo = 500 * mb
	AttachmentMaxPDF   = 20 * mb
	// AttachmentUploadMaxBody — лимит тела multipart-запроса загрузки: максимальный файл +
	// запас на заголовки частей/boundary. Прочие роуты — прежний 4 МБ (middleware.BodyLimit).
	AttachmentUploadMaxBody = AttachmentMaxVideo + mb

	// attachmentSniffLen — сколько байт начала файла читает детектор (как http.DetectContentType).
	attachmentSniffLen = 512
	attachmentNameMax  = 255
)

// attachmentLimit — лимит размера по виду.
func attachmentLimit(kind string) int64 {
	switch kind {
	case AttachmentKindVideo:
		return AttachmentMaxVideo
	case AttachmentKindPDF:
		return AttachmentMaxPDF
	default:
		return AttachmentMaxPhoto
	}
}

// attachmentLimitMessage — понятный текст 413 по виду файла.
func attachmentLimitMessage(kind string) string {
	switch kind {
	case AttachmentKindVideo:
		return "Видео больше 500 МБ — загрузить нельзя"
	case AttachmentKindPDF:
		return "PDF больше 20 МБ — загрузить нельзя"
	default:
		return "Фото больше 20 МБ — загрузить нельзя"
	}
}

// sniffAttachment — вид и Content-Type по первым байтам файла. ok=false — тип не
// поддерживается (HTML/SVG/архивы/исполняемые/HEIC и т.п. → 400). SVG намеренно не фото:
// это XML с возможным скриптом.
func sniffAttachment(head []byte) (kind, contentType string, ok bool) {
	if ct, isQT := sniffQuickTime(head); isQT {
		return AttachmentKindVideo, ct, true
	}
	ct := http.DetectContentType(head)
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return AttachmentKindPhoto, ct, true
	case "video/mp4", "video/webm":
		return AttachmentKindVideo, ct, true
	case "application/pdf":
		return AttachmentKindPDF, ct, true
	}
	return "", "", false
}

// sniffQuickTime — ISO-BMFF `ftyp` с major brand "qt  " (MOV с iPhone): стандартный
// http.DetectContentType его не распознаёт (узнаёт только бренды mp4*).
func sniffQuickTime(head []byte) (string, bool) {
	if len(head) < 12 || !bytes.Equal(head[4:8], []byte("ftyp")) {
		return "", false
	}
	if bytes.Equal(head[8:12], []byte("qt  ")) {
		return "video/quicktime", true
	}
	return "", false
}

// sanitizeAttachmentName — отображаемое имя файла: только базовое имя (без каталогов),
// без управляющих и format-символов (Cf, в т.ч. bidi), валидный UTF-8, ≤255 рун; пустое → "file". В путь на диске
// имя не попадает никогда (ключ хранения — uuid).
func sanitizeAttachmentName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToValidUTF8(name, "")
	name = strings.Map(func(r rune) rune {
		// Cf — bidi/format-символы (U+202E и др.): подмена отображаемого имени
		// («photo\u202Egpj.exe»), в имени файла не нужны.
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > attachmentNameMax {
		name = string([]rune(name)[:attachmentNameMax])
	}
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

// contentDisposition — inline + filename (ASCII) + filename* (UTF-8, RFC 6266/
// 5987): ASCII-фолбэк без кавычек/обратных слэшей/не-ASCII, точное имя — в filename*.
func contentDisposition(name string) string {
	var ascii strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '%' || r == ';' {
			ascii.WriteByte('_')
			continue
		}
		ascii.WriteRune(r)
	}
	return fmt.Sprintf(`inline; filename="%s"; filename*=UTF-8''%s`, ascii.String(), rfc5987Encode(name))
}

// rfc5987Encode — attr-char остаются как есть, прочие байты UTF-8 — %XX.
func rfc5987Encode(s string) string {
	const attrChars = "!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.IndexByte(attrChars, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// byteRange — один диапазон [Start, Start+Length).
type byteRange struct {
	Start, Length int64
}

// parseRange — заголовок Range по семантике http.ServeContent для ОДНОГО диапазона:
// ok=false, err=nil — Range нет/не bytes/несколько диапазонов → отдаём файл целиком (RFC
// 7233 разрешает игнорировать Range); err != nil — синтаксически битый либо
// неудовлетворимый диапазон → 416.
func parseRange(h string, size int64) (byteRange, bool, error) {
	if h == "" {
		return byteRange{}, false, nil
	}
	const prefix = "bytes="
	if !strings.HasPrefix(h, prefix) {
		return byteRange{}, false, nil
	}
	spec := strings.TrimSpace(h[len(prefix):])
	if strings.Contains(spec, ",") {
		return byteRange{}, false, nil
	}
	startS, endS, found := strings.Cut(spec, "-")
	if !found {
		return byteRange{}, false, errInvalidRange
	}
	startS, endS = strings.TrimSpace(startS), strings.TrimSpace(endS)
	if startS == "" {
		// суффикс: последние N байт
		n, err := strconv.ParseInt(endS, 10, 64)
		if err != nil || n <= 0 || size == 0 {
			return byteRange{}, false, errInvalidRange
		}
		if n > size {
			n = size
		}
		return byteRange{Start: size - n, Length: n}, true, nil
	}
	start, err := strconv.ParseInt(startS, 10, 64)
	if err != nil || start < 0 || start >= size {
		return byteRange{}, false, errInvalidRange
	}
	end := size - 1
	if endS != "" {
		e, err := strconv.ParseInt(endS, 10, 64)
		if err != nil || e < start {
			return byteRange{}, false, errInvalidRange
		}
		if e < end {
			end = e
		}
	}
	return byteRange{Start: start, Length: end - start + 1}, true, nil
}

var errInvalidRange = errors.New("board: invalid range")
