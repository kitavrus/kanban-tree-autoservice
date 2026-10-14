# Идиомы проекта — слепок с фичи `warehouses`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/warehouses`

## Образцы слоёв (по одному файлу на слой; ещё 8 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/warehouses/repo.go`

```go
package warehouses

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Warehouse — склад (repo-модель, DS-001).
type Warehouse struct {
	ID        string
	Name      string
	Position  int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (

const columns = `id, name, position, is_active, created_at, updated_at`

func scan(row pgx.Row) (*Warehouse, error)

type ListParams struct {
	IncludeArchived bool
	Limit, Offset   int
}

// List — порядок по position (склад по умолчанию — первый активный, DS-005).
func (r *Repo) List(ctx context.Context, p ListParams) ([]Warehouse, int, error)

func (r *Repo) Get(ctx context.Context, id string) (*Warehouse, error)

// Create — position = max+1 (в конец списка); 23505 → errNameTaken/errNameArchived.
func (r *Repo) Create(ctx context.Context, in Input) (*Warehouse, error)

// dbtx — пул или транзакция (архив склада идёт в tx под блокировкой активных, D47).
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Update — переименование + is_active; ErrNoRows → ErrNotFound (сервис маппит).
func (r *Repo) Update(ctx context.Context, id string, in Input) (*Warehouse, error)

// UpdateIn — Update в заданной tx/пуле. Классификация 23505 — отдельным запросом на пуле
// (mapNameConflict), поэтому работает и после аборта tx.
func (r *Repo) UpdateIn(ctx context.Context, q dbtx, id string, in Input) (*Warehouse, error)

// SetActive — SSR archive/restore, без перезаписи имени.
func (r *Repo) SetActive(ctx context.Context, id string, active bool) error

// SetActiveIn — SetActive в заданной tx/пуле.
func (r *Repo) SetActiveIn(ctx context.Context, q dbtx, id string, active bool) error

// mapNameConflict — 23505 (warehouses_name_lower_uq) → errNameTaken | errNameArchived.
func (r *Repo) mapNameConflict(ctx context.Context, name string, err error) error
```

### `internal/features/warehouses/warehouses_http_live_test.go`

```go
//go:build integration

package warehouses_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// WarehousesLiveSuite (§TD-8): справочник складов (docs/features/warehouses/draft-spec.md
// DS-001/DS-010/DS-011) — TS-002 (CRUD, дубль имени вкл. архивные → 400 fields[name],
// переименование, архив), TS-014 (executor → 403 API и не видит «Склады» в сайдбаре), SSR.
type WarehousesLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestWarehousesLiveSuite(t *testing.T)

func (s *WarehousesLiveSuite) SetupSuite()

type warehouseDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	IsActive bool   `json:"is_active"`
}

type warehouseListDTO struct {
	Items  []warehouseDTO `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func (s *WarehousesLiveSuite) fields(body []byte) map[string]string

func (s *WarehousesLiveSuite) list(query string) warehouseListDTO

func (s *WarehousesLiveSuite) create(name string) warehouseDTO

func (s *WarehousesLiveSuite) byName(l warehouseListDTO, name string) (warehouseDTO, bool)

// Сид миграции 0008: «Алматы» (1), «Москва» (2); конверт списка §API-1.
func (s *WarehousesLiveSuite) TestSeed_AlmatyMoscow_Envelope()

// TS-002: создать, дубль имени (регистр/пробелы, вкл. архивные) → 400 fields[name];
// переименовать; архивировать.
func (s *WarehousesLiveSuite) TestTS002_CRUD_DuplicateRenameArchive()

// Валидация, 404, мусор → 400 (не 500), удаления нет (DS-001).
func (s *WarehousesLiveSuite) TestValidation_NotFound_NoDelete()
```

---

Исходники образца: 39,313 символов · слепок: 3,881 · сжатие ×10.1
