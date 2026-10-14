---
stage: 8-code-review
feature: transfers-partners
tier: M
verdict: APPROVED
high: 0
medium: 1
low: 6
---
# Code Review: transfers-partners — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза) · MVP (§CM-1) · swagger:no → Sec 15 n/a

Вход: `input-bundle.md`, `diff-migrations.patch`, исходники `internal/features/transfers/{service,repo,validate,handler,routes,ssr,dto}.go` целиком, diff board / clients / timezone / seeddemo / JS / e2e. Гейты (build, test, lint, integration, E2E 194 passed) — по прогонам оркестратора, не перегонялись. Чекеры 9/10; `e2e_drift` FAIL — ложное срабатывание (формат `TR-NNN (TS-NNN)` в `e2e/tests/`), TS-029/032/034/036/039 привязаны: TR-012…TR-016, TR-007, EW-005.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[internal/features/transfers/repo.go:915, :944-947 → :300, :355] MEDIUM — гонка «единица не из заказа» (DS-035, DS-026).** `checkRefs` читает `tasks.client_id` и состав `task_equipment` БЕЗ блокировки заказа; занятие единицы (`claimEquipment`) идёт позже. Параллельные `DELETE /tasks/:id/equipment/:eid` (`task_equipment_service.go:114-132`) либо смена клиента заказа (`tasks_service.go:207-226`) держат `LockTask … FOR UPDATE`, видят `active_transfer_id IS NULL`, убирают единицу и коммитятся; `INSERT transfers` ждёт их на FK (KEY SHARE) и проходит. Итог: незавершённый документ с единицей, которой нет в оборудовании заказа, либо `transfers.client_id` ≠ клиенту заказа. То же в `Update` (:328).
   Исправление: в `checkRefs` читать заказ `SELECT client_id::text FROM tasks WHERE id = $1 FOR KEY SHARE` (конфликтует с `FOR UPDATE` в `LockTask`; под READ COMMITTED после ожидания перечитывается актуальная строка → `not_in_task` / `task_id: not_found`). Порядок блокировок не ломается: заказ → документ → остаток → единица. Тест: параллельные create + remove/смена клиента.

## Незначительные замечания (можно в следующем PR)
1. **[web/ssr/_shared/js/transfers.js:733-746] LOW.** `prefillFromTask` берёт все единицы «у клиента» / «на складе», включая занятые другим незавершённым документом (`active_transfer_id != null`), хотя список «+ Оборудование заказа» те же единицы отключает (`unitBusy`). Предзаполненная форма гарантированно даёт 409 `EQUIPMENT_IN_TRANSFER`. Исправление: `&& !unitBusy(eq)` в обоих фильтрах.
2. **[web/ssr/_shared/js/task-card.js:468] LOW.** «перемещение #N» теперь ссылка и для исполнителя; если единица стоит в документе «склад → склад», ссылка ведёт на «Перемещение не найдено». Исправление: отдавать направление активного документа в `CardEquipment` и для не-админа рисовать текст при не-клиентском направлении.
3. **[internal/features/clients/routes.go (`GET /api/v1/equipment/:id/history`), history.go:131-139] LOW — вынести пользователю.** API истории единицы (RequireAPI, без admin) отдаёт исполнителю номера, статусы, склады и даты ВСЕХ документов единицы, включая «склад → склад» — расходится с DS-042 «существование не раскрывается». As-built ядра (эндпоинт в diff не менялся по правам), SSR-страница истории — только admin. Вариант: для не-админа фильтровать `t.direction = ANY(client)`.
4. **[internal/features/clients/repo.go:363-369, transfers.client_id] LOW — вынести пользователю.** После завершения документов заказ можно перевести на другого клиента: `transfers.client_id` остаётся прежним (снимок на момент создания), а инвариант спеки §4 — «клиент = клиент заказа». При незавершённых документах смена блокируется (`ForeignInTransfer`), расходятся только исторические. Решение: принять снимок (зафиксировать DS) либо запретить смену клиента заказа с документами (409).
5. **[e2e/tests/transfers-partners.spec.ts:400, :424-429] LOW.** TR-015: `today` берётся один раз после создания документа; на стыке суток пользователя шаг «период исправлен» (`с = по = today`) не найдёт документ, созданный «вчера». Предикаты `allowHttpErrors` (:414, TR-016 :512) сравнивают с теми же строками, что вводятся в поля, — сами по себе от полуночи не зависят. Исправление: брать `today` из `created_at` документа через `KT.time`.
6. **[internal/features/board/card_dto.go:224] LOW.** `TaskEquipmentSendRequest` — тип без единого использования (остаток ядра, не из этого diff). Удалить.

## Оценка по вопросам оркестратора
1. **Права — допустимо, дефектов нет.** `routes.go`: API — `RequireAPI`, SSR — `RequireSSR`. `List` — `ClientOnly` → `direction = ANY(client)` (repo.go:117), фильтры `direction=` / `supplier_id=` чужого дают пустой список, `total = 0`. `Get` — `canAccess` → 404 (service.go:196). `Update` — `head` (404) ДО валидации и транзакции (:245); `direction` / `task_id` → `not_allowed`; `supplier_id` / лишний склад у документа с клиентом → `not_allowed` (validate.go:131-144) — сменить направление нельзя. `Send` / `Receive` / `Cancel` — `actionTarget` → `head` до проверки `version` и до repo (:296-299); направление неизменяемо, TOCTOU нет. `Create` — 403 до валидации (:220), некорректное направление → 400. SSR не отдаёт документы; меню и фильтры урезаны шаблоном. `GET /api/v1/equipment` — только `active_transfer_id`; «Остатки» — сумма. 404/403 согласованы с DS-018/DS-042. Остаточное — LOW-2, LOW-3.
2. **Миграция 0015 — допустимо.** CHECK маршрута — ровно четыре сочетания, `direction` NOT NULL, backfill через DEFAULT + DROP DEFAULT, индексы на три FK + `direction`, `created_at`. CHECK «факт ≤ отправленного» снят поиском по определению; единственный путь записи факта — `resolveReceive` (validate.go:335: превышение только у `supplier_to_warehouse`; документы с клиентом — `items: not_allowed`; create+receive передаёт `nil`). Причины движений — три новых. Down: потеря документов не-w2w описана в шапке файла; `stock_movements.transfer_id` и `equipment.active_transfer_id` — `ON DELETE SET NULL`, порядок UPDATE → DELETE соблюдает `equipment_location_check`. Нюанс down: единица «в пути» возврата клиенту становится «у клиента», а не на складе отправления — MVP, не флагается.
3. **Местоположение — допустимо.** `doSend`: источник — склад отправления либо `at_client` (repo.go:701-711); `settleEquipment`: пустой склад → `at_client`, `warehouse_id NULL`; отмена из `in_transit` — в `st.from`; проверка числа обновлённых строк. «Принять сразу» — одна транзакция (:306-311). «Один склад при возврате» держит `notAtSourceError` при отправке. Принадлежность заказу и клиенту — :944-947 (гонка — MEDIUM-1).
4. **Поставщик — допустимо.** Отправка без движений; приёмка: `stock += qty_received`, `supplier_in` при факте > 0 (:424-429); недопоставка закрывает документ; «в пути» — stock_repo.go CTE `transit`; архивный поставщик: новый → 400, существующий — `grandSupplier` (:907).
5. **Период — допустимо.** `DayStart` (ParseInLocation) + `NextDay` через календарь (23/25-часовые сутки корректны); полуинтервал `[from; to+1день)`; `before_from` → 400; SQL полностью параметризован; `task_id` / `supplier_id` / `direction` валидируются, скоуп накладывается поверх.
6. **Удаление заказа — допустимо.** Отлов FK по коду не используется и не нужен: `hasTransfers` под `LockTask FOR UPDATE` до `Reconcile` (tasks_service.go:371-378). Гонка закрыта: вставка документа берёт KEY SHARE на заказ — либо удаление ждёт и видит документ (409), либо вставка получает 23503 `transfers_task_id_fkey` → 400 `task_id: not_found` (repo.go:255).
7. **Карточка заказа — допустимо.** Блок, кнопки, `?new=1&direction=&task_id=` — реализованы; `ReceiveAtWarehouse`, `DefaultWarehouse` (task_equipment), `errNoActiveWarehouse`, select и JS-хелперы склада приёма удалены; `warehouse_id` — `json.RawMessage`, игнорируется. Остаток — LOW-1, LOW-6.
8. **Решения вне спеки — допустимо:** LEFT JOIN + «клиент» в истории (единица не бывает в документе от поставщика — `kindAllowed`); `client_id` в теле игнорируется (спека §6.2 п.3); `transfers_client_id_idx` — индекс FK; `transit_to_name = «клиент»`; «Принять сразу» только у нового приёма (DS-013). К пользователю — LOW-3, LOW-4.
9. **seed-demo — допустимо.** Документы новых направлений проходят `transfers_route_check` и `equipment_location_check`; движение `supplier_in` на факт 18 при заказанных 20; покрыт `seeddemo_integration_test.go`.
10. **Тесты — достаточно.** Live-тесты TS-004/006/007/008/015/020/023/025 проверяют по существу (TS-015: все 7 эндпоинтов × 2 чужих направления, 404 без `version`, документ не тронут). TS-023: граница суток в БД — Go live-тест, экранная половина — TR-016; достаточно. Риск полуночи — LOW-5.

## Sec 18 — API-контракт
- Конверт списка `{items,total,limit,offset}` — да. Ключи snake_case, URL kebab — да (чекер).
- Nullable: `from/to_warehouse_*`, `task_*`, `client_*`, `supplier_*` — `*string` / `*int64` ↔ NULL в §4; смена `string → null` у складов — breaking относительно ядра, MVP, клиент (`transfers.js`, `e2e/fixtures/auth.ts`) обновлён.
- Ошибки — только `errorspkg`; 400 → `details.fields[]`; 403 `FORBIDDEN`, 404 `TRANSFER_NOT_FOUND`, 409 с `details`.
- N+1 нет (подзапрос `count(*)` в одном SQL).

## Соответствие кода спеке
- [x] DDL §4 ↔ миграция 0015 — ДА
- [x] Контракты §6 (поля, коды ошибок) — ДА
- [x] Права DS-018 / DS-042 — ДА (оговорки LOW-2, LOW-3)
- [x] §Impact: все 13 позиций имеют правку в diff — ДА
- [x] Scope creep (части 5–6) не реализован — ДА
- [x] TS-001…TS-025, хвосты TS-006/020/027 — покрыты; E2E TS-029/032/034/036/039 — покрыты
- Monitoring / Rollout / Runbook / Perf — N/A (M, не заявлены)

## Статус фаз
- Носитель статусов не сверялся построчно (pre-condition оркестратора); незакрытых scaffold-стабов (`t.Skip("scaffold stub`, `test.fixme`) в diff нет.

## Итог
APPROVED. MEDIUM-1 — исправить в этом PR (одна строка `FOR KEY SHARE` + тест). LOW-3, LOW-4 — решение пользователя.
