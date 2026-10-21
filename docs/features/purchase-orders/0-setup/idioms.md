# Идиомы проекта — слепок с фичи `suppliers`

> СГЕНЕРИРОВАНО `tools/idiom_digest.py` из живого кода. Показывает, КАК в этом
> проекте пишется слой: структура, сигнатуры, контракты, SQL. **Тела функций
> вырезаны намеренно** — там доменная специфика образца, твоей фиче она не нужна.
> Нужна конкретная реализация — открой файл точечно, но не вычитывай фичу целиком
> (§context-mode: обзорное чтение — в sandbox).

## Раскладка каталогов

- `.claude/worktrees/agitated-fermat-76a73a/internal/features/suppliers`
- `.claude/worktrees/charming-brahmagupta-e30492/internal/features/suppliers`
- `.claude/worktrees/determined-mestorf-7fadc2/internal/features/suppliers`
- `.claude/worktrees/frosty-lumiere-aba02a/internal/features/suppliers`
- `.claude/worktrees/sleepy-leakey-04a583/internal/features/suppliers`
- `.claude/worktrees/sleepy-rhodes-c4ce4b/internal/features/suppliers`
- `.claude/worktrees/sweet-satoshi-832a66/internal/features/suppliers`
- `.claude/worktrees/upbeat-margulis-bae97c/internal/features/suppliers`
- `internal/features/suppliers`

## Образцы слоёв (по одному файлу на слой; ещё 102 однотипных файлов не включены — идиома у слоя одна)

### `.claude/worktrees/sweet-satoshi-832a66/internal/features/suppliers/soft_delete.go`

```go
package suppliers

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Мягкое удаление поставщиков (docs/features/soft-delete-warehouses-suppliers/draft-spec.md
// §4.2, §6.3, §6.4, часть 6 мастер-спеки soft-delete): удаление с причиной, восстановление,
// служебная уборка. Архив (is_active) и удаление независимы (DS-009).

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// DeleteResponse — ответ DELETE /api/v1/suppliers/:id (§6.3). DeletedAt — UTC.
type DeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RestoreResponse — ответ POST /api/v1/suppliers/:id/restore (§6.4).
type RestoreResponse struct {
	ID string `json:"id"`
}

// ErrPurgeInUse — служебная уборка: на поставщика ссылаются документы (§6.5).
var ErrPurgeInUse = errorspkg.Conflict("Запись используется")

var (

// SoftDelete — мягкое удаление поставщика: один UPDATE с условием «ещё не удалён»; 0 строк →
// ErrNotFound (второй одновременный запрос — тоже, DS-024). Документы поставщика в любом статусе
// не читаются и не мешают: UPDATE вместо DELETE, внешний ключ transfers.supplier_id не
// срабатывает (DS-014). is_active не меняется.
func (r *Repo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error)

// Restore — восстановление поставщика: один UPDATE с условием «удалён» стирает три поля удаления,
// is_active прежний (DS-009). Название за время удаления мог занять действующий поставщик —
// нарушение частичного уникального индекса → errRestoreNameTaken (DS-017); индекс же решает
// гонку «создать/переименовать ↔ восстановить». 0 строк: поставщика нет → ErrNotFound,
// поставщик действует → errNotDeleted.
func (r *Repo) Restore(ctx context.Context, id string) error

// Purge — физическое удаление поставщика (служебная уборка автотестов, DS-027): действующего
// либо удалённого; на него ссылаются документы (FK RESTRICT) → errPurgeInUse.
func (r *Repo) Purge(ctx context.Context, id string) error

// Delete — МЯГКОЕ удаление поставщика (§6.3): только admin — роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020); документы не мешают (DS-014).
func (s *Service) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error)

// Restore — восстановление поставщика (§6.4): только admin; название занято действующим
// поставщиком → 409 NAME_TAKEN; не удалён → 409 NOT_DELETED.
func (s *Service) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error)

// Purge — вид suppliers служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард
// admin — в пакете board, только при TEST_PURGE_ENABLED=true, DS-027): вид записи не
// раскрывается (404 NOT_FOUND); на поставщика ссылаются документы → 409 CONFLICT.
func (s *Service) Purge(ctx context.Context, id string) error

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI — principal всегда
// не nil).
func currentActor(c fiber.Ctx) Actor

// Delete — DELETE /api/v1/suppliers/:id (admin), тело {reason}: 200 + id, deleted_at.
func (h *Handler) Delete(c fiber.Ctx) error

// Restore — POST /api/v1/suppliers/:id/restore (admin), тела нет.
func (h *Handler) Restore(c fiber.Ctx) error
```

### `.claude/worktrees/sweet-satoshi-832a66/internal/features/suppliers/suppliers_soft_delete_http_live_test.go`

```go
//go:build integration

package suppliers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// Мягкое удаление поставщиков (docs/features/soft-delete-warehouses-suppliers/draft-spec.md
// §4.2, §6.3, §6.4, §6.5, §11) — методы SuppliersLiveSuite (§TD-8: боевой бинарь, реальный
// логин): TS-032 (удаление при наличии документов, пометка supplier_is_deleted, название
// свободно, NAME_TAKEN при восстановлении), TS-033 (права), причина удаления, служебная уборка и
// гонки §3. Документы создаются через API перемещений того же сервера.

// ---- wire-типы (§TD-6.2) ----

type sdDeleteDTO struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type sdRestoreDTO struct {
	ID string `json:"id"`
}

type sdErrDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// sdDocDTO — документ «Перемещение»: поставщик и признак его удаления (§6.5).
type sdDocDTO struct {
	ID                int64  `json:"id"`
	Status            string `json:"status"`
	Version           int    `json:"version"`
	SupplierID        string `json:"supplier_id"`
	SupplierName      string `json:"supplier_name"`
	SupplierIsDeleted bool   `json:"supplier_is_deleted"`
}

// ---- помощники ----

func sdMark() string { return uuid.NewString()[:8] }

func (s *SuppliersLiveSuite) sdErr(raw []byte) sdErrDTO

func (s *SuppliersLiveSuite) sdScalar(q string, args ...any) string

func (s *SuppliersLiveSuite) sdAdminID() string

// supState — «удалён|is_active|причина|кто удалил» строки поставщика; "" — строки нет.
func (s *SuppliersLiveSuite) supState(id string) string

func (s *SuppliersLiveSuite) deleteSup(b *testutil.Browser, id string, reason any) (int, sdDeleteDTO, []byte)

func (s *SuppliersLiveSuite) mustDeleteSup(id string)

func (s *SuppliersLiveSuite) restoreSup(b *testutil.Browser, id string) (int, []byte)

// listedSup — поставщик виден в списке при фильтре архива isActive ("" — все).
func (s *SuppliersLiveSuite) listedSup(id, isActive string) bool

// sdProduct — товар (SQL-фикстура каталога) для строк документов.
func (s *SuppliersLiveSuite) sdProduct() string

// supply — документ «поставщик → склад» (Алматы) через API перемещений; send — сразу «в пути».
func (s *SuppliersLiveSuite) supply(supplierID, productID string, send bool) (int, sdDocDTO, []byte)

func (s *SuppliersLiveSuite) mustSupply(supplierID, productID string, send bool) sdDocDTO

func (s *SuppliersLiveSuite) doc(id int64) sdDocDTO

func (s *SuppliersLiveSuite) sdExecutor(login string) *testutil.Browser

// ---- TS-032 ----

// TS-032 (DS-014, DS-026): поставщик с документами в разных статусах («Заявка», «В пути»,
// «Отменён») удаляется — 200 {id, deleted_at}; отметка «кто, когда, почему» записана, причина
// обрезана, признак архива не изменился; документы остаются и отдают supplier_is_deleted = true
// (и название поставщика), у документа действующего поставщика — false. Для обычных адресов
// поставщика больше нет: список при любом фильтре архива, PUT, повторное удаление — 404.
// Негатив: удалённый поставщик в новом документе → 400 supplier_id: not_found.
func (s *SuppliersLiveSuite) TestTS032_DeleteSupplierWithDocuments()

// TS-032 (DS-017, DS-009; §6.4): название удалённого поставщика свободно — создание и
// переименование с тем же названием проходят; восстановление при занятом названии → 409
// NAME_TAKEN, поставщик остаётся удалённым; после переименования действующего — 200 {id}, отметка
// стёрта, документ снова без пометки. Повтор восстановления → 409 NOT_DELETED; несуществующий →
// 404. Поставщик, удалённый из архива, возвращается в архив. Негатив (as-is): название
// действующего поставщика при создании по-прежнему занято — 400 name: duplicate.
func (s *SuppliersLiveSuite) TestTS032_NameFreeAfterDelete_Restore()

// ---- TS-033 ----

// TS-033 (DS-003, abuse case 1): исполнитель → 403 «Нет доступа» на удалении и восстановлении —
// до поиска записи (существующий, удалённый, несуществующий и битый идентификатор) и до проверки
// причины; поставщик не изменён. Негатив: без сессии → 401.
func (s *SuppliersLiveSuite) TestTS033_SoftDeleteAdminOnly()

// ---- причина удаления ----

// DS-020 (abuse case 5): причина обязательна — нет тела, пусто, пробелы → 400 reason: required;
// 501 символ → too_long; NUL → invalid_text; поставщик не удалён. 500 символов — допустимо.
func (s *SuppliersLiveSuite) TestDeleteReasonValidation()

// ---- служебная уборка ----

// DS-027 (§6.5): DELETE /api/v1/test/purge/suppliers/:id (TEST_PURGE_ENABLED=true) — действующий и
// мягко удалённый поставщик стираются физически; поставщик, на которого ссылается документ (в том
// числе когда сам поставщик мягко удалён), → 409 CONFLICT «Запись используется» и остаётся; нет
// поставщика либо битый идентификатор → 404 NOT_FOUND. Негатив: исполнитель → 403.
func (s *SuppliersLiveSuite) TestPurge_Suppliers()

// ---- гонки (§3 «Конкурентность») ----

func (s *SuppliersLiveSuite) lockWaiters() int

type raceResult struct {
	status int
	raw    []byte
}

// race — запрос req уходит, пока тест держит блокировку в tx; по pg_stat_activity тест
// дожидается, что запрос встал в ожидание, затем фиксирует tx и возвращает ответ. Без sleep.
func (s *SuppliersLiveSuite) race(hold func(ctx context.Context) (commit func()), req func() (int, []byte)) raceResult

// DS-024 (§3 п. 2): удаление поставщика идёт (отметка поставлена, транзакция не зафиксирована) —
// второй запрос удаления обязан дождаться и получить 404 SUPPLIER_NOT_FOUND; отметка первого не
// перезаписана (UPDATE с условием «ещё не удалён»).
func (s *SuppliersLiveSuite) TestRace_SecondDeleteGets404()

// §3 п. 4 (DS-017): создание поставщика с названием удалённого идёт (строка вставлена,
// транзакция не зафиксирована) — восстановление обязано дождаться на частичном уникальном индексе
// и получить 409 NAME_TAKEN: ровно один из двух получает название.
func (s *SuppliersLiveSuite) TestRace_RestoreWaitsForCreateSameName()

// §3 п. 4 (DS-017): восстановление поставщика идёт (отметка стёрта, транзакция не
// зафиксирована) — создание поставщика с тем же названием обязано дождаться на индексе и получить
// 400 name: duplicate (as-is ответ создания).
func (s *SuppliersLiveSuite) TestRace_CreateWaitsForRestoreSameName()
```

---

Исходники образца: 563,026 символов · слепок: 9,946 · сжатие ×56.6
