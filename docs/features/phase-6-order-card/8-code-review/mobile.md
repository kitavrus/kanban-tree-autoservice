---
stage: 8-code-review
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# Mobile API Review (8-code-review): phase-6-order-card — APPROVED

**Артефакты, которые читал:**
- `docs/features/phase-6-order-card/8-code-review/input-bundle.md`
- `docs/features/phase-6-order-card/8-code-review/diff-backend.patch` (точечно, по файлам)
- `3-design/output.md §Контракт`, `3-design/design-errors.md §Codes 16-22 / §Error Mapping / §Validation Fields`, `3-design/design-go-layers.md §DTO Task Changes / §DTO OrderCard / §DTO Row Requests`
- Реализация: `card_dto.go`, `card_handler.go`, `card_service.go`, `card_repo.go`, `routes.go`, `errors.go`, `tasks_dto.go`, `tasks_repo.go`, `task_equipment_service.go`, `task_equipment_validate.go`, `subtasks_service.go`, `subtasks_validate.go`, `subtask_products_service.go`, `subtask_products_validate.go`, `subtask_logs_service.go`, `enums.go`

**Tier:** M

**swagger:no (UI-scope)** — swagger.json/baseline не существуют как артефакты; контракт сверен по DTO-структурам/маршрутам из diff. Swagger-специфичные пункты (аннотации, `$ref`) — n/a.

## Findings

### HIGH (блокеры)
Нет.

### MEDIUM (advisory)
Нет.

### LOW (nice-to-have)

#### [L-01] `numeric.Amount` — as-is пакет, не в скоупе диффа
**Класс:** Контракт
**Локация:** `pkg/numeric/amount.go` (не изменён в этом PR)
**Проблема:** подтверждено — `Amount.MarshalJSON` отдаёт JSON-число без кавычек, `*numeric.Amount` при nil сериализуется как `null` (штатное поведение `encoding/json` для указателя) — соответствует требованию "numeric.Amount как JSON-число". Фиксирую как факт, не как проблему: это as-is код фазы 5, отдельно не гарантирован этим PR, но реализация фазы 6 полагается на него везде корректно (`CardSubtaskProduct.Price/Qty/LineTotal`, `CardStock`, `TaskCard.TotalAmount`).
**Рекомендация:** нет действия. Информационная запись для полноты отчёта.

## Проверка по пунктам задания

- **Пути (вложенные create / плоские edit-delete, искл. оборудования):** `routes.go` 1:1 совпадает с таблицей `3-design/output.md §Контракт` — `POST /tasks/:id/subtasks` (вложенный create), `PUT/DELETE /subtasks/:id` (плоские), `POST /tasks/:id/equipment` + `PUT/DELETE /tasks/:id/equipment/:equipment_id` (оборудование — исключение, путь остаётся вложенным на всех трёх методах). Совпадение полное.
- **Ответ мутаций `{card, warnings}`:** `OrderCardMutationResponse{Card, Warnings}` в `card_dto.go`; все 13 мутационных методов сервиса (`card_service.go`, `task_equipment_service.go`, `subtasks_service.go`, `subtask_products_service.go`, `subtask_logs_service.go`) строят ответ через `cardMutation()` — единая re-fetch-after-write точка, `Warnings` гарантированно `[]StockWarning{}`, не nil.
- **`GET /tasks/:id/card`:** реализован (`card_handler.go Card()`), путь и код ответа (200) совпадают с контрактом.
- **`total_amount` единообразно:** одно и то же имя поля и вычисление (`totalAmountSelectSQL`) используется в `TaskRepo.Get` (GET /tasks/:id) и `TaskRepo.List` (GET /tasks) — идентичный SQL-фрагмент, не два разных выражения. Карточка использует своё отдельное `subtask_total`/`CardSubtask.Total`, что design одобряет явно («Итог заказа — только task.total_amount... другого имени итога нет»).
- **`position` у `CardSubtaskProduct`:** поле присутствует (`card_dto.go`), заполняется и сортируется в SQL (`card_repo.go Products()`: `ORDER BY sp.subtask_id, sp.position, sp.id`) — соответствует ADR-103.
- **`created_at` в `EquipmentHistory`:** поле есть, заполняется через `json_agg` в SQL (`card_repo.go Equipment()`), тип `time.Time` — соответствует design.
- **Nullable:** все опциональные поля из design-go-layers.md (`AssigneeID/AssigneeName/AssigneeIsActive/DueDate/Price/EquipmentID` у `CardSubtask`; `Price` у `CardSubtaskProduct`/`SubtaskProductUpdateRequest`; `LastLog` у `TaskCard`) реализованы как указатели 1:1, без расхождений nullable true/false между DTO.
- **`numeric.Amount` как JSON-число:** подтверждено (см. L-01).
- **Коды 16–22:** `errors.go` — дословное совпадение текста сообщений, HTTP-статусов и sentinel-имён с `design-errors.md §Codes 16-22`. Порядок проверок (404 → 403 → 409 → 400 ссылочные) соблюдён во всех проверенных сервисах (`subtask_logs_service.go`: 404 родителя перед 403 `SUBTASK_LOG_FORBIDDEN`; `subtasks_service.go`: `lockRowsScope` → `Lock` строки → `SUBTASK_FINAL_COLUMN` → ссылочные поля; `task_equipment_service.go`: `LockTask` → `UsedBy` → `TASK_EQUIPMENT_IN_USE` с `details.subtasks`).
- **Валидация 400 `details.fields`:** все Row Request типы имеют `normalize()/validate() []errorspkg.Field`, ошибки возвращаются через `errorspkg.Validation(f...)` — единообразно с эталоном `tasks_validate.go`.
- **N+1:** не найдено. `CardRepo` — 7 отдельных запросов на карточку (design заявляет именно это), товары и журнал каждый одним запросом на все услуги заказа сразу (`map[string][]...]` по `subtask_id`), без цикла с запросом на каждую услугу. `TaskRepo.List`/`Get` считают `total_amount`/`last_log` коррелированными подзапросами/LATERAL JOIN в одном SQL-запросе на страницу, не в цикле Go.

## Резюме

Контракт design реализован без отклонений: пути, коды ответов, DTO-поля (включая nullable), коды ошибок 16–22 и порядок их проверки, а также `total_amount`/`position`/`created_at` — совпадают дословно с `3-design/output.md`, `design-errors.md`, `design-go-layers.md`. N+1 не обнаружено. HIGH=0 → **APPROVED**.
