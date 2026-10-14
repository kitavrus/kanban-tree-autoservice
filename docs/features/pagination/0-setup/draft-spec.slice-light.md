<!-- AUTO-GENERATED spec_slice.py (profile: light) из draft-spec.md — НЕ редактировать; полная спека — источник истины, гейты работают только с ней -->
# Draft Spec — pagination (доска + справочники каталога)

**Tier:** S
**Трек:** /pipeline-light · **Ветка:** `feat/pagination` (от `origin/feat/phase-8-polish`, bb4a782)
**UI-scope:** затронутые экраны (доска, каталог: услуги/товары/категории, форма услуги)
**Совместимость:** API — только аддитивные изменения (новые опциональные query-параметры и поле), конверт `{items,total,limit,offset}` (CLAUDE.md §API-1) сохраняется.


## 11. Тестовые сценарии

| ID | Уровень | Сценарий |
|---|---|---|
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
