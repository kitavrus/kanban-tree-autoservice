# Design Tests: phase-5-board

> Owner: тест-матрица. Уровни: unit (без БД) · integration `BaseSuite` (dockertest, тег `integration`) · live `testutil.LiveSuite` (реальный бинарь + TCP + реальный login, §TD-8; auth-стабы и `app.Test` запрещены, кроме fuzz §9.1) · E2E Playwright (`e2e/tests/board.spec.ts`, `e2e/tests/stock.spec.ts`, `e2e/tests/board-guards.spec.ts`; POM `e2e/pages/BoardPage.ts`, `e2e/pages/StockPage.ts`; ID `BD-NNN`). Каждая строка → AC.

## Test Files
| Файл | Уровень |
|---|---|
| `internal/features/board/columns_validate_test.go`, `tasks_validate_test.go`, `tasks_filter_test.go`, `enums_test.go` | unit |
| `internal/features/board/board_fuzz_test.go` | fuzz (§9.1) |
| `internal/features/board/stock_engine_integration_test.go` | integration (BaseSuite) |
| `internal/features/board/board_http_live_test.go` (`BoardLiveSuite`), `stock_http_live_test.go` (`StockLiveSuite`) | live |
| `internal/features/clients/clients_http_live_test.go` (+кейсы D26), `internal/features/catalog/services_http_live_test.go` (+SERVICE_IN_USE) | live (дополнение) |
| `internal/seeddemo/seeddemo_integration_test.go` | integration |
| `pkg/errorspkg/errors_test.go` (+`WithDetail` не мутирует sentinel, details merge) | unit |
| `internal/ssr/shell_test.go` (+badge) | unit |
| `e2e/fixtures/board.ts` (API-хелперы + `pg`-хелпер услуг, ADR-103), `e2e/fixtures/board-types.ts` (§TD-6.2) | E2E fixtures |

## Happy Path
| ID | Сценарий | Уровень | Driver |
|---|---|---|---|
| LV-001 | GET /columns после миграции — 5 колонок D32 с флагами и порядком | live | AC-008 |
| LV-002 | POST /tasks → 201, id bigint растёт, позиция в конец; GET /tasks/:id | live | AC-009 |
| LV-003 | PUT /tasks/:id с version → 200, version+1; поля обновлены | live | AC-009 |
| LV-004 | move между колонками → column_id/position сохранены; GET /tasks порядок по position | live | AC-002 |
| LV-005 | move внутри колонки prev/next → между соседями; пустая колонка; верх (только next); низ (только prev) | live | AC-002 |
| LV-006 | move в «В работе» заказа с товарами → stock−, task_stock=demand, журнал `task_consume`; назад в «Согласование» → возврат `task_return`; «В работе»→«Ждёт запчасти» — 0 новых движений | live | AC-004 |
| LV-007 | списание при нехватке → 200, stock<0, warnings kind=negative; stock≤min → below_min | live | AC-005 |
| LV-008 | PUT колонки consumes_stock true/false при 2 заказах → пересчёт всех, warnings объединены | live | AC-007 |
| LV-009 | CRUD колонок: create (в конец), rename/color/position (перенумерация 1..N), delete пустой | live | AC-006 |
| LV-010 | DELETE /tasks/:id admin → товары вернулись, stock_movements.task_id = NULL, subtasks удалены | live | AC-012 |
| LV-011 | admin меняет is_paid → paid_at выставлен/снят | live | AC-011 |
| LV-012 | GET /stock?scope=risky|all, KPI summary = множества ADR-002 (out/low/will_low); pending только по не-consumes/не-final колонкам | live | AC-015 |
| LV-013 | PUT /stock/:product_id admin → stock, журнал `manual` дельтой | live | AC-015 |
| LV-014 | SSR `/` и `/stock` 200 для executor и admin; пункт «Остатки» с бейджем = risk_count | live | AC-001, AC-016 |
| LV-015 | GET /assignees — executor 200, только активные | live | AC-009 |
| IT-001 | seeddemo.Run на пустой БД: заказы во всех колонках, услуги, товары; повтор → no-op «заказы уже есть»; без админа → ошибка | integration | AC-019 |

## Filters
| ID | Сценарий | Уровень | Driver |
|---|---|---|---|
| LV-020 | client_id, q (название/клиент/`#N`/`N`), category_id (по услугам), paid, due=soon/overdue (финальные исключены, просроченная услуга учитывается) — каждый отдельно и комбинация | live | AC-013 |
| LV-021 | executor без `assignee` → только свои (ответственный ИЛИ исполнитель услуги); `assignee=all` → все; admin без параметра → все | live | AC-014 |
| LV-022 | limit/offset + total; limit>1000 → 400; дефолты limit (tasks 500, columns 100, assignees 200); порядок items по контракту (колонка.position → task.position → id) | live | AC-013 |
| UT-001 | ParseTaskFilter: enum'ы, uuid, `#12`/`12`/`12a`, пустые → дефолты | unit | AC-013 |

## Invalid Input
TD-4 adversarial (live):
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-030 | все эндпоинты без сессии | 401 | — |
| LV-031 | write без CSRF-заголовка | 403 CSRF каркаса | — |
| LV-032 | мусорное тело (не JSON, лишнее поле, `prev_id:"x"`, `stock:"5"`, массив) | 400, не 500 | — |
| LV-033 | POST/PUT /tasks: пустой title, title 201 символ, нет client_id, несуществующие client_id/column_id/assignee_id, due_date `2026-13-40` | 400 `details.fields[]` | AC-009 |
| LV-034 | PUT колонки: пустое name, color `red`, position 0 | 400 fields | AC-006 |
| LV-035 | `/tasks/abc`, `/tasks/-1`, `/tasks/99999999999999999999`, `/columns/not-uuid`, `/stock/not-uuid` | 404 | — |
| LV-036 | move: prev_id из другой колонки / несуществующий / = самому заказу | 200, позиция по правилу «сосед игнорируется» | AC-002 |
| LV-037 | query: `paid=maybe`, `due=x`, `assignee=bad`, `limit=-1`, `scope=zzz` | 400 field | AC-013 |
| UT-002 | columns_validate / tasks_validate — таблица граничных значений | unit | AC-006, AC-009 |

## Access Control
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-040 | executor: POST/PUT/DELETE /columns | 403 `FORBIDDEN` | AC-006 |
| LV-041 | executor: DELETE /tasks/:id | 403, заказ жив | AC-012 |
| LV-042 | executor: PUT /tasks is_paid ≠ текущему → 403 `PAID_ADMIN_ONLY`; = текущему → 200; POST с is_paid=true → 403 `PAID_ADMIN_ONLY` | AC-011 |
| LV-043 | executor: PUT /stock/:id → 403; GET /stock, /stock/summary → 200 | AC-015 |
| LV-044 | executor: POST/PUT/move заказа → 200 (D19) | AC-009, AC-002 |

## Business Rules
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-050 | move в «Готово» при subtask status<>done | 409 SUBTASKS_NOT_DONE, details.subtasks[{id,service_name,status}], колонка/склад без изменений | AC-003 |
| LV-051 | та же проверка через PUT со сменой column_id | 409 | AC-003 |
| LV-052 | все услуги done → move в «Готово» 200 | AC-003 |
| LV-053 | DELETE колонки с заказом | 409 COLUMN_NOT_EMPTY | AC-006 |
| LV-054 | удалить/сделать финальной последнюю нефинальную | 409 LAST_NON_FINAL_COLUMN | AC-006 |
| LV-055 | PUT с устаревшей version | 409 TASK_VERSION_CONFLICT, данные не затёрты | AC-010 |
| LV-056 | DELETE клиента с заказом (API + SSR POST /clients/:id/delete) | 409 CLIENT_HAS_TASKS / flash | AC-017 |
| LV-057 | DELETE оборудования из task_equipment и из subtasks.equipment_id | 409 EQUIPMENT_USED_IN_TASKS | AC-018 |
| LV-058 | DELETE услуги из subtasks | 409 SERVICE_IN_USE (не 500) | ADR-005 |
| LV-059 | DELETE товара из subtask_products/task_stock | 409 PRODUCT_IN_USE (не 500) | ADR-005 |

## Race Condition
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-060 | два клиента (2 `Browser`) параллельно move одной карточки в разные колонки (N=20 горутин, `-race`) | все 200 (LWW), итог = одна из целей, `task_stock` = target итоговой колонки, инвариант склада | AC-002, AC-004 |
| LV-061 | форма (PUT со старой version) vs move | move 200, PUT 409 TASK_VERSION_CONFLICT | AC-010 |
| LV-062 | параллельно: toggle consumes_stock колонки ‖ move заказа в/из неё | нет deadlock-500 (retry), инвариант склада, `task_stock` согласован с итоговой колонкой | AC-007 |
| LV-063 | параллельные move двух заказов с общим товаром | stock = начальный − Σ списаний, без потерянных обновлений | AC-004 |
| LV-064 | 50 вставок между одними соседями → ребалансировка, порядок сохранён, позиции уникальны | AC-002 |
| LV-065 | DELETE клиента ‖ POST заказа на этого клиента | либо 409 CLIENT_HAS_TASKS, либо 400 field client_id — не 500 | AC-017 |

## Stock Invariant
PRD §3 «Расхождения остаток ↔ сумма движений = 0».
- IT-010 (`stock_engine_integration_test.go`): таблично — последовательность (consume, return, повтор consume, toggle колонки, delete заказа, manual adjust) → после КАЖДОГО шага `products.stock = начальный + Σ stock_movements.qty_delta` для всех товаров и `task_stock(task) = demand × consumes(column)`.
- Хелпер `assertStockInvariant(t, pool)` вызывается в конце LV-006…LV-010, LV-013, LV-060…LV-063 и в IT-001 (seed-demo).
- «Начальный» = stock в момент создания товара (фиксируется тестом; журнал до создания пуст — D44).

## Fuzz Targets
Класс 9 (qa.md) — триггер: парсинг query/JSON от клиента.
| Target | Функция | Инварианты | Seeds |
|---|---|---|---|
| `FuzzParseTaskFilter` | `ParseTaskFilter` (url.Values-вход) | no panic; ошибка — только `*errorspkg.AppError` 400 | `#`, `#0`, `#999999999999999999999`, `%`, `_\`, `assignee=me&assignee=all`, UTF-8 мусор |
| `FuzzTaskInputValidate` | `TaskCreateRequest.normalize().validate()` | no panic; fields ⊂ {title, client_id, column_id, assignee_id, due_date} | 200/201 символ, `\u0000`, `2026-02-30` |
| `FuzzColumnInputValidate` | `ColumnInput.validate()` | no panic | `#GGGGGG`, `#fff`, position MinInt |
| `FuzzMoveBody` | `app.Test` POST move (исключение §9.1) | статус ∈ {200,400,404,409}, никогда 5xx | `{}`, `{"prev_id":1e99}`, `[]`, `null` |

## E2E Scenarios
Стенд `make local-up` + `make migrate-up`; изоляция — уникальный клиент/товар/колонки `uniq()` на тест, доска открывается с `?client_id=<uniq>`; §TD-4 чистота консоли/сети (ожидаемые 409 — `allowHttpErrors`). `--project=chromium --headed --workers=1`.
| ID | Сценарий | Driver |
|---|---|---|
| BD-001 | доска: колонки в порядке, карточка с `#N`, названием, клиентом, ответственным, сроком, оплатой; цвет полосы = цвет колонки | AC-001 |
| BD-002 | HTML5 DnD (`locator.dragTo`) «Новые»→«Согласование», перестановка внутри колонки; reload — положение сохранено | AC-002 |
| BD-003 | заказ с открытой услугой (pg-фикстура) → DnD в «Готово»: карточка вернулась, ui-dialog со списком услуг; услуга done → перенос проходит | AC-003 |
| BD-004 | товар stock=5; заказ с товаром 2 → «В работе»: /stock показывает 3; обратно — 5; «В работе»→«Ждёт запчасти» — 3 | AC-004 |
| BD-005 | товар stock=1, потребность 3 → перенос проходит, toast warnings со ссылкой «Остатки» | AC-005 |
| BD-006 | admin: создать колонку, переименовать, цвет, позиция, удалить; удаление непустой — сообщение; executor не видит «+Колонка»/правки | AC-006 |
| BD-007 | admin включает «списывать» у своей колонки с заказом → остаток уменьшился; выключает → вернулся | AC-007 |
| BD-008 | дефолтные колонки D32 присутствуют (по имени/флагам через UI-модалку) | AC-008 |
| BD-009 | создание заказа формой (combobox клиента, колонка, ответственный, срок) → карточка с новым `#N`; правка той же формой; двойной клик «Сохранить» — один запрос | AC-009 |
| BD-010 | два контекста открыли форму; первый сохранил, второй видит «Заказ уже изменён»; форма vs move — то же | AC-010 |
| BD-011 | admin отмечает оплату; executor: переключатель оплаты disabled, прямой PUT → 403 `PAID_ADMIN_ONLY` | AC-011 |
| BD-012 | admin удаляет заказ с подтверждением ui-dialog — товары вернулись; у executor кнопки нет | AC-012 |
| BD-013 | фильтры клиент+текст+вид работ+оплата+срок сочетаются, отражены в URL, reload сохраняет, «Сбросить» | AC-013 |
| BD-014 | executor при входе видит только свои (ответственный / исполнитель услуги), «Все» — видна чужая карточка | AC-014 |
| BD-015 | «Остатки»: KPI, таблица рисковых с прогнозом; admin правит остаток; executor — без кнопки правки | AC-015 |
| BD-016 | бейдж «Остатки» виден executor и admin; после move с уходом в минимум — +1 без перезагрузки | AC-016 |
| BD-017 | удаление клиента с заказом в /clients — сообщение, клиент на месте | AC-017 |
| BD-018 | удаление оборудования из task_equipment (pg-фикстура) — сообщение | AC-018 |
| BD-020 | 360×800, обе темы: `/`, форма заказа, `/stock` — `hasHorizontalOverflow()` = false; доска скроллится внутри `.board-columns`; перенос через меню «Переместить в…» | AC-020 |
- AC-019 (seed-demo) — IT-001 (не E2E: команда + БД; E2E-стенд общий). Проверка оркестратором — ручной smoke Runbook.
- LAST_NON_FINAL_COLUMN — только LV-054 (на общем E2E-стенде нельзя финализировать все колонки).
