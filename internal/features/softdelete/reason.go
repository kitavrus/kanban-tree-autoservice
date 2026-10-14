// Package softdelete — мягкое удаление: общие заготовки всех видов записей (валидатор причины,
// код NOT_DELETED, реестр видов) и чтение раздела «Удалённые» (GET /api/v1/deleted, страница
// /deleted). Само удаление и восстановление записи живёт в пакете её вида (заказ — board);
// softdelete их не импортирует — зависимость односторонняя: вид → softdelete.
package softdelete

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// ReasonMax — причина удаления, символов после обрезки крайних пробелов (DS-020).
const ReasonMax = 500

// Коды ошибок полей (details.fields[].message) — общий словарь проекта.
const (
	msgRequired    = "required"
	msgInvalid     = "invalid"
	msgTooLong     = "too_long"
	msgInvalidText = "invalid_text"
)

// ErrNotDeleted — восстановление записи, которая не удалена (уже восстановлена) — DS-024.
// Общий для всех видов записей.
var ErrNotDeleted = errorspkg.New("NOT_DELETED", http.StatusConflict, "Запись уже восстановлена")

// ErrNameTaken — восстановление записи, чьё название за время удаления заняла действующая
// запись (DS-017). Общий для видов с уникальным названием (категории, типы оборудования).
var ErrNameTaken = errorspkg.New("NAME_TAKEN", http.StatusConflict, "Название занято — переименуйте действующую запись")

// DeleteRequest — тело DELETE /api/v1/{ресурс}/:id (DS-028). Указатель: тело без reason и
// reason из пробелов дают один ответ — required.
type DeleteRequest struct {
	Reason *string `json:"reason"`
}

// ValidateReason — причина удаления: 1–500 символов после обрезки крайних пробелов, без NUL
// и битого UTF-8. Возвращает обрезанную причину либо ошибку поля reason (400 VALIDATION).
func ValidateReason(raw *string) (string, []errorspkg.Field) {
	reason := ""
	if raw != nil {
		reason = strings.TrimSpace(*raw)
	}
	switch {
	case reason == "":
		return "", []errorspkg.Field{{Name: "reason", Message: msgRequired}}
	case sqlutil.HasInvalidText(reason):
		return "", []errorspkg.Field{{Name: "reason", Message: msgInvalidText}}
	case utf8.RuneCountInString(reason) > ReasonMax:
		return "", []errorspkg.Field{{Name: "reason", Message: msgTooLong}}
	}
	return reason, nil
}

// ParseDeleteRequest — тело DELETE разбирается как JSON независимо от Content-Type; нечитаемое
// либо пустое тело — то же, что отсутствие причины: 400 VALIDATION reason: required.
func ParseDeleteRequest(body []byte) DeleteRequest {
	var req DeleteRequest
	if body = bytes.TrimSpace(body); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			req = DeleteRequest{}
		}
	}
	return req
}

// ReasonError — текст отказа по полю reason для серверной формы удаления (окно «Удаление»,
// текст под полем «Причина», DS-020); "" — отказ не по причине.
func ReasonError(err error) string {
	var app *errorspkg.AppError
	if !errors.As(err, &app) {
		return ""
	}
	for _, f := range app.Fields {
		if f.Name != "reason" {
			continue
		}
		switch f.Message {
		case msgRequired:
			return "Укажите причину удаления"
		case msgTooLong:
			return "Причина — не длиннее 500 символов"
		case msgInvalidText:
			return "Недопустимые символы в причине"
		}
		return "Недопустимое значение"
	}
	return ""
}

// ToastDeleted — тост после удаления записи серверной формой (PRG-переход, DS-033).
const ToastDeleted = "Удалено. Восстановить можно в разделе «Удалённые»"
