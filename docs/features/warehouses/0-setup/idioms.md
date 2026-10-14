# Идиомы проекта — слепок с фичи `catalog`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/catalog`

## Образцы слоёв (по одному файлу на слой; ещё 29 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/catalog/services_http_live_test.go`

```go
//go:build integration

package catalog_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ServicesLiveSuite (§TD-8, подфаза B task.md): CRUD услуг, комплект (дубли/qty≤0/
// архивный товар → 400), удаление товара из комплекта (PRODUCT_IN_USE) сквозным путём
// через API услуг, «по факту», права (D19), IDOR, мусор → 400, CSRF, SSR.
type ServicesLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestServicesLiveSuite(t *testing.T)

func (s *ServicesLiveSuite) SetupSuite()

type svcProductDTO struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Price     amount `json:"price"`
	Qty       amount `json:"qty"`
}

type serviceDTO struct {
	ID         string          `json:"id"`
	CategoryID string          `json:"category_id"`
	Name       string          `json:"name"`
	Price      *amount         `json:"price"`
	IsActive   bool            `json:"is_active"`
	Products   []svcProductDTO `json:"products"`
}

// errOf/fields — копии хелперов CatalogLiveSuite (тот же пакет catalog_test, но другой
// приёмник suite — методы не наследуются между независимыми типами suite.Suite).
func (s *ServicesLiveSuite) errOf(body []byte) apiErr

func (s *ServicesLiveSuite) fields(body []byte) map[string]string

// newCategory — копия хелпера CatalogLiveSuite (тот же пакет catalog_test, но другой
// приёмник suite — методы не наследуются между независимыми типами suite.Suite).
func (s *ServicesLiveSuite) newCategory(kind, name string) categoryDTO

func (s *ServicesLiveSuite) newProductB(categoryID string, patch map[string]any) productDTO

func serviceBody(categoryID string, patch map[string]any) map[string]any

func (s *ServicesLiveSuite) newService(categoryID string, patch map[string]any) serviceDTO

// ---- happy path: категория+услуга+комплект CRUD, re-fetch-after-write ----

func (s *ServicesLiveSuite) TestServiceCreateGetList_WithProducts()

// ---- «по факту» — price отсутствует/null → null в ответе ----

func (s *ServicesLiveSuite) TestService_PriceByFact_NullBothWays()

// ---- D10: дубли товара в комплекте объединяются суммой qty ----

func (s *ServicesLiveSuite) TestService_DuplicateProducts_QtySummed()

// ---- qty ≤ 0 → 400; архивный/несуществующий товар → 400 по полю строки ----

func (s *ServicesLiveSuite) TestService_Validation_QtyAndProductRow()

// ---- code review Медиум#2: товар, УЖЕ бывший в комплекте услуги, архивированный (ровно
// поток CT-005: PRODUCT_IN_USE → архив вместо удаления) — пересохранение услуги с ЭТОЙ ЖЕ
// строкой должно проходить; НОВАЯ строка с архивным товаром (которого раньше не было в
// комплекте) — 400. task.md: «архивный товар не предлагается ПРИ ДОБАВЛЕНИИ», не при
// пересохранении уже существующей. ----

func (s *ServicesLiveSuite) TestService_Update_ArchivedProductInComplect_Grandfathered()

// ---- code review Блокер 1: границы NUMERIC(12,2)/(12,3) для услуг → 400 по своему полю,
// не 500. ----

func (s *ServicesLiveSuite) TestService_NumericBoundaries400()

// ---- невалидная категория (не существует / не тот kind) → 400 ----

func (s *ServicesLiveSuite) TestService_InvalidCategory400()

// ---- удаление товара из комплекта услуги — 409 PRODUCT_IN_USE сквозным путём через API
// услуг (task.md: «сквозной тест через API услуг»); архивирование вместо удаления работает,
// свободный товар удаляется. ----

func (s *ServicesLiveSuite) TestService_ProductInComplect_DeleteConflict_ArchiveWorks()

// ---- удаление/архив услуги, is_active обязателен на PUT ----

func (s *ServicesLiveSuite) TestService_Update_ArchiveRestore_Delete()

// ---- IDOR/битый uuid → 404 ----

func (s *ServicesLiveSuite) TestBadOrMissingServiceID_404()

// ---- мусор в теле → 400, не 500 ----

func (s *ServicesLiveSuite) TestService_Validation_GarbageNot500()
```

### `internal/features/catalog/services_repo.go`

```go
package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Service — repo-модель услуги (без комплекта — грузится отдельно ServiceProductRow,
// re-fetch-after-write). Price == nil — «по факту» (NULL в БД).
type Service struct {
	ID         string
	CategoryID string
	Name       string
	Price      *string
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ServiceProductRow — строка комплекта с денормализованными полями товара (JOIN products).
type ServiceProductRow struct {
	ProductID string
	Name      string
	Unit      string
	Price     string
	Qty       string
}

type ServiceRepo struct {
	pool *pgxpool.Pool
}

func NewServiceRepo(pool *pgxpool.Pool) *ServiceRepo { return &ServiceRepo{pool: pool} }

var (
	// errServiceInUse — 23503 на subtasks_service_id_fkey (миграция 0005): заказ ссылается
	// на услугу — удалить нельзя (ADR-005, классификация по имени constraint, не по тексту).

// errServiceProductInvalid — строка комплекта ссылается на несуществующий/архивный товар
// (task.md: «архивный/несуществующий товар → 400 по полю строки»).
type errServiceProductInvalid struct{ Index int }

func (e *errServiceProductInvalid) Error() string

const serviceColumns = `id, category_id, name, price::text, is_active, created_at, updated_at`

func scanService(row pgx.Row) (*Service, error)

type ServiceListParams struct {
	CategoryID      string
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — q ищет по названию услуги (в отличие от товаров — нет артикула/коробки/места).
func (r *ServiceRepo) List(ctx context.Context, p ServiceListParams) ([]Service, int, error)

func (r *ServiceRepo) Get(ctx context.Context, id string) (*Service, error)

// Products — комплект услуги, порядок по position (SSR/API — «Название ×qty»).
func (r *ServiceRepo) Products(ctx context.Context, serviceID string) ([]ServiceProductRow, error)

// ProductsBatch — комплекты сразу для набора услуг (Sec 18/Серьёзное 1 — N+1: раньше
// ServiceService.List дёргал Products() на КАЖДУЮ услугу страницы отдельно, до 501
// round-trip на limit=500; здесь — один запрос WHERE service_id = ANY($1::uuid[]),
// группировка в Go, эталон — clients phonesBatch).
func (r *ServiceRepo) ProductsBatch(ctx context.Context, serviceIDs []string) (map[string][]ServiceProductRow, error)

// existingServiceProductIDs — product_id, УЖЕ присутствующие в комплекте этой услуги (до
// правки) — грандфазер-набор для checkProducts на Update (code review Medium#2).
func existingServiceProductIDs(ctx context.Context, tx pgx.Tx, serviceID string) (map[string]bool, error)

// validActiveProductIDs — активные товары из набора id, одним запросом (ANY($1::uuid[])).
func validActiveProductIDs(ctx context.Context, tx pgx.Tx, ids []string) (map[string]bool, error)

// checkProducts — первая строка со ссылкой на несуществующий/архивный товар → индекс.
// grandfathered — product_id, уже присутствовавшие в ТЕКУЩЕМ комплекте этой услуги (nil
// на Create — там прошлого комплекта не существует): для них is_active НЕ проверяется —
// существование уже гарантировано FK service_products.product_id (code review Medium#2:
// task.md запрещает архивный товар «при добавлении» в комплект, не при пересохранении уже
// существующей строки — иначе PRODUCT_IN_USE → архив делает услугу несохраняемой вовсе).
func checkProducts(ctx context.Context, tx pgx.Tx, products []ServiceProductInput, grandfathered map[string]bool) error

// replaceServiceProducts — DELETE+INSERT всего комплекта (проще диффа, product_id RESTRICT
// не мешает — FK защищает удаление самого товара, не строк service_products).
func replaceServiceProducts(ctx context.Context, tx pgx.Tx, serviceID string, products []ServiceProductInput) error

func servicePriceArg(in ServiceInput) *string

// Create — категория (kind=service) + все товары комплекта активны, одной транзакцией.
// in.Products — ИСХОДНЫЕ (не смерженные) строки: checkProducts проверяет существование/
// активность по ним, чтобы errServiceProductInvalid.Index указывал на строку, которую
// реально видит SSR-форма/API-клиент (code review fix-loop 2 Medium); merged — уже
// продедуплицированный список (services_service.prepareProducts) для записи в БД.
func (r *ServiceRepo) Create(ctx context.Context, in ServiceInput, merged []ServiceProductInput) (*Service, error)

// Update — полная замена карточки + комплекта (DELETE+INSERT), одной транзакцией.
// in.Products/merged — см. Create выше (исходные для checkProducts, смерженные для записи).
func (r *ServiceRepo) Update(ctx context.Context, id string, in ServiceInput, merged []ServiceProductInput) (*Service, error)

func (r *ServiceRepo) SetActive(ctx context.Context, id string, active bool) error

// Delete — CASCADE подчищает service_products; 23503 на subtasks_service_id_fkey
// (миграция 0005 — заказ ссылается на услугу) → errServiceInUse. Классификация по имени
// constraint (не по тексту, ADR-005): «любой FK» здесь не годится — у услуги есть и
// service_products (CASCADE, никогда не 23503), и в будущем другие ссылки.
func (r *ServiceRepo) Delete(ctx context.Context, id string) error
```

---

Исходники образца: 174,563 символов · слепок: 9,181 · сжатие ×19.0
