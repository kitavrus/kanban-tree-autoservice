package catalog

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Коды ошибок диапазона/точности NUMERIC-полей (code review Блокер 1 — TD-4 «границы →
// 400, не 500»; msgInvalid/msgRequired/msgTooLong/msgInvalidText — categories_validate.go).
// Значения — алиасы на pkg/numeric (ADR-101): логика диапазона перенесена туда целиком,
// здесь остаются только имена полей формы этой фичи.
const (
	msgInvalidFormat = numeric.MsgInvalidFormat
	msgTooPrecise    = numeric.MsgTooPrecise
	msgTooLarge      = numeric.MsgTooLarge
)

// amountScale — алиас на numeric.Scale (ADR-101).
type amountScale = numeric.Scale

var (
	priceScale = amountScale{IntDigits: 10, FracDigits: 2} // NUMERIC(12,2): products.price, services.price
	qtyScale   = amountScale{IntDigits: 9, FracDigits: 3}  // NUMERIC(12,3): products.stock/min_stock, service_products.qty
)

// validateAmount — тонкая обёртка над numeric.Amount.Validate (ADR-101, 0 дублирующей
// логики диапазона): пустая строка — валидно.
func validateAmount(val string, scale amountScale, allowNegative bool) string {
	return numeric.Amount(val).Validate(scale, allowNegative)
}

// errAmountOutOfRange/ErrAmountOutOfRange — страховка от 500 на PG 22003, которую
// validateAmount() обязан предотвращать на входе для всех известных полей; остаётся
// защитным дном на непредвиденный путь (не привязан к конкретному полю формы — репозиторий
// на уровне RETURNING не знает, какая именно колонка переполнилась).
var (
	errAmountOutOfRange = errors.New("catalog: amount out of range")
	ErrAmountOutOfRange = errorspkg.New("AMOUNT_OUT_OF_RANGE", http.StatusBadRequest, "Введённое число слишком велико для этого поля")
)

// mapAmountOverflow — 22003 numeric_value_out_of_range → errAmountOutOfRange (repo
// sentinel, без HTTP — это знает только service-слой), иначе err как есть.
func mapAmountOverflow(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22003" {
		return errAmountOutOfRange
	}
	return err
}
