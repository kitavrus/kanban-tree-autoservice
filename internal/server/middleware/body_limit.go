package middleware

import (
	"io"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// BodyLimit — лимит тела по роутам при fiber.Config{StreamRequestBody: true} (фаза 7):
// fasthttp больше не буферизует тело целиком до хендлера, поэтому лимит «как было»
// (defaultLimit, для всех роутов) держит этот middleware — тело читается в память не
// более defaultLimit байт и подкладывается обратно (c.Body()/Bind работают как раньше).
// Роуты, для которых large(c) == true (загрузка вложений), тело НЕ читают здесь — только
// ранний отказ по Content-Length > largeLimit; дальше хендлер читает поток сам.
// Превышение → 413 REQUEST_TOO_LARGE + Connection: close (непрочитанный остаток тела на
// соединении не остаётся).
//
// Инвариант против request desync (CL.0, code review фазы 7): в режиме StreamRequestBody
// fasthttp после ответа НЕ дочитывает недочитанное тело — остаток на keep-alive был бы
// разобран как СЛЕДУЮЩИЙ запрос. Поэтому любой ответ либо идёт после полного чтения тела
// (обычные роуты: тело целиком читается здесь, до CSRF/auth), либо закрывает соединение
// (роут загрузки — Connection: close безусловно, ДО CSRF/auth: их 403/401 уходят раньше
// хендлера, который дочитал бы поток; отказы этого middleware — тоже close).
func BodyLimit(defaultLimit int64, large func(c fiber.Ctx) bool, largeLimit int64) fiber.Handler {
	return func(c fiber.Ctx) error {
		req := c.Request()
		cl := int64(req.Header.ContentLength()) // -1 chunked, -2 без тела

		if large != nil && large(c) {
			c.RequestCtx().SetConnectionClose()
			if cl > largeLimit {
				return tooLarge(c)
			}
			return c.Next()
		}

		if cl > defaultLimit {
			return tooLarge(c)
		}
		if req.IsBodyStream() {
			buf, err := io.ReadAll(io.LimitReader(req.BodyStream(), defaultLimit+1))
			if err != nil {
				c.RequestCtx().SetConnectionClose() // тело прочитано не до конца
				return errorspkg.BadRequest("Некорректное тело запроса")
			}
			if int64(len(buf)) > defaultLimit {
				// Небольшой хвост сверх лимита дочитываем (≤ overflowDrain), чтобы клиент,
				// уже дописывающий тело, получил 413, а не RST/«broken pipe».
				_, _ = io.Copy(io.Discard, io.LimitReader(req.BodyStream(), overflowDrain))
				return tooLarge(c)
			}
			req.SetBody(buf)
		}
		return c.Next()
	}
}

const overflowDrain = 1 << 20

func tooLarge(c fiber.Ctx) error {
	c.RequestCtx().SetConnectionClose()
	return errorspkg.ErrRequestTooLarge
}
