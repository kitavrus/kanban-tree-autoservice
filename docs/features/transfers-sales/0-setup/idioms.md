# Идиомы проекта — слепок с фичи `transfers`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/transfers`

## Образцы слоёв (по одному файлу на слой; ещё 11 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/transfers/repo.go`

```go
package transfers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

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

// notAtSourceError — единица числится не в источнике документа (DS-036, DS-013; 409
// TRANSFER_EQUIPMENT_NOT_AT_SOURCE): не на складе отправления либо, при fromClient (приём от
// клиента), не «у клиента».
type notAtSourceError struct {
	equipmentID string
	fromClient  bool
}

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

// ListParams — фильтры списка (комбинация по «И»). CreatedFrom/CreatedTo — границы периода
// в UTC: [CreatedFrom; CreatedTo) (DS-034). ClientOnly — скоуп исполнителя: только документы
// с клиентом (DS-018).
type ListParams struct {
	Status, WarehouseID, Direction, SupplierID string
	TaskID                                     *int64
	CreatedFrom, CreatedTo                     *time.Time
	ClientOnly                                 bool
	Limit, Offset                              int
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

// Head — направление и заказ документа без блокировки (оба после создания не меняются):
// вход валидатора правки и проверки скоупа исполнителя.
func (r *Repo) Head(ctx context.Context, id int64) (direction string, taskID *int64, err error)

// Create — новое перемещение (requested) и, при send, сразу отправка (in_transit); при
// receive (только приём от клиента, DS-013) — ещё и приёмка, всё одной транзакцией. Единицы
// строк занимаются документом (DS-019). Lock Order (DS-048): документ → product_stock →
// equipment.
func (r *Repo) Create(ctx context.Context, in Input, userID string, send, receive bool) (*TransferResponse, error)

// Update — правка заявки (только requested): точки маршрута по направлению, примечание,
// строки целиком; снятые единицы освобождаются, добавленные занимаются (DS-019). Направление
// и заказ не меняются — in.Direction / in.TaskID заполнены из документа (Head).
func (r *Repo) Update(ctx context.Context, id int64, in Input, version int) (*TransferResponse, error)

// Send — requested → in_transit. Склад → склад: −qty_sent с источника (transfer_out); единицы
// из источника (склад отправления либо клиент) → «в пути»; приход от поставщика — только
// статус (DS-040).
func (r *Repo) Send(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error)

// Receive — in_transit → received (эффекты по направлению — doReceive).
func (r *Repo) Receive(ctx context.Context, id int64, version int, in []ReceiveItemInput, userID string) (*TransferResponse, error)

// doReceive — приёмка документа «в пути» (строка transfers уже заблокирована):
//   - склад → склад: факт по товарным строкам, на получателе +qty_sent (transfer_in) и
//     −недостача (transfer_loss);
//   - приход от поставщика: +факт на складе назначения (supplier_in; факт 0 — без движения),
//     факт может превышать заказанное (DS-015);
//   - документы с клиентом: товарных строк и факта нет.
//
// Единицы — целиком в точку назначения (склад либо клиент) и освобождаются (DS-037).
func doReceive(ctx context.Context, tx pgx.Tx, id int64, doc lockedTransfer, in []ReceiveItemInput, userID string) error

// Cancel — requested → cancelled (без движений, единицы освобождаются); in_transit →
// cancelled: склад → склад — +qty_sent на источник (transfer_return); единицы — обратно в
// источник (склад отправления либо клиент); приход от поставщика — без движений (DS-040).
func (r *Repo) Cancel(ctx context.Context, id int64, version int, userID string) (*TransferResponse, error)

// lockedTransfer — шапка документа под блокировкой: from / to / supplier — пустая строка,
// если точка маршрута направлению не положена (NULL).
type lockedTransfer struct {
	status    string
	direction string
	from, to  string
	supplier  string
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

// settleEquipment — единицы документа «в пути» оседают в точке warehouseID (приёмка — точка
// назначения, отмена — источник) и освобождаются. Пустой warehouseID — точка «клиент»:
// единица становится at_client, склад пуст (DS-013).
func settleEquipment(ctx context.Context, tx pgx.Tx, transferID int64, warehouseID string) error

// doSend — отправка заявки (строка transfers уже заблокирована) + статус in_transit.
// Склад → склад: списание с источника, возвращает предупреждения по его остаткам
// (отрицательный/на минимуме — разрешено, D17/D18). Приход от поставщика: движений нет —
// товар ещё не наш (DS-040). Каждая единица обязана числиться в источнике документа: на
// складе отправления (DS-036) либо, для приёма от клиента, «у клиента» (DS-013) — иначе
// notAtSourceError.
func doSend(ctx context.Context, tx pgx.Tx, id int64, doc lockedTransfer, userID string) ([]StockWarning, error)

func insertItems(ctx context.Context, tx pgx.Tx, id int64, items []ItemInput) error

// equipmentIDs — id единиц из строк запроса (порядок строк).
func equipmentIDs(items []ItemInput) []string

// transferEquipmentIDs — id единиц строк документа, по возрастанию id.
func transferEquipmentIDs(ctx context.Context, tx pgx.Tx, id int64) ([]string, error)

func sentItems(ctx context.Context, tx pgx.Tx, id int64) ([]sentItem, error)

func existingProductIDs(ctx context.Context, tx pgx.Tx, id int64) (map[string]bool, error)

// activeFlags — id → is_active для таблицы со столбцом is_active (warehouses, products,
// suppliers).
func activeFlags(ctx context.Context, tx pgx.Tx, table string, ids []string) (map[string]bool, error)

// checkRefs — точки маршрута и строки документа существуют и допустимы:
//   - склады направления существуют и активны;
//   - поставщик (приход от поставщика) существует и активен; архивный, уже стоящий в
//     документе (grandSupplier), допустим при пересохранении (TS-020);
//   - заказ (документы с клиентом) существует — возвращается его клиент (DS-012);
//   - товары существуют и активны (архивный, уже бывший в заявке, — допустим, D52);
//   - единицы существуют; в документе с клиентом — входят в оборудование заказа и
//     принадлежат его клиенту, иначе not_in_task (DS-035).
func checkRefs(ctx context.Context, tx pgx.Tx, in Input, grandfathered map[string]bool, grandSupplier string) (*string, error)

// idSet — множество id (text) из запроса с первым параметром-массивом ids; пустой ids —
// пустое множество без запроса.
func idSet(ctx context.Context, tx pgx.Tx, sql string, ids []string, args ...any) (map[string]bool, error)
```

### `internal/features/transfers/transfers_partners_http_live_test.go`

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

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Направления документа «Перемещение» — клиент и поставщик
// (docs/features/transfers-partners/draft-spec.md §11) — методы TransfersLiveSuite (§TD-8):
// TS-004 (единица из оборудования заказа, клиент — из заказа), TS-006 (хвост: приём, когда
// единица не «у клиента»), TS-007 (движения остатка), TS-008 (приёмка по направлениям),
// TS-015 (права исполнителя), TS-020 (хвост: архивный поставщик), TS-023 (фильтр «период» в
// поясе пользователя), TS-025 («в пути» на «Остатках»), DS-013 (три шага и «принять сразу»).

// ---- фикстуры ----

// newClient — клиент (SQL: фикстура, не предмет теста).
func (s *TransfersLiveSuite) newClient(name string) string

// newTask — заказ клиента в первой колонке доски (SQL: фикстура).
func (s *TransfersLiveSuite) newTask(clientID string) int64

// newUnitOf — единица клиента clientID: warehouseID "" — «у клиента», иначе «на складе».
func (s *TransfersLiveSuite) newUnitOf(clientID, warehouseID string) string

// attach — единица в оборудовании заказа (SQL: фикстура; местоположение не меняется).
func (s *TransfersLiveSuite) attach(taskID int64, unitIDs ...string)

// taskWithUnits — заказ клиента фикстуры с n единицами «у клиента» в его оборудовании.
func (s *TransfersLiveSuite) taskWithUnits(n int) (int64, []string)

func (s *TransfersLiveSuite) newSupplier(active bool) string

// executor — исполнитель (общий на сьюту), вошедший через реальный логин.
func (s *TransfersLiveSuite) executor() *testutil.Browser

func unitItems(ids ...string) []map[string]any

func intakeBody(taskID int64, to string, units ...string) map[string]any

func returnBody(taskID int64, from string, units ...string) map[string]any

func (s *TransfersLiveSuite) supplyBody(supplierID, productID string, qty any) map[string]any

func with(body map[string]any, kv ...any) map[string]any

// doAs — запрос от имени b; успешный ответ разбирается как документ.
func (s *TransfersLiveSuite) doAs(b *testutil.Browser, method, path string, body any) (int, transferDTO, []byte)

func (s *TransfersLiveSuite) get(id int64) transferDTO

// listIDs — id документов списка от имени b с фильтром query.
func (s *TransfersLiveSuite) listIDs(b *testutil.Browser, query string) (map[int64]bool, listDTO)

// productMovementsSum — сумма движений остатка пары товар × склад по всем причинам.
func (s *TransfersLiveSuite) productMovementsSum(productID, warehouseID string) string

// ---- DS-013: три шага документов с клиентом, «принять сразу» ----

// Приём от клиента: заявка → отправка («в пути», единица занята документом) → приёмка
// (единица на складе назначения, свободна); возврат клиенту: со склада → «в пути» → «у
// клиента». Движений остатка документы с клиентом не создают (TS-007). Ответ несёт
// направление, заказ, клиента заказа; склад «чужой» точки маршрута — null.
func (s *TransfersLiveSuite) TestClientDocuments_ThreeSteps()

// Отмена документов с клиентом: заявка — единица свободна; «в пути» — единица возвращается в
// источник: приём → «у клиента», возврат → склад отправления.
func (s *TransfersLiveSuite) TestClientDocuments_Cancel()

// receive: true — создать, отправить и принять одной транзакцией (клиент принёс сам); при
// ошибке не создаётся ничего (§3 п.3).
func (s *TransfersLiveSuite) TestClientToWarehouse_ReceiveNow()

// ---- TS-004 ----

// TS-004: единица документа с клиентом обязана входить в оборудование заказа документа и
// принадлежать его клиенту — иначе 400 items[i].equipment_id: not_in_task; client_id в теле
// игнорируется — клиент берётся из заказа (DS-012, abuse cases 3–4).
func (s *TransfersLiveSuite) TestTS004_EquipmentNotInTask()

// TS-004 / TS-018 (гонка, DS-035 / DS-026): документ с клиентом создаётся параллельно со
// снятием единицы с заказа либо со сменой клиента заказа. Заказ блокируется документом
// (FOR KEY SHARE) до чтения task_equipment, поэтому исход — строго один из двух: документ
// создан, а единица осталась в заказе (board → 409 EQUIPMENT_IN_TRANSFER), либо единица
// убрана, а документ получил not_in_task. «Единица в незавершённом документе, но не в заказе»
// и transfers.client_id ≠ tasks.client_id недостижимы.
func (s *TransfersLiveSuite) TestTS004_ParallelCreateVsTaskEquipmentChange()

// ---- TS-006 (хвост) ----

// TS-006: отправка «клиент → склад», когда единица не «у клиента», и «склад → клиент», когда
// она не на складе отправления, → 409 TRANSFER_EQUIPMENT_NOT_AT_SOURCE со своим текстом;
// send / receive при создании — тот же 409, документ не создан.
func (s *TransfersLiveSuite) TestTS006_ClientDocuments_NotAtSource()

// ---- TS-007 ----

// TS-007: приход от поставщика — при отправке и отмене движений нет, при приёмке supplier_in
// +факт на складе назначения; остаток product_stock сходится с суммой движений; факт 0 —
// движение не пишется, документ «Получено».
func (s *TransfersLiveSuite) TestTS007_SupplierMovements()

// ---- TS-008 ----

// TS-008: приёмка по направлениям — склад → склад: факт > отправленного → 400 exceeds_sent;
// приход от поставщика: факт больше / меньше заказанного → 200, остаток +факт; items у
// документов с клиентом → 400 not_allowed; receive: true вне приёма от клиента → 400;
// отрицательный факт → 400; параллельный receive прихода — остаток вырос один раз.
func (s *TransfersLiveSuite) TestTS008_ReceiveByDirection()

// ---- TS-015 ----

// TS-015: исполнитель создаёт и проводит документы с клиентом (201 / 200); создание «склад →
// склад» и «поставщик → склад» → 403; GET / PUT / send / receive / cancel документа чужого
// направления → 404 (существование не раскрывается); список не содержит чужих направлений
// даже с фильтром direction (abuse cases 1–2).
func (s *TransfersLiveSuite) TestTS015_ExecutorScope()

// attachNew — новая единица клиента фикстуры «у клиента», сразу в оборудовании заказа.
func (s *TransfersLiveSuite) attachNew(taskID int64) string

// TS-015 / TS-034 (SSR): экран и пункт меню «Перемещения» доступны исполнителю; меню
// направлений и фильтр «Направление» — два значения, фильтра «Поставщик» нет. Администратору
// — все четыре направления и фильтр «Поставщик».
func (s *TransfersLiveSuite) TestTS015_SSR_ExecutorScreen()

// ---- TS-020 (хвост) ----

// TS-020: архивный поставщик в новом документе → 400 supplier_id: archived; в уже созданном
// остаётся — заявка пересохраняется и проводится; неизвестный → not_found.
func (s *TransfersLiveSuite) TestTS020_ArchivedSupplierInDocument()

// ---- TS-023 ----

// TS-023: фильтр «период», граница суток. Документ создан 2026-10-06T19:30:00Z — 00:30
// 7 октября по Алматы и 22:30 6 октября по Москве. Пользователь с поясом Asia/Almaty находит
// его 7 октября и не находит 6-го; пользователь с Europe/Moscow — наоборот. Негатив: «по»
// раньше «с» и несуществующая дата → 400 (abuse case 6).
func (s *TransfersLiveSuite) TestTS023_PeriodFilter_UserTimezone()

// Фильтры списка §6.1: направление, заказ, поставщик — формат, комбинация по «И», поля строки.
func (s *TransfersLiveSuite) TestListFilters_DirectionTaskSupplier()

// ---- TS-025 ----

// TS-025: «Остатки» — «в пути» включает приход от поставщика в статусе «В пути» для склада
// назначения; заявка (requested), полученный и отменённый документ в «в пути» не входят.
func (s *TransfersLiveSuite) TestTS025_StockInTransit_IncludesSupplier()
```

---

Исходники образца: 240,630 символов · слепок: 17,506 · сжатие ×13.7
