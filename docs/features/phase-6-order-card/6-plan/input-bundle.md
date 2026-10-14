# Input Bundle: phase-6-order-card / стадия 6-plan — generated 2026-09-27 11:31

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
#### AC-001: Полная карточка вместо простой формы
#### AC-002: Шапка заказа сохраняется кнопкой, конфликт не затирает
#### AC-003: Оплата — только администратор
#### AC-004: Принятое оборудование клиента в заказе
#### AC-005: Новое оборудование прямо из карточки
#### AC-006: История ремонтов единицы
#### AC-007: Услуги заказа из каталога
#### AC-008: Типовой комплект товаров услуги
#### AC-009: Подсказка наличия товара
#### AC-010: Списание при правке товаров в работе
#### AC-011: Статус услуги и финальная колонка
#### AC-012: Журнал работ услуги
#### AC-013: Итог заказа
#### AC-014: Одновременная правка услуг и товаров
#### AC-015: Акт приёма
#### AC-016: Акт выдачи
#### AC-017: Права исполнителя в карточке

## Источник: 1-research/output.md [sections]
## Паттерны в коде
- Многосущностный feature-пакет: файлы-по-сущности (dto/handler/repo/service/validate) в ОДНОМ пакете `board` — эталон `internal/features/board/errors.go:1-4`; фаза 6 добавляет `subtasks`-файлы в ТОТ ЖЕ пакет, не отдельную фичу.
- Lock Order (ADR-104): `columns → tasks → products`, FOR UPDATE/FOR SHARE в этом порядке внутри `withTx`; новые мутации `subtasks`/`subtask_products` ДОЛЖНЫ брать `LockTask` ПЕРЕД правкой — см. LOW code review ниже (Граничные случаи).
- Re-fetch-after-write: ответ mutation-хендлера из `repo.Get()` после commit, не RETURNING (`internal/features/board/tasks_service.go:330-340`).
- Sentinel-ошибки — общие `var Err... = errorspkg.New(...)` на пакете (`internal/features/board/errors.go:15-22`), `.WithDetail` — КОПИЯ (`pkg/errorspkg/errors.go:36-44`), sentinel не мутируется.
- Optimistic locking (`version`) — только `clients`/`tasks` (D28, `docs/plan.md:152`); `subtasks`/`subtask_products` — last-write-wins без version (паттерн уже проверен на `tasks.Move`).
- DTO-валидация в методах запроса `(in Request) normalize()/validate()` (`internal/features/board/tasks_validate.go:30-100`), не в handler/service.
- Денежные/количественные поля — `pkg/numeric.Amount` (строка в Go, число на wire) — обязателен для `subtask_products.qty/price`, `subtasks.price`, итогов заказа.
- `withTx` с ретраем 40P01/40001×3 (`internal/features/board/tx.go:16-29`) — обязателен для мутаций склада/нескольких таблиц.
- Testid снимается атрибутом (не только `d-none`) для элементов, видимых только одной роли (`web/ssr/_shared/js/board.js:576-579`).

## Источник: draft-spec.md [sections]
MISSING

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 17 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
