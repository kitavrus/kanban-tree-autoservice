# Phase 04: Stock API + бейдж «Остатков» в меню + DI (роутер, удаление `home`)

**Layer:** backend
**Commit**: `feat(phase-5): stock API, бейдж меню, регистрация board в роутере (удаляет home)`

> **Отклонение от рекомендованной нарезки (обоснованно):** удаление `internal/features/home`/`web/ssr/home` выполняется В ЭТОЙ фазе, не в фазе 08 — `GET /` не может быть зарегистрирован дважды (`home.Register` и `board.Register` конфликтуют на одном пути), поэтому удаление `home` неразрывно с DI-фазой.

## Files to CREATE
- `internal/features/board/stock_dto.go` — `StockRow`, `StockListResponse`, `StockSummaryResponse`, `StockAdjustRequest` (design-go-layers.md §DTO Stock) — `Stock`/`MinStock`/`Pending`/`Forecast` типа `numeric.Amount` (helper-binding — НЕ `float64`)
- `internal/features/board/stock_repo.go` — `StockRepo`: `List` (design-sql.md §Stock List Query, использует §Stock Pending CTE), `Summary` (§Stock Summary Query), `Adjust` (§Stock Adjust Query)
- `internal/features/board/stock_service.go` — `StockService`: `List`, `Summary`, `Adjust`, `NavBadges(ctx) map[string]int` (ошибка → `slog.Warn` + `nil`, ADR-004)
- `internal/features/board/stock_handler.go` — `List` (`scope`,`q`,`limit` default 200 max 1000,`offset`), `Summary`, `Adjust` (admin)
- `internal/features/board/stock_http_live_test.go` (`StockLiveSuite`, `testutil.LiveSuite`) — `TestStockLiveSuite_List_ScopeAndKPI` (LV-012), `TestStockLiveSuite_Adjust_LogsManualMovement` (LV-013), `TestStockLiveSuite_SSR_BoardAndStockPages200` (LV-014, каркас-стаб), `TestStockLiveSuite_Executor_AdjustForbidden_ReadAllowed` (LV-043)
- `internal/features/board/ssr.go` — `Root = "web/ssr/board/templates"`; `board` → `Page(c,"board",BoardVM{Shell,ViewerID,IsAdmin})`; `stock` → `Page(c,"stock",StockVM{Shell})` (design-go-layers.md §Handler SSR)
- `web/ssr/board/templates/pages/board.html` — СТАБ каркаса: `{{block "scripts"}}` пустой, минимальный `#board` div (наполнение — фаза 06)
- `web/ssr/board/templates/pages/stock.html` — СТАБ каркаса (наполнение — фаза 07)
- `web/ssr/_shared/js/nav-badge.js` — `KT.refreshNavBadge()` → `GET /api/v1/stock/summary` → обновляет `[data-nav-badge="/stock"]` (ADR-004)

## Files to MODIFY
- `internal/features/board/routes.go` — расширить `Register`: группа `/stock` (List/Summary/Adjust-admin); `Handlers` — добавить `Stock *StockHandler`; `if s == nil { return }` + `app.Get("/", a.RequireSSR(), s.board)`, `app.Get("/stock", a.RequireSSR(), s.stock)`
- `internal/ssr/menu.go` — `NavItem` — добавить поле `Badge int`; `BuildMenu(enabled []string, isAdmin bool, badges map[string]int) []NavItem`
- `internal/ssr/shell.go` — `type BadgeSource interface{ NavBadges(ctx context.Context) map[string]int }`; `func (b ShellBuilder) WithBadges(src BadgeSource) ShellBuilder` (копия); `Build` — `badges := b.src.NavBadges(c.Context())` (если `src != nil`) → `BuildMenu(enabled, isAdmin, badges)`
- `internal/ssr/shell_test.go` — добавить `TestShellBuilder_WithBadges_PassesBadgesToMenu`, `TestShellBuilder_NoBadgeSource_BadgeZero`
- `web/ssr/_layout/templates/layouts/base.html` — у пункта `/stock`: `<span class="badge rounded-pill text-bg-danger ms-auto" data-nav-badge="/stock" data-testid="nav-badge-stock" {{if not .Badge}}hidden{{end}}>{{.Badge}}</span>` (span есть всегда); подключить `nav-badge.js` после `api.js`
- `internal/server/router.go` — `NewShellBuilder("/", "/stock", "/users", "/catalog", "/clients").WithBadges(stockService)`; вызвать `board.Register(app, api, auth, boardHandlers, boardSSR)`; удалить регистрацию `home` и импорт `internal/features/home`

## Files to DELETE
- `internal/features/home/` (весь пакет)
- `web/ssr/home/` (весь каталог шаблонов)

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Live (`StockLiveSuite`, TD-8): `TestStockLiveSuite_List_ScopeAndKPI` (LV-012), `TestStockLiveSuite_Adjust_LogsManualMovement` (LV-013), `TestStockLiveSuite_SSR_BoardAndStockPages200` (LV-014), `TestStockLiveSuite_Executor_AdjustForbidden_ReadAllowed` (LV-043)
- Unit: `TestShellBuilder_WithBadges_PassesBadgesToMenu`, `TestShellBuilder_NoBadgeSource_BadgeZero`

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (`BASE=be769e5` → FULL-триггер по миграции 0005, прогнан `./...`)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `stock_http_live_test.go` (`testutil.LiveSuite`) — LV-012, LV-013, LV-014, LV-043 зелёные (TD-8) (+ TD-4 адверсариал: query-валидация, adjust-валидация, 404, 401)
- [x] `stock_dto.go`: `Stock`/`MinStock`/`Pending`/`Forecast` — `pkg/numeric.Amount` (helper-binding, НЕ `float64`)
- [x] AC-015 (Stock API backend), AC-016 (`NavBadges`→бейдж) покрыты
- [x] `internal/features/home`, `web/ssr/home` удалены; `internal/server/router.go` не импортирует `home`; `GET /` отдаёт board-каркас (200)
- [x] Ручной smoke: `GET /` и `GET /stock` — 200 (каркас-стаб допустим, полный UI — фазы 06/07) — покрыто автоматически `TestStockLiveSuite_SSR_BoardAndStockPages200` (реальный бинарь+БД); отдельный ручной прогон против dev-стенда не делался (dev-стенд не трогать по инструкции фазы)
- [x] `data-nav-badge="/stock"` span в `base.html` всегда рендерится (hidden при 0) — проверено в `TestStockLiveSuite_SSR_BoardAndStockPages200` (видимый при risk_count>0); нулевой кейс — юнит `TestShellBuilder_NoBadgeSource_BadgeZero`/`TestBuildMenu` badges-кейс в `internal/ssr`
