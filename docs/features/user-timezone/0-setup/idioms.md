# Идиомы проекта — слепок с фичи `users`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `internal/features/users`

## Образцы слоёв (по одному файлу на слой; ещё 7 однотипных файлов не включены — идиома у слоя одна)

### `internal/features/users/repo.go`

```go
package users

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

type User struct {
	ID        string
	Login     string
	Name      string
	Role      string
	Phone     string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// ActiveTasks — заполняется только в List (nil в остальных запросах).
	ActiveTasks *int
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (

const columns = `id, login, name, role, phone, is_active, created_at, updated_at`

// activeTasksSQL — «Активных заказов» в списке сотрудников (как в прототипе): заказы вне
// финальной колонки, где сотрудник — ответственный заказа или исполнитель незавершённой
// услуги. Коррелирован с users.id внешнего запроса.
const activeTasksSQL = `(SELECT count(*) FROM tasks t JOIN columns c ON c.id = t.column_id

func scan(row pgx.Row) (*User, error)

// ListParams — фильтры + сортировка + пагинация. Sort/Order приходят из сервиса УЖЕ
// прошедшими whitelist: склеиваются в SQL как идентификаторы.
type ListParams struct {
	Q        string // подстрока в ФИО / логине / телефоне, регистронезависимо
	Role     string // "" = любая
	IsActive *bool  // nil = любой статус
	Sort     string
	Order    string // ASC | DESC
	Limit    int
	Offset   int
}

func (r *Repo) List(ctx context.Context, p ListParams) ([]User, int, error)

// GetByID — id уже провалидирован как uuid сервисом.
func (r *Repo) GetByID(ctx context.Context, id string) (*User, error)

func (r *Repo) Create(ctx context.Context, in Input, passwordHash string) (*User, error)

// UpdateCheck — проверка перед записью внутри транзакции: before — карточка до правки,
// otherActiveAdmins — сколько активных админов, кроме правимого.
type UpdateCheck func(before *User, otherActiveAdmins int) error

// Update — полная замена карточки в транзакции. Строки всех активных админов и
// правимого пользователя блокируются (FOR UPDATE, в порядке id — без дедлоков), поэтому
// два параллельных «разжалования» админов не оставят систему без администратора (D29).
// passwordHash nil — пароль не меняется. Возвращает карточку до и после.
func (r *Repo) Update(ctx context.Context, id string, in Input, passwordHash *string, check UpdateCheck) (before, after *User, err error)

func mapUnique(err error) error
```

### `internal/features/users/users_http_live_test.go`

```go
//go:build integration

package users_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// UsersLiveSuite (§TD-8): сотрудники — API и SSR-экран, права, D29, сессии.
type UsersLiveSuite struct {
	testutil.LiveSuite
	admin *testutil.Browser
}

func TestUsersLive(t *testing.T)

func (s *UsersLiveSuite) SetupSuite()

type user struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Phone    string `json:"phone"`
	IsActive bool   `json:"is_active"`
}

type apiErr struct {
	Code    string `json:"code"`
	Details *struct {
		Fields []struct{ Name, Message string } `json:"fields"`
	} `json:"details"`
}

func (s *UsersLiveSuite) errOf(body []byte) apiErr

func (s *UsersLiveSuite) fields(body []byte) map[string]string

func (s *UsersLiveSuite) create(b *testutil.Browser, body map[string]any) (int, user, []byte)

func (s *UsersLiveSuite) put(b *testutil.Browser, id string, body map[string]any) (int, []byte)

func card(u user, patch map[string]any) map[string]any

func (s *UsersLiveSuite) newExecutor(login string) user

// ---- happy path ----

func (s *UsersLiveSuite) TestCreateGetListUpdate()

func (s *UsersLiveSuite) TestListEnvelopeAndPaging()

// «Активных заказов» (как в прототипе): вне финальной колонки, сотрудник — ответственный
// заказа или исполнитель незавершённой услуги; заказ считается один раз.
func (s *UsersLiveSuite) TestList_ActiveTasks()

// ---- authz ----

func (s *UsersLiveSuite) TestExecutorAndAnonymousHaveNoAccess()

func (s *UsersLiveSuite) TestRoleChangeAppliesWithoutRelogin()

// ---- validation / garbage → 400, not 500 ----

func (s *UsersLiveSuite) TestCreate_Validation()
```

---

Исходники образца: 47,838 символов · слепок: 4,288 · сжатие ×11.2
