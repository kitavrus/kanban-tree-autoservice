# Идиомы проекта — слепок с фичи `softdelete`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/softdelete`

## Образцы слоёв (по одному файлу на слой; ещё 18 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/softdelete/clients_source.go`

```go
package softdelete

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ClientsSource — удалённые клиенты (вкладка «Клиенты», часть 4 soft-delete-clients). Читает
// таблицу clients напрямую: softdelete — reader, отметку удаления пишет пакет clients.
type ClientsSource struct {
	pool *pgxpool.Pool
}

func NewClientsSource(pool *pgxpool.Pool) *ClientsSource { return &ClientsSource{pool: pool} }

// clientsWhereSQL — фильтр вкладки «Клиенты»: $1 — строка поиска (название компании, причина;
// уже экранирована), $2/$3 — период [с; по), $4 — кто удалил. Номера у клиента нет. Частичный
// индекс clients_deleted_at_idx покрывает условие и сортировку.
const clientsWhereSQL = `

// List — строка клиента: title — название компании, subtitle — пустая строка (§6.5).
func (s *ClientsSource) List(ctx context.Context, f Filter) ([]Item, int, error)

func (s *ClientsSource) Count(ctx context.Context) (int, error)

// filterArgs — общие параметры $1..$4 фильтра вкладок клиентов и оборудования: пустые строки —
// NULL («без фильтра»).
func filterArgs(f Filter) []any

// EquipmentSource — удалённые единицы оборудования (вкладка «Оборудование», часть 4
// soft-delete-clients). Читает equipment напрямую; отметку удаления пишет пакет clients.
type EquipmentSource struct {
	pool *pgxpool.Pool
}

func NewEquipmentSource(pool *pgxpool.Pool) *EquipmentSource { return &EquipmentSource{pool: pool} }

// DeletedWithClient — значение deleted_with у единицы, ушедшей вместе с клиентом (DS-015).
const DeletedWithClient = "client"

// EquipmentTitle — заголовок строки удалённой единицы: «тип марка модель» (§6.5); пустая марка
// пропускается.
func EquipmentTitle(typeName, brand, model string) string

// EquipmentDeletedWith — deleted_with строки единицы: "client" у ушедшей вместе с клиентом,
// иначе nil (JSON null).
func EquipmentDeletedWith(withClient bool) *string

// equipmentWhereSQL — фильтр вкладки «Оборудование»: $1 — строка поиска (тип, марка, модель,
// серийный номер, название клиента, причина; уже экранирована), $2/$3 — период, $4 — кто
// удалил. Клиент и тип соединяются без условия «не удалён»: клиент единицы сам может быть в
// удалённых. Частичный индекс equipment_deleted_at_idx покрывает условие и сортировку.
const equipmentWhereSQL = `

// List — строка единицы: title — «тип марка модель», subtitle — название клиента,
// deleted_with — "client" у ушедших вместе с клиентом (§6.5, DS-015).
func (s *EquipmentSource) List(ctx context.Context, f Filter) ([]Item, int, error)

func (s *EquipmentSource) Count(ctx context.Context) (int, error)
```

### `internal/features/softdelete/softdelete_http_live_test.go`

```go
//go:build integration

package softdelete_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SoftDeleteLiveSuite (§TD-8: боевой бинарь, реальный логин) — раздел «Удалённые»
// (docs/features/soft-delete-core/draft-spec.md §6.3, §6.4, §8.1): TS-009 (список: вид,
// фильтры по «И», конверт, сортировка), TS-010 (период в поясе пользователя), TS-011
// (счётчики вкладок), TS-033 (права на адресах раздела), SSR-оболочка экрана /deleted и пункт
// меню. Заказы создаются и удаляются через API доски того же сервера.
type SoftDeleteLiveSuite struct {
	testutil.LiveSuite
	admin   *testutil.Browser
	adminID string
}

func TestSoftDeleteLiveSuite(t *testing.T)

func (s *SoftDeleteLiveSuite) SetupSuite()

// ---- wire-типы (1:1 с softdelete.Item / ListResponse / SummaryResponse, §TD-6.2) ----

type itemDTO struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"deleted_by"`
	DeleteReason string  `json:"delete_reason"`
	DeletedWith  *string `json:"deleted_with"`
	IsArchived   bool    `json:"is_archived"`
}

type listDTO struct {
	Items  []itemDTO `json:"items"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

type summaryDTO struct {
	Counts map[string]int `json:"counts"`
}

type errDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *struct {
		Fields []struct {
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"details"`
}

const api = "/api/v1/deleted"

func (s *SoftDeleteLiveSuite) errOf(raw []byte) errDTO

func (s *SoftDeleteLiveSuite) fields(raw []byte) map[string]string

// list — GET /api/v1/deleted от имени b; query — пары ключ, значение.
func (s *SoftDeleteLiveSuite) list(b *testutil.Browser, kv ...string) (int, listDTO, []byte)

func (s *SoftDeleteLiveSuite) mustList(kv ...string) listDTO

func ids(l listDTO) []string

// ---- фикстуры ----

func (s *SoftDeleteLiveSuite) newClient(company string) string

func (s *SoftDeleteLiveSuite) columnID() string

func (s *SoftDeleteLiveSuite) newTask(b *testutil.Browser, clientID, title string) int64

// deleted — заказ, мягко удалённый b с причиной reason; at != "" — момент удаления
// (фикстура времени, SQL: не предмет теста). Возвращает номер заказа строкой.
func (s *SoftDeleteLiveSuite) deleted(b *testutil.Browser, clientID, title, reason, at string) string

func (s *SoftDeleteLiveSuite) user(login, name, role string) (*testutil.Browser, string)

func (s *SoftDeleteLiveSuite) setTimezone(b *testutil.Browser, tz string)

// ---- TS-009 ----

// TS-009 (DS-011, DS-028): kind обязателен и из реестра; параметры валидируются (400 с
// details.fields[]). Негатив: kind=orders → invalid, limit=201 → invalid.
func (s *SoftDeleteLiveSuite) TestTS009_Validation()

// TS-009: строка списка и конверт {items, total, limit, offset}; сортировка — deleted_at по
// убыванию, при равенстве — id; пагинация; фильтры q (название, причина, номер — без учёта
// регистра), deleted_by, период — сочетаются по «И». Виды, чья часть не смержена, — пустой
// список без ошибки.
func (s *SoftDeleteLiveSuite) TestTS009_ListFiltersEnvelopeOrder()

// ---- TS-009: вид transfers (часть 2, soft-delete-transfers §6.3, §8.2) ----

// newTransfer — заявка «склад → склад» Алматы → Москва с одной товарной строкой, созданная b
// через API документов того же сервера (товар — SQL-фикстура).
func (s *SoftDeleteLiveSuite) newTransfer(b *testutil.Browser, mark string) (int64, int)

// TS-009 (DS-011, soft-delete-transfers §6.3): вид transfers отдаёт удалённые документы —
// title «Перемещение #N», subtitle «направление · статус», is_archived = false, deleted_with =
// null; сортировка — deleted_at по убыванию; поиск по причине и номеру («#N»), фильтр «кто
// удалил»; counts.transfers = total списка. Восстановленный документ из списка уходит.
// Негатив: действующий документ в список не входит; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_TransfersKind()

// ---- TS-009: вид columns (часть 3, soft-delete-board-columns §6.3, §8.2) ----

// newColumn — колонка доски, созданная b через API доски того же сервера.
func (s *SoftDeleteLiveSuite) newColumn(b *testutil.Browser, name string, isFinal bool) string

// TS-009 (DS-011, soft-delete-board-columns §6.3): вид columns отдаёт удалённые колонки —
// title — название, subtitle «финальная» либо пустая строка, is_archived = false, deleted_with =
// null; сортировка — deleted_at по убыванию; поиск по названию и причине без учёта регистра,
// фильтр «кто удалил»; counts.columns = total списка. Восстановленная колонка из списка
// уходит. Негатив: действующая колонка в список не входит; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_ColumnsKind()

// ---- TS-009: виды каталога (часть 5, soft-delete-catalog §6.9, §8.2) ----

// postID — POST path с телом body от имени администратора, 201 → id созданной записи.
func (s *SoftDeleteLiveSuite) postID(path string, body map[string]any) string

// TS-009 (DS-011, DS-009, soft-delete-catalog §6.9): виды products, services,
// catalog-categories, equipment-types отдают удалённые записи — title — название; subtitle — у
// товара и услуги название категории, у категории «товары» либо «услуги», у типа пустая строка;
// is_archived = true только у записи, удалённой из архива (TS-028 негатив); deleted_with = null;
// поиск по названию и причине, фильтр «кто удалил»; counts вкладки = total списка;
// восстановленная запись из списка уходит. Негатив: действующая запись в список не входит; у
// записи, удалённой активной, is_archived = false; исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS009_CatalogKinds()

// ---- TS-010 ----

// TS-010 (DS-025): заказ удалён 2026-10-06T23:30:00Z — это 6 октября для пользователя с
// поясом UTC и 7 октября для пользователя с поясом Europe/Moscow (+03:00): каждый находит его
// в «свой» день и не находит в соседний. Негатив: deleted_to раньше deleted_from → 400.
func (s *SoftDeleteLiveSuite) TestTS010_PeriodInUserTimezone()

// ---- TS-011 ----

// TS-011 (DS-011): счётчики вкладок — по всем 11 видам реестра (ключи snake_case), каждый
// совпадает с total своего списка; удаление и восстановление заказа меняют counts.tasks.
// Негатив: исполнитель → 403.
func (s *SoftDeleteLiveSuite) TestTS011_Summary()

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): раздел «Удалённые» — только администратору: исполнитель →
// 403 «Нет доступа» на списке (при любых параметрах, включая некорректные) и счётчиках; без
// сессии → 401.
func (s *SoftDeleteLiveSuite) TestTS033_AdminOnly()

// ---- SSR ----

// Экран /deleted (§8.1): администратору — 200, оболочка экрана и пункт меню «Удалённые»;
// исполнителю — страница отказа «Нет доступа» (403), пункта меню нет.
func (s *SoftDeleteLiveSuite) TestSSR_PageAndMenu()
```

---

Исходники образца: 78,641 символов · слепок: 9,834 · сжатие ×8.0
