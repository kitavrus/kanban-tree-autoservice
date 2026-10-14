# phase-8-polish — light-progress

## Чеклист
- [x] Статический аудит: захардкоженные цвета (css/html/js), таблицы без .table-responsive, flex без min-width:0
- [x] / доска — ок (board-columns скроллится внутри; BD-020/021 уже покрывали); PL-002
- [x] Карточка заказа-модалка — шапка без wrap (заголовок + акты + закрыть) → flex-wrap, акты своей строкой на ≤576px; длинные слова/серийники — overflow-wrap:anywhere; min-width:0 у детей d-flex; PL-003
- [x] Акты печати — светлые (layout data-bs-theme=light, без app.css); на ≤576px таблицы переносят слова, подписи столбиком; PL-004
- [x] /stock — ок (table-responsive, kpi flex-wrap); PL-005
- [x] /clients, /clients/new, /clients/:id — ок; PL-006
- [x] /clients/:id/equipment/new, /equipment/:id — ок; PL-007
- [x] /catalog/services|products|equipment-types (+формы) — ок; PL-008/009/010
- [x] /users (+формы) — ок; PL-011
- [x] /login — ок; PL-001
- [x] Тёмная тема: белый текст на светлых брендовых заливках (#60A5FA/#F87171, контраст ~2.5:1) → токен --kt-on-accent (меню, вкладки, категории, логотип, .btn-primary, счётчик, .text-bg-danger)
- [x] ErrRequestTooLarge → pkg/errorspkg (middleware и board ссылаются на errorspkg.ErrRequestTooLarge; код/статус/текст те же)
- [x] e2e/tests/polish.spec.ts (PL-001…PL-011)
- [x] Гейт: make build ok / go test -race ok / integration (board, middleware, errorspkg) ok / make lint 0 issues / node --check ok / tsc ok / playwright --list 11 tests

## Решения
- D-801: матрица PL — один test() на экран/семейство экранов, внутри цикл тема × {360×800, 1280×800}; тема через addInitScript(kt-theme) (последний init-скрипт побеждает), как BD-020.
- D-802: помимо data-bs-theme проверяется фактический фон body (тёмный в dark, светлый в light) — ловит захардкоженные светлые фоны; для актов — всегда светлый.
- D-803: для модалки дополнительно проверяется отсутствие overflow у .modal-header/.modal-body (страница под fixed-модалкой не скроллится в любом случае).
- D-804: текст на брендовой заливке в тёмной теме — тёмный (#0B1220) через --kt-on-accent вместо #fff: WCAG AA контраст; светлая тема визуально без изменений.
- D-805: ErrRequestTooLarge — переменная в pkg/errorspkg, из middleware удалена (без алиаса: других потребителей нет); board больше не импортирует middleware ради ошибки (для auth-хелперов импорт остаётся — это не серверный слой ошибок).
- D-806: клиенты/каталог/типы/сотрудники в polish-спеке не удаляются (как во всех прочих спеках — uniq-имена); заказы — autouse _boardCleanup.
- D-807: JS task-card.js не менялся — все правки адаптива модалки в app.css (#task-modal-scoped).
