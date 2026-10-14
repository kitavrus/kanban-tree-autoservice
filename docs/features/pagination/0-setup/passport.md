# Passport: pagination (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:yes | adaptive:full | themes:both

## AC-индекс — пуст (AC ещё не созданы либо ось DS/TS)

## DS-индекс — нет (draft-spec отсутствует)

## TS-индекс
| TS-001 | unit | ParseTaskFilter: `column_id` невалидный → VALIDATION поле `column_id`; `per_column` 0/1001/abc → поле `per_column`; валидные — попадают в фильтр |
| TS-002 | integration (LiveSuite) | `per_column=2` на колонке с 5 заказами → 2 карточки этой колонки, `column_total=5` у каждой, `total` = все подходящие под фильтр |
| TS-003 | integration (LiveSuite) | `column_id=X&limit=2&offset=2` → карточки 3–4 колонки X в порядке position,id; `total`=5 |
| TS-004 | integration (LiveSuite) | фильтр (например `q`) + `per_column` → `column_total` считается с учётом фильтра |
| TS-005 | integration (LiveSuite) | SSR `/catalog/products?offset=100` при >100 товарах → 200, в HTML пейджер «Показаны 101–…», ссылка назад; `offset=-5`/`abc` → как 0 |
| TS-006 | integration (LiveSuite) | SSR `/catalog/services?offset=…` — то же для услуг, фильтры `q/category/archived` сохраняются в ссылках пейджера |
| TS-007 | E2E | Доска: колонка с 51+ заказом (создать через API в фикстуре) → видно 50 карточек и «показаны 50 из M», «Показать ещё» догружает остальные, пометка исчезает |
| TS-008 | E2E | Доска: `board-total` показывает число заказов |
| TS-009 | E2E | Каталог товаров: >100 товаров → пейджер, переход «Вперёд» меняет строки, «Назад» возвращает; поиск сбрасывает на первую страницу |
| TS-010 | E2E | Форма услуги: товар находится вводом в комбобокс строки комплекта через серверный поиск |

## Реестр артефактов (owner-файлы; читать напрямую ЗАПРЕЩЕНО — вход только через бандл)
- draft-spec.md — 8KB
- 0-setup/idioms.md — 12KB
- 0-setup/pipeline-state.md — 0KB
