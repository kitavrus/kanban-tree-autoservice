# Идиомы проекта — слепок с фичи `stocktransfers`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/stocktransfers`

## Образцы слоёв (по одному файлу на слой; ещё 8 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/stocktransfers/dto.go`

```go
package stocktransfers

import (
	"time"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

// Статусы перемещения (DS-006).
const (

// Statuses — допустимые значения фильтра списка.
var Statuses = []string{StatusRequested, StatusInTransit, StatusReceived, StatusCancelled}

// Причины движений перемещения (DS-007, CHECK stock_movements_reason_check миграции 0008).
const (

// ItemInput — строка товара в запросе создания/правки.
type ItemInput struct {
	ProductID string         `json:"product_id"`
	QtySent   numeric.Amount `json:"qty_sent"`
}

// TransferCreateRequest — POST /api/v1/stock-transfers. send=true — сразу «Отправить»
// (in_transit, DS-006), иначе «Заявка» (requested).
type TransferCreateRequest struct {
	FromWarehouseID string      `json:"from_warehouse_id"`
	ToWarehouseID   string      `json:"to_warehouse_id"`
	Note            string      `json:"note"`
	Items           []ItemInput `json:"items"`
	Send            bool        `json:"send"`
}

// TransferUpdateRequest — PUT /api/v1/stock-transfers/:id (только requested).
type TransferUpdateRequest struct {
	FromWarehouseID string      `json:"from_warehouse_id"`
	ToWarehouseID   string      `json:"to_warehouse_id"`
	Note            string      `json:"note"`
	Items           []ItemInput `json:"items"`
	Version         *int        `json:"version"`
}

// TransferActionRequest — POST /:id/send и /:id/cancel.
type TransferActionRequest struct {
	Version *int `json:"version"`
}

// ReceiveItemInput — факт по строке при приёмке; строка без записи — факт = qty_sent.
type ReceiveItemInput struct {
	ID          string         `json:"id"`
	QtyReceived numeric.Amount `json:"qty_received"`
}

// TransferReceiveRequest — POST /:id/receive.
type TransferReceiveRequest struct {
	Version *int               `json:"version"`
	Items   []ReceiveItemInput `json:"items"`
}

// StockWarning — остаток пары товар×склад после списания с источника (отрицательный или
// на/ниже минимума; разрешено, как D17/D18 — только предупреждение).
type StockWarning struct {
	ProductID     string         `json:"product_id"`
	WarehouseID   string         `json:"warehouse_id"`
	WarehouseName string         `json:"warehouse_name"`
	Name          string         `json:"name"`
	Stock         numeric.Amount `json:"stock"`
	MinStock      numeric.Amount `json:"min_stock"`
	Kind          string         `json:"kind"` // negative | below_min
}

type TransferItemResponse struct {
	ID          string          `json:"id"`
	ProductID   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	Unit        string          `json:"unit"`
	QtySent     numeric.Amount  `json:"qty_sent"`
	QtyReceived *numeric.Amount `json:"qty_received"`
}

// TransferResponse — карточка перемещения (re-fetch-after-write). Warnings — только в
// ответе отправки (иначе пустой массив).
type TransferResponse struct {
	ID                int64                  `json:"id"`
	FromWarehouseID   string                 `json:"from_warehouse_id"`
	FromWarehouseName string                 `json:"from_warehouse_name"`
	ToWarehouseID     string                 `json:"to_warehouse_id"`
	ToWarehouseName   string                 `json:"to_warehouse_name"`
	Status            string                 `json:"status"`
	Note              string                 `json:"note"`
	CreatedBy         string                 `json:"created_by"`
	CreatedByName     string                 `json:"created_by_name"`
	SentBy            *string                `json:"sent_by"`
	SentByName        *string                `json:"sent_by_name"`
	SentAt            *time.Time             `json:"sent_at"`
	ReceivedBy        *string                `json:"received_by"`
	ReceivedByName    *string                `json:"received_by_name"`
	ReceivedAt        *time.Time             `json:"received_at"`
	CancelledBy       *string                `json:"cancelled_by"`
	CancelledByName   *string                `json:"cancelled_by_name"`
	CancelledAt       *time.Time             `json:"cancelled_at"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	Version           int                    `json:"version"`
	Items             []TransferItemResponse `json:"items"`
	Warnings          []StockWarning         `json:"warnings"`
}

// TransferSummary — строка списка (без строк товаров).
type TransferSummary struct {
	ID                int64     `json:"id"`
	FromWarehouseID   string    `json:"from_warehouse_id"`
	FromWarehouseName string    `json:"from_warehouse_name"`
	ToWarehouseID     string    `json:"to_warehouse_id"`
	ToWarehouseName   string    `json:"to_warehouse_name"`
	Status            string    `json:"status"`
	Note              string    `json:"note"`
	ItemsCount        int       `json:"items_count"`
	CreatedByName     string    `json:"created_by_name"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Version           int       `json:"version"`
}

type TransferListResponse struct {
	Items  []TransferSummary `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}
```

### `internal/features/stocktransfers/stocktransfers_http_live_test.go`

```go
//go:build integration

package stocktransfers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// StockTransfersLiveSuite (§TD-8): перемещения товаров между складами
// (docs/features/warehouses/draft-spec.md DS-006/DS-007/DS-010/DS-011) — TS-008 (заявка →
// правка → отправка → приёмка с недостачей), TS-009 (отмены, финальные статусы, from = to),
// TS-010 (version, двойной receive), TS-007 «В пути», TS-014 (executor → 403), SSR.
type StockTransfersLiveSuite struct {
	testutil.LiveSuite
	admin   *testutil.Browser
	adminID string
	almaty  string
	moscow  string
	catID   string
	seq     int
}

func TestStockTransfersLiveSuite(t *testing.T)

func (s *StockTransfersLiveSuite) SetupSuite()

// ---- wire-типы ----

type itemDTO struct {
	ID          string       `json:"id"`
	ProductID   string       `json:"product_id"`
	ProductName string       `json:"product_name"`
	QtySent     json.Number  `json:"qty_sent"`
	QtyReceived *json.Number `json:"qty_received"`
}

type warningDTO struct {
	ProductID   string      `json:"product_id"`
	WarehouseID string      `json:"warehouse_id"`
	Stock       json.Number `json:"stock"`
	Kind        string      `json:"kind"`
}

type transferDTO struct {
	ID                int64        `json:"id"`
	FromWarehouseID   string       `json:"from_warehouse_id"`
	FromWarehouseName string       `json:"from_warehouse_name"`
	ToWarehouseID     string       `json:"to_warehouse_id"`
	Status            string       `json:"status"`
	Note              string       `json:"note"`
	CreatedByName     string       `json:"created_by_name"`
	SentAt            *string      `json:"sent_at"`
	ReceivedAt        *string      `json:"received_at"`
	CancelledAt       *string      `json:"cancelled_at"`
	Version           int          `json:"version"`
	Items             []itemDTO    `json:"items"`
	Warnings          []warningDTO `json:"warnings"`
}

type listDTO struct {
	Items []struct {
		ID         int64  `json:"id"`
		Status     string `json:"status"`
		ItemsCount int    `json:"items_count"`
	} `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

// ---- хелперы ----

func (s *StockTransfersLiveSuite) errOf(raw []byte) apiErr

func (s *StockTransfersLiveSuite) fields(raw []byte) map[string]string

// newProduct — товар с остатком stock на «Алматы» (SQL: фикстура, не предмет теста).
func (s *StockTransfersLiveSuite) newProduct(stock string) string

func (s *StockTransfersLiveSuite) pairStock(productID, warehouseID string) string

// movements — reason:склад:delta по перемещению (id движений — uuid, порядок не задан).
func (s *StockTransfersLiveSuite) movements(transferID int64) []string

func (s *StockTransfersLiveSuite) do(method, path string, body any) (int, transferDTO, []byte)

func (s *StockTransfersLiveSuite) create(body map[string]any) transferDTO

func (s *StockTransfersLiveSuite) action(t transferDTO, act string, body map[string]any) (int, transferDTO, []byte)

func (s *StockTransfersLiveSuite) inTransit(productID, warehouseID string) string

// ---- TS-008 ----

// TS-008: заявка → правка строк → отправка (минус на источнике, «в пути») → приёмка по
// факту с недостачей (приход + transfer_loss) — итоговые остатки верны.
func (s *StockTransfersLiveSuite) TestTS008_RequestEditSendReceiveWithLoss()

// Создание сразу «Отправить» (send=true) — in_transit одной операцией.
func (s *StockTransfersLiveSuite) TestCreateAndSend()

// ---- TS-009 ----

// TS-009: отмена in_transit → возврат на источник; отмена requested → без движений;
// правка received → 409; from = to → 400.
func (s *StockTransfersLiveSuite) TestTS009_CancelFinalSameWarehouse()

// Валидация тела: строки, склады/товары (не существует, архив), приёмка вне границ, мусор.
func (s *StockTransfersLiveSuite) TestValidation()
```

---

Исходники образца: 70,679 символов · слепок: 9,381 · сжатие ×7.5
