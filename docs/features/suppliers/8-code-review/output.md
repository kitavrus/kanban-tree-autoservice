---
stage: 8-code-review
feature: suppliers
tier: S
verdict: APPROVED
high: 0
medium: 2
low: 6
---
# Code Review: suppliers — APPROVED
**Review level:** Slim (tier: S) + Sec 18 API-линза; режим spec-track merged (source of truth — `draft-spec.md` §3/§4/§6/§8)

База `18a0137` → HEAD `0cc63da`. Гейты (build/test/lint/integration/E2E 28 passed) — pre-condition, приняты от оркестратора, не перегонялись. Улика E2E: `9-validation/evidence/last-run.json` (`status: passed`), `e2e-run.txt` (chromium headed, 28 passed).

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[web/ssr/_shared/js/suppliers.js:234-243 (`openById`), :17 (`LOOKUP_LIMIT = 500`)] Прямая ссылка `?id=` ломается после 500 записей — мина под SUP-004 на общем dev-стенде.**
   Запасной поиск — один запрос `limit=500&offset=0` без фильтров; сортировка API — активные, затем архивные по `lower(name)`. E2E-записи архивируются, а не удаляются (`e2e/tests/suppliers.spec.ts:20-32`), имя начинается с `e2e-<RUN>`, `RUN = Date.now().toString(36)` (`e2e/fixtures/auth.ts:13`) — записи свежего прогона сортируются ПОСЛЕДНИМИ среди архивных. Прогон оставляет ~10 архивных записей → примерно через 50 прогонов шаг «прямая ссылка на архивного» (`e2e/tests/suppliers.spec.ts:314-317`) начнёт падать с ложным тостом «Поставщик не найден». То же в продукте при > 500 поставщиков.
   Как исправить (без нового endpoint, в рамках §6.1): в `openById` листать страницы по 500 (`offset += 500`), пока запись не найдена и `offset < total`. Альтернатива — `GET /api/v1/suppliers/:id`: это новое поведение вне спеки, нужен to-be DS (решение пользователя; вероятно понадобится в `transfers-partners`).
2. **[internal/server/middleware/auth.go:142; общий `StatusPages.Forbidden`; e2e/tests/suppliers.spec.ts:223-224] Тексты 403 расходятся со спекой, тест расхождение не видит.**
   Спека: API 403 `message: «Недостаточно прав»` (§6.1–§6.3), страница «Недостаточно прав» (§3 abuse 1, §8.1, TS-035). Факт: API отдаёт `"admin only"`, страница — «Нет доступа» (общие as-is компоненты; на «Нет доступа» завязаны тесты users/clients/catalog/equipment). SUP-003 проверяет только `403`, live-тест — только статус.
   Код фичи менять не нужно. Решение пользователя: (а) поправить `draft-spec.md` и мастер-спеку `transfers` под as-is («Нет доступа» / `admin only`) — фраза повторится в остальных 5 частях; либо (б) to-be DS на переименование в общих компонентах отдельной задачей. После решения — закрепить текст ассертом в SUP-003 и `TestTS020_ExecutorForbidden`.

## Незначительные замечания (можно в следующем PR)
1. **[e2e/tests/suppliers.spec.ts:129]** SUP-002: предикат `url.includes('/api/v1/suppliers') && status === 400` шире нужного — маскирует 400 на GET списка («Некорректный фильтр»). Сузить: `&& !url.includes('?')` (POST и PUT `/:id` — без query, GET списка — всегда с `?`).
2. **[web/ssr/_shared/js/suppliers.js:201-209 (`mutate`), :272-275, :266-267]** Пока запрос в полёте, клик по строке / «Добавить» открывает другую форму; по завершении `closeForm()` закроет её, тост «Сохранено» относится к предыдущей. Исправить: `if (state.busy) return;` в начале `openForm`.
3. **[web/ssr/_shared/js/suppliers.js:50-58]** Локальный тост. Допустимо (общего компонента нет, DS-052 — `transfers-core`); при реализации DS-052 вынести в `KT.toast` и заменить здесь.
4. **[internal/features/suppliers/handler.go:50-52]** `is_active: required` возвращается отдельно от остальных ошибок полей (PUT без `is_active` и с пустым `name` → только `is_active`). Как в warehouses; на экране недостижимо (JS всегда шлёт `is_active`).
5. **[docs/features/suppliers/0-setup/light-progress.md]** Последний чекбокс «Фактический прогон E2E» не закрыт при наличии улики — отметить `[x]` (оркестратор).
6. **[8-code-review/checks/live_suite.txt]** Чекер ответил «HTTP-слоя в модулях нет» — слепое пятно на плоском layout'е пакета (PASS по пропуску). По существу live-сьют есть: `internal/features/suppliers/suppliers_http_live_test.go` (`testutil.LiveSuite`, `app.Test` нет). Правка — в инструменте, не в коде.

## Оценка отступлений исполнителя
| # | Отступление | Оценка |
|---|---|---|
| 1 | JS поверх API вместо PRG | допустимо — соответствует спеке (§6 «Режим B: API + фронтенд», §8.1: тосты, debounce 300 мс, `?id=`/`?new=1`) |
| 2 | 403: «Нет доступа» вместо «Недостаточно прав» | вынести пользователю — MEDIUM 2 (код фичи не менять) |
| 3 | Нет `GET /:id`, поиск формы через `limit=500` | дефект — MEDIUM 1 (чинится пагинацией поиска без нового endpoint) |
| 4 | `invalid_text`; `q` > 200 → 400; clamp `limit`; `is_active` строго `true\|false` | допустимо: `invalid_text` закрывает 500 от PostgreSQL на NUL/битом UTF-8 (экран показывает общий текст поля); `q` 0–200 и `true/false` — буква §6.1/TS-020; clamp — трактовка «1–500». Рекомендация: дописать коды `invalid_text` и `q: too_long` в §6 спеки как as-built |
| 5 | Локальный тост | допустимо — LOW 3 |
| 6 | Нет slog мутаций | допустимо — DS-051 помечен «реализуется в `transfers-core`» |
| 7 | E2E архивирует, не удаляет | допустимо для MVP (удаления нет по DS-007), но в связке с п.3 даёт MEDIUM 1 |
| 8 | SUP-005 `allowHttpErrors` | допустимо — `url.endsWith('/api/v1/suppliers') && status === 400` (`suppliers.spec.ts:344`) пропускает только ответ без query и без `/:id` (POST создания); 5xx, PUT и GET списка не маскирует |

## Чеклист Slim
**Security (Sec 6)** — ок. `routes.go:13,21`: `RequireAPI`/`RequireSSR` + `RequireAdmin` на группе (executor 403, аноним 401 — `TestTS020_ExecutorForbidden`). SQL только параметрами, `ILIKE` через `sqlutil.EscapeLike` (тест `100%_`). Mass assignment: `SupplierCreateRequest` без `is_active`, неизвестное поле → 400. XSS: весь вывод через `esc()` / `textContent`. CSRF — общий middleware, `KT.api` шлёт `X-Csrf-Token`. Секретов/PII в логах нет. Новых зависимостей нет.
**Architecture** — ок. Плоский пакет `internal/features/suppliers` по образцу warehouses/stocktransfers (D43); SQL только в `repo.go`, handler без бизнес-логики; регистрация в `internal/server/router.go`. `google/uuid` — as-is паттерн проекта (`pkg/ids` нет).
**Error Handling** — ок. Только `pkg/errorspkg`; `23505` → 400 `name: duplicate` через `*pgconn.PgError`; `ErrNoRows` и не-uuid → 404 `SUPPLIER_NOT_FOUND`; мусорное тело → 400, не 500.
**Tests** — ок. Unit `validate_test.go`; LiveSuite — CRUD, валидация, гонка создания (ровно один 201), фильтры/конверт, права; миграционный тест up/down; E2E SUP-001…006 — happy, негатив формы, authz, overflow 375×812 в двух темах, concurrency (два контекста, LWW). POM + fixtures, `waitForTimeout` нет, данные через `uniq()`.
**Rollback** — ок. `0012_suppliers.down.sql` есть, цикл down/up проверен тестом; режим MVP.

## Sec 18 — API-контракт
- Nullable: все поля NOT NULL в DDL ↔ не-указатели в DTO ↔ строки/пустые строки на wire — согласовано с §4.1.
- Конверт списка `{items,total,limit,offset}` — ровно 4 ключа (тест); пустой список — `[]`, не `null`.
- URL kebab, JSON snake_case (api_conventions ✅). N+1 нет (count + один select).
- Ошибки §6: `required`/`too_long`/`duplicate`/`invalid`, `is_active: required`, `is_active: invalid`, 404 — возвращаются и покрыты. Расхождение — только текст 403 (MEDIUM 2).
- Breaking: новая поверхность, as-is не задет.

## Детерминированные чекеры
- go_naming, api_conventions, contract, date_bound, e2e_mode, rule_usage — ✅.
- `fixture_sync` FAIL — ложное (ищет `scripts/e2e`): `e2e/fixtures/auth.ts` обновлён в том же коммите `efce930`, что и миграция (wire-типы `Supplier*` + хелперы `AdminApi`), payload'ы типизированы.
- `e2e_drift` FAIL — ложное (путь + формат имени): TS-035 покрыт `SUP-001…SUP-006 (TS-035)`, TS-020 — `TestTS020_*` (unit + live + миграция).
- `fuzz_verdict` SKIP (tier S) — n/a. `live_suite` — LOW 6.

## Соответствие кода спеке
- [x] §4.1 DDL (типы, дефолты, CHECK, `suppliers_name_lower_uq`) — ДА
- [x] §6.1–§6.3 контракты, валидация, сортировка — ДА (кроме текста 403, MEDIUM 2)
- [x] §8.1 экран (фильтры, таблица, форма, тосты, блокировка кнопок, адрес формы) — ДА
- [x] §Impact «вне каталога фичи»: `internal/server/router.go`, `internal/ssr/menu.go`, `migrations/0012_*`, `e2e/*` — все в diff — ДА
- [x] Non-goals не реализованы (удаления нет, `supplier_id` в документах нет) — ДА
- Monitoring / Rollout / Runbook / Performance Budget / Swagger (`swagger:no`) — N/A

Наблюдение (не finding): постраничной навигации на экране нет — при > 50 записях строка «Показано 50 из N», остальное доступно поиском. Буква §8.1 (`limit=50&offset=0`) соблюдена; если нужно листание — to-be DS.

## Статус фаз
- [ ] Все чекбоксы `light-progress.md` закрыты — НЕТ: открыт только «Фактический прогон E2E» (зона оркестратора, улика есть — LOW 5).

## Итог
APPROVED: блокеров нет. До мержа желательно закрыть MEDIUM 1 (правка в одной функции) и получить решение пользователя по MEDIUM 2.
