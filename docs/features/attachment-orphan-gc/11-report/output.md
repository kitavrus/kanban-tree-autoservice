# Отчёт: attachment-orphan-gc — сборщик файлов-сирот вложений

**Вердикт ревью:** APPROVED (high 0 / medium 0 / low 3) — `8-code-review/output.md`
**Ветка:** `feat/attachment-orphan-gc` от `feat/phase-8-polish` (`d24b7cb`); не запушена.

## Что сделано

Фоновый сборщик удаляет из `UPLOADS_DIR` файлы вложений, у которых нет строки в
`task_attachments` (после падения процесса между `Put` и INSERT или между commit DELETE и
удалением файла), а также брошенные временные `.upload-*`. Трогает только файлы старше
`UPLOADS_GC_MIN_AGE` (1h) с именем-ключом в канонической форме `NewKey` или temp-префиксом —
чужие файлы и каталоги не удаляются. Ключи сверяются с БД целиком до первого удаления: при
ошибке БД не удаляется ничего. Предохранитель: если старых ключей ≥ 20 и ни одного нет в БД
(чужая или пустая база), проход прерывается с `ErrSweepSuspicious` без удаления.

Код: `internal/filestore/orphans.go` (`SweepOrphans`, `KeyIndex`, `RunOrphanCollector`),
`AttachmentRepo.ExistingKeys` (`internal/features/board/attachments_repo.go`), env в
`internal/config/config.go` и `.env.example`, подключение в `cmd/server/main.go`.

## Решение: фоновый тикер в сервере, а не `cmd/…` + make-таргет

- Каталог и пул БД уже есть в процессе сервера, так что отдельная точка входа с тем же env не нужна.
- В проекте нет планировщика: cron и инфраструктура не развёрнуты. Команду без расписания на
  практике никто бы не запускал, и сироты копились бы дальше.
- Сироты появляются при аварийном падении, то есть к следующему старту. Первый проход идёт
  сразу при старте, дальше — каждые `UPLOADS_GC_INTERVAL` (6h).
- Проход идемпотентен: два инстанса blue-green на одном каталоге не конфликтуют
  (`ErrNotExist` при удалении не считается ошибкой).
- Отключение без релиза — `UPLOADS_GC_INTERVAL=0`. Миграций нет, откат тривиален.

Параметры: `UPLOADS_GC_INTERVAL` (6h; 0 — выключен, иначе ≥ 1s), `UPLOADS_GC_MIN_AGE`
(1h, ≥ 1m).

## Гейты судейства (реальные прогоны оркестратора)

| Гейт | Вердикт |
|---|---|
| `make build` | PASS |
| `go test ./... -count=1` | PASS |
| `golangci-lint run ./...` | PASS (0 issues) |
| `BASE=d24b7cb make test-integration-scoped` (-race) | PASS — 8 затронутых пакетов, 2 прогона (до и после фиксов ревью) |
| review_bundle, чекеры | 9 pass / 0 fail / 0 skip |
| frontend_phase_gate / E2E | N/A — фронт не тронут (`--no-e2e`) |
| TS-покрытие | N/A — спеки нет, трек по тексту задачи |
| Гейт 2б (review_verdict_check --no-e2e) | PASS (APPROVED) |

Тесты:
- unit — `orphans_test.go`: отбор, граница возраста, неканонические uuid, ошибка индекса,
  пачки по 500, предохранитель, отмена, temp-префикс `Put`, тикер; `config_test.go`.
- integration BaseSuite — `attachments_gc_integration_test.go`: `ExistingKeys` на PG, три
  происхождения сирот (нет строки / строка удалена / каскад от удаления заказа),
  идемпотентность, недоступная БД.
- LiveSuite — `attachments_gc_http_live_test.go`: бинарь сервера с `UPLOADS_GC_INTERVAL=1s`,
  загрузка через API, файлы состарены через `Chtimes`.

## Known issues / as-is

- У Fiber не задан `ReadTimeout` (as-is с фазы 7). Если загрузка висит дольше `MIN_AGE`,
  сборщик удалит её temp-файл и загрузка вернёт 500. Данные при этом не теряются.
- Предохранитель работает по принципу «всё или ничего». Законный случай «≥ 20 сирот и ни
  одного живого ключа» требует ручной чистки каталога: сборщик раз в интервал пишет Error
  `orphan sweep failed … ErrSweepSuspicious`. Порог по доле сирот и env-override — на будущее.
- В `main.go` создаётся второй `filestore.NewLocal` на тот же каталог — это дёшево и развязывает wiring сборщика от `router.go`.
- Обратные сироты (строка есть, файла нет) не входят в задачу; `OpenAttachment` отдаёт 404 и пишет warn (as-is).

## Процесс

- трек: light (один исполнитель; полный /pipeline не выполнялся — осознанный выбор)
- исполнитель: оркестратор в основной сессии, субагент не запускался. `fff_preflight` дал
  FAIL FFF-NOT-CONFIGURED (в проекте нет `.mcp.json` с fff), поэтому поиск шёл через `git grep`.
- scaffold: skip-правка (новой сущности или экрана нет; overlay: нет)
- идиомы: слепок `idiom_digest` не снимался — фича встраивается в существующий пакет board/filestore
- полный регресс E2E-сюиты не гонялся (скоуп: feature) — фронт не тронут
- полный integration-регресс не гонялся (скоуп: task) — `make test-integration-scoped` по затронутым пакетам
- fix-loop: 1 цикл по ревью (2 medium + 3 low закрыты)

## Хронометраж

| Стадия | Длит. |
|---|---|
| light-exec | 4м 34с |
| light-judge | 2м 18с |
| light-review | 7м 04с |
| **Итого** | **13м 56с** |

Подробно — `0-setup/timeline.md`.
- стоимость контекста: разведка 21 910 ток (1.5% cache_read) — context_cost PASS
