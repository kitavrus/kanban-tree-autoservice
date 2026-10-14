package errorspkg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppError_WithDetail_ReturnsCopy_DoesNotMutateSentinel — ADR-102: sentinel'ы это
// общие var в пакетах-фичах, WithDetail обязан вернуть НОВЫЙ *AppError, не трогая исходный
// (иначе один запрос подмешал бы Extra в ошибку, которую увидит другой параллельный запрос).
func TestAppError_WithDetail_ReturnsCopy_DoesNotMutateSentinel(t *testing.T) {
	sentinel := New("SUBTASKS_NOT_DONE", http.StatusConflict, "Есть незавершённые услуги")

	withDetail := sentinel.WithDetail("subtasks", []string{"a", "b"})

	assert.Nil(t, sentinel.Extra, "исходный sentinel не мутирован")
	require.NotNil(t, withDetail.Extra)
	assert.Equal(t, []string{"a", "b"}, withDetail.Extra["subtasks"])
	assert.NotSame(t, sentinel, withDetail, "WithDetail возвращает копию, не тот же указатель")

	// повторный вызов на исходном sentinel — снова чистая копия, без утечки между вызовами.
	again := sentinel.WithDetail("other", 1)
	assert.Nil(t, sentinel.Extra)
	assert.NotContains(t, again.Extra, "subtasks")
}

func doErrorHandler(t *testing.T, err error) map[string]any {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/x", func(c fiber.Ctx) error { return err })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	resp, doErr := app.Test(req)
	require.NoError(t, doErr)
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

// TestFiberErrorHandler_MergesFieldsAndExtraIntoDetails — details = {"fields": ...} ∪ Extra.
func TestFiberErrorHandler_MergesFieldsAndExtraIntoDetails(t *testing.T) {
	base := Validation(Field{Name: "title", Message: "required"})
	withExtra := base.WithDetail("subtasks", []map[string]string{{"id": "1"}})

	body := doErrorHandler(t, withExtra)

	details, ok := body["details"].(map[string]any)
	require.True(t, ok, "details присутствует: %+v", body)
	assert.NotEmpty(t, details["fields"])
	assert.NotEmpty(t, details["subtasks"])
}

// TestFiberErrorHandler_OmitsDetailsWhenEmpty — Fields и Extra оба пусты → details
// опускается из тела (существующий контракт для sentinel-ошибок без деталей).
func TestFiberErrorHandler_OmitsDetailsWhenEmpty(t *testing.T) {
	body := doErrorHandler(t, NotFound("не найдено"))

	_, hasDetails := body["details"]
	assert.False(t, hasDetails, "%+v", body)
	assert.Equal(t, "NOT_FOUND", body["code"])
}
