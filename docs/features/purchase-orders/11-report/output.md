# Отчёт: purchase-orders — «Закупки по дефициту» и кабинет поставщика

**Вердикт ревью:** APPROVED (цикл 2; high 0 / medium 1 / low 5) — `8-code-review/output.md`
**Ветка:** `feat/purchase-orders` (не запушена, PR не открыт)
**Трек:** light (один агент; полный /pipeline не выполнялся — осознанный выбор пользователя при Tier L)
**scaffold:** manifest (z_scaffold `--from-draft-spec` → ручная правка манифеста; view kind `list-with-items`; overlay: нет, добавлен `.claude/scaffold-layout.json` с `e2e_dir: e2e`)
**Вход агента:** spec-slice light, 111K → 95K (−15%)

## Что сделано

Новый раздел «Закупки»: заказ поставщику или список «Базар (сами)», строки которого
предлагаются из дефицита («не хватает под заказы» + «ниже минимума», подсказка «уже в открытом
заказе» — у исполнителя только по своим заказам). Статусы draft → sent → ready → picked_up,
плюс cancelled (с причиной) и мягкое удаление черновика с восстановлением в «Удалённых».
Новая роль `supplier`: учётки создаёт админ на экране «Пользователи», поставщик заперт в
кабинете `/supplier/orders` (белый список в `RequireAPI`/`RequireSSR`), видит только свои
заказы, по строке отвечает «соберу / цена / комментарий» и жмёт «Собрано». Исполнитель видит
и меняет только свои закупки (чужие → 404). PDF списка по образцу акта выдачи. Конкурентность —
версии + 409, блокировки строк; архив/удаление поставщика с незакрытыми закупками запрещены
в той же транзакции. Миграция одна — `0019_add_purchase_orders` (+ `users.role/supplier_id`).
Swagger сгенерирован для новых эндпоинтов (`docs/swagger/`, добавлена зависимость `swaggo/swag`).

## Гейты судейства (реальные прогоны оркестратора, финальный код — коммит 11f5d93)

| Гейт | Итог |
|---|---|
| `make build` / `go vet ./...` | ✅ |
| `go test ./... -count=1` | ✅ 22 пакета ok |
| `golangci-lint run ./...` | ✅ 0 issues |
| `make test-integration-scoped` (diff-скоуп от feature-base) | ✅ 25 пакетов ok |
| E2E полная сюита `make e2e-isolated-full` (chromium, headed, workers=1, retries=1) | ✅ 213 passed / 0 failed / 0 flaky — улика `9-validation/evidence/last-run.json` |
| e2e `tsc --noEmit` | ✅ |
| Swagger актуален (`make swagger` → без диффа) | ✅ |
| Go-fuzz (4 таргета × 10s) | ✅ 0 падений |
| Чекеры бандла | 9 pass / 1 fail — `fuzz_verdict` SPEC-MISSING (см. known issues) |
| TS-покрытие (26 TS спеки) | ✅ все id встречаются в тестах |
| `frontend_phase_gate.sh` | n/a — гейт требует Vue-каталог `frontend/`, проект SSR; применимые части (e2e tsc, swagger) прогнаны вручную |

Циклы: судейство — 2 fix-loop (fuzz/swagger/TS-021; flaky e2e из-за общей фикстуры `uniq`),
ревью — 1 fix-loop (HIGH: нельзя было заблокировать учётку архивного поставщика; обход DS-043).

## Known issues / осознанно не сделано

- `fuzz_verdict` FAIL SPEC-MISSING — ограничение чекера: путь `frontend/e2e/fuzz/` захардкожен; спека лежит в раскладке проекта `e2e/tests/fuzz/purchase-orders_fuzz.spec.ts` и прогнана в полной сюите.
- TS-021 (адаптив/темы кабинета): нет baseline `toHaveScreenshot`, только проверки overflow + вложения-скриншоты (MEDIUM ревью).
- E2E UI-ветки `VERSION_CONFLICT` нет (покрыто live-тестами API).
- Новый код `PURCHASE_ORDER_SUPPLIER_UNAVAILABLE` (409 при восстановлении черновика недоступного поставщика) не описан в §6 спеки и в `@Failure` swagger — следствие DS-043, стоит дописать to-be DS.
- `SendCheck` при недоступном поставщике отвечает 400 `supplier_id: unavailable` (LOW: лучше 409).
- Race-тесты считают ожидания в `pg_stat_activity` по всей БД (LOW: возможен ложный сигнал при параллельных пакетах).
- Кабинет поставщика на десктопе — карточки сеткой, а не таблица, как в §8.6.
- `POST /api/v1/purchase-orders/:id/restore` добавлен по образцу других видов «Удалённых», в §6 не описан.
- Учётки поставщиков, созданные e2e, не удаляются (ADR-007) — уборка переводит их в неактивных исполнителей.
- Изменена общая e2e-фикстура `e2e/fixtures/test.ts` (`uniq` теперь по хэшу теста) — затрагивает все специи; полная сюита зелёная.
- открытые DS: нет.

## Скоуп регресса (§EE-5)

- E2E — скоуп: full (полная сюита, 213 тестов).
- полный integration-регресс не гонялся (скоуп: task — `make test-integration-scoped` по diff от feature-base).

## Хронометраж

Из `0-setup/timeline.md`: light-exec 38м 08с · light-judge 21м 37с (переделки 17м 47с) ·
light-review 19м 28с (1 рестарт) · итого **1ч 19м**. Аномалия журнала: второй заход судейства
после fix-loop ревью не был открыт `stage start` (записан как restart ревью).

стоимость контекста: разведка 131 017 ток (19.6% cache_read) — CONTEXT-COST WARN (порог 10%).
