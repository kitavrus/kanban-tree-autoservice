# Идиомы проекта — слепок с фичи `transfers`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/transfers`

## Образцы слоёв (по одному файлу на слой; ещё 9 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/transfers/repo.go`

```go
package transfers

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

var (
	// errVersion — присланный version не совпал с текущим (409, DS-048).
	// errOverflow — 22003 numeric_value_out_of_range при изменении остатка.

// Действия над документом — ключ сообщения 409 TRANSFER_STATUS_CONFLICT (§6.4–§6.7).
const (

// statusError — действие недоступно в текущем статусе документа (409; сообщение зависит от
// пары действие × статус).
type statusError struct{ action, status string }

func (e *statusError) Error() string

// equipmentBusyError — единица уже стоит в другом незавершённом документе (DS-019, 409
// EQUIPMENT_IN_TRANSFER).
type equipmentBusyError struct {
	equipmentID string
	transferID  int64
}

func (e *equipmentBusyError) Error() string { return "transfers: equipment in transfer" }

// notAtSourceError — единица числится не на складе отправления (DS-036, 409
// TRANSFER_EQUIPMENT_NOT_AT_SOURCE).
type notAtSourceError struct{ equipmentID string }

func (e *notAtSourceError) Error() string { return "transfers: equipment not at source" }

// fieldsError — ошибки полей, найденные под транзакцией (склад/товар/единица не существует
// или в архиве, факт приёмки вне границ).
type fieldsError struct{ Fields []errorspkg.Field }

func (e *fieldsError) Error() string { return "transfers: invalid fields" }

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

type ListParams struct {
	Status, WarehouseID string
	Limit, Offset       int
}

func nullable(s string) *string

// List — перемещения, новые сверху; warehouse_id — откуда ИЛИ куда.
func (r *Repo) List(ctx context.Context, p ListParams) ([]TransferSummary, int, error)

// Get — карточка перемещения со строками (q — пул или транзакция).
func (r *Repo) Get(ctx context.Context, id int64) (*TransferResponse, error)

func getTransfer(ctx context.Context, q querier, id int64) (*TransferResponse, error)

func amountPtr(s *string) *numeric.Amount

// inTx — транзакция с откатом при ошибке; fn возвращает id перемещения для re-fetch.
func (r *Repo) inTx(ctx context.Context, fn func(tx pgx.Tx) ([]StockWarning, int64, error)) (*TransferResponse, error)

func mapPgErr(err error) error

// Create — новое перемещение (requested) и, при send, сразу отправка (in_transit). Единицы
// строк занимаются документом (DS-019). Lock Order (DS-048): документ → product_stock →
// equipment.
func (r *Repo) Create(ctx context.Context, in Input, userID string, send bool) (*TransferResponse, error)

// Update — правка заявки (только requested): склады, примечание, строки целиком; снятые
// единицы освобождаются, добавленные занимаются (DS-019).
func (r *Repo) Update(ctx context.Context, id int64, in Input, version int) (*TransferResponse, error)

// Send — requested → in_transit: −qty_sent с источника (transfer_out), единицы → «в пути».
func (r *Repo) Send(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error)

// Receive — in_transit → received: факт по товарным строкам, на получателе +qty_sent
// (transfer_in) и −недостача (transfer_loss); единицы — целиком на склад назначения и
// освобождаются (DS-037).
func (r *Repo) Receive(ctx context.Context, id int64, version int, in []ReceiveItemInput, userID string) (*TransferResponse, error)

// Cancel — requested → cancelled (без движений, единицы освобождаются); in_transit →
// cancelled (+qty_sent на источник, transfer_return; единицы — обратно на склад отправления).
func (r *Repo) Cancel(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error)

type lockedTransfer struct {
	status   string
	from, to string
}

// lockTransfer — FOR UPDATE строки перемещения (первая блокировка транзакции, D50: затем
// product_stock, затем equipment); version сверяется ДО статуса — устаревшая форма получает
// 409 version.
func lockTransfer(ctx context.Context, tx pgx.Tx, id int64, version int, action string, allowed ...string) (lockedTransfer, error)

// lockPairs — D34 для перемещений: отсутствующие строки product_stock (товар×склад)
// создаются ON CONFLICT DO NOTHING, затем блокируются FOR UPDATE в порядке
// (product_id, warehouse_id). Только товарные строки.
func lockPairs(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error

// equipmentState — местоположение и занятие единицы под блокировкой строки.
type equipmentState struct {
	location    string
	warehouseID *string
	activeID    *int64
}

// lockEquipment — блокировка строк equipment в порядке id (последняя ступень Lock Order).
// FOR NO KEY UPDATE, не FOR UPDATE: вставка строки документа уже держит FOR KEY SHARE на
// единице (FK transfer_items.equipment_id) — два параллельных создания с одной единицей на
// FOR UPDATE взаимно заблокировались бы (40P01), а так второй ждёт первого и получает 409.
func lockEquipment(ctx context.Context, tx pgx.Tx, ids []string) (map[string]equipmentState, error)

// claimEquipment — занятие единиц документом (DS-019, DS-041): guard-UPDATE
// active_transfer_id IS NULL; единица занята другим документом → equipmentBusyError с его
// номером. ids — в порядке строк запроса; ошибка — по первой занятой в порядке id.
func claimEquipment(ctx context.Context, tx pgx.Tx, transferID int64, ids []string) error

// settleEquipment — единицы документа «в пути» оседают на складе warehouseID (приёмка —
// склад назначения, отмена — склад отправления) и освобождаются.
func settleEquipment(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error

// doSend — списание с источника + единицы «в пути» + статус in_transit; возвращает
// предупреждения по остаткам источника (отрицательный/на минимуме — разрешено, D17/D18).
// Каждая единица обязана числиться на складе отправления (DS-036), иначе notAtSourceError.
func doSend(ctx context.Context, tx pgx.Tx, id int64, userID string) ([]StockWarning, error)

func insertItems(ctx context.Context, tx pgx.Tx, id int64, items []ItemInput) error

// equipmentIDs — id единиц из строк запроса (порядок строк).
func equipmentIDs(items []ItemInput) []string

// transferEquipmentIDs — id единиц строк документа, по возрастанию id.
func transferEquipmentIDs(ctx context.Context, tx pgx.Tx, id int64) ([]string, error)

func sentItems(ctx context.Context, tx pgx.Tx, id int64) ([]sentItem, error)

func existingProductIDs(ctx context.Context, tx pgx.Tx, id int64) (map[string]bool, error)

// activeFlags — id → is_active для таблицы со столбцом is_active (warehouses, products).
func activeFlags(ctx context.Context, tx pgx.Tx, table string, ids []string) (map[string]bool, error)

// checkRefs — склады существуют и активны; товары существуют и активны (архивный товар,
// уже бывший в этой заявке, — допустим при пересохранении, D52); единицы существуют.
func checkRefs(ctx context.Context, tx pgx.Tx, in Input, grandfathered map[string]bool) error
```

### `internal/features/transfers/transfers_http_live_test.go`

```go
//go:build integration

package transfers_test

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

// TransfersLiveSuite (§TD-8): документ «Перемещение» склад → склад со смешанными строками
// (docs/features/transfers-core/draft-spec.md) — товары (as-is: заявка → правка → отправка →
// приёмка с недостачей, отмены, «В пути» на остатках) и единицы оборудования клиентов:
// TS-005 (занятие единицы документом), TS-006 (отправка только со склада отправления),
// TS-009 (version до статуса, свои сообщения переходов, двойной receive), TS-028 (мусорное
// тело — без 5xx), admin-guard (executor → 403), SSR.
type TransfersLiveSuite struct {
	testutil.LiveSuite
	admin   *testutil.Browser
	adminID string
	almaty  string
	moscow  string
	catID   string
	// clientID/typeID — владелец и тип единиц оборудования фикстуры.
	clientID string
	typeID   string
	seq      int
}

func TestTransfersLiveSuite(t *testing.T)

func (s *TransfersLiveSuite) SetupSuite()

// ---- wire-типы ----

// itemDTO — строка документа: у товара заполнены product_*/qty_*, у единицы — equipment_*
// (остальное в JSON — null, здесь — нулевое значение).
type itemDTO struct {
	ID                  string       `json:"id"`
	Kind                string       `json:"kind"`
	ProductID           string       `json:"product_id"`
	ProductName         string       `json:"product_name"`
	EquipmentID         string       `json:"equipment_id"`
	EquipmentName       string       `json:"equipment_name"`
	EquipmentSerial     string       `json:"equipment_serial"`
	EquipmentClientName string       `json:"equipment_client_name"`
	QtySent             *json.Number `json:"qty_sent"`
	QtyReceived         *json.Number `json:"qty_received"`
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
	Message string `json:"message"`
	Details *struct {
		Fields      []struct{ Name, Message string } `json:"fields"`
		TransferID  int64                            `json:"transfer_id"`
		EquipmentID string                           `json:"equipment_id"`
	} `json:"details"`
}

// ---- хелперы ----

func (s *TransfersLiveSuite) errOf(raw []byte) apiErr

func (s *TransfersLiveSuite) fields(raw []byte) map[string]string

// newProduct — товар с остатком stock на «Алматы» (SQL: фикстура, не предмет теста).
func (s *TransfersLiveSuite) newProduct(stock string) string

func (s *TransfersLiveSuite) pairStock(productID, warehouseID string) string

// movements — reason:склад:delta по перемещению (id движений — uuid, порядок не задан).
func (s *TransfersLiveSuite) movements(transferID int64) []string

func (s *TransfersLiveSuite) do(method, path string, body any) (int, transferDTO, []byte)

func (s *TransfersLiveSuite) create(body map[string]any) transferDTO

func (s *TransfersLiveSuite) action(t transferDTO, act string, body map[string]any) (int, transferDTO, []byte)

func (s *TransfersLiveSuite) inTransit(productID, warehouseID string) string

// ---- товарные строки (as-is) ----

// Заявка → правка строк → отправка (минус на источнике, «в пути») → приёмка по
// факту с недостачей (приход + transfer_loss) — итоговые остатки верны.
func (s *TransfersLiveSuite) TestProducts_RequestEditSendReceiveWithLoss()

// Создание сразу «Отправить» (send=true) — in_transit одной операцией.
func (s *TransfersLiveSuite) TestCreateAndSend()

// Отмена in_transit → возврат на источник; отмена requested → без движений;
// правка received → 409; from = to → 400.
func (s *TransfersLiveSuite) TestProducts_CancelFinalSameWarehouse()

// Валидация тела: строки, склады/товары (не существует, архив), приёмка вне границ, мусор.
func (s *TransfersLiveSuite) TestValidation()
```

---

Исходники образца: 134,698 символов · слепок: 12,131 · сжатие ×11.1
