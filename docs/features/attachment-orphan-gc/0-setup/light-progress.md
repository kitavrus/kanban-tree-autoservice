# light-progress — attachment-orphan-gc

Исполнитель: оркестратор в основной сессии (субагент не запускался: fff_preflight FAIL
FFF-NOT-CONFIGURED — в проекте нет `.mcp.json` с сервером fff; задача S, поиск — `git grep`).

- [x] Решение «тикер vs cmd» — фоновый тикер в процессе сервера (обоснование — 11-report)
- [x] `filestore.SweepOrphans` + `KeyIndex` + `RunOrphanCollector` (internal/filestore/orphans.go)
- [x] `tempPrefix` — общая константа Put и сборщика
- [x] `AttachmentRepo.ExistingKeys` (filestore.KeyIndex на task_attachments)
- [x] config `UPLOADS_GC_INTERVAL` (6h, 0=off) / `UPLOADS_GC_MIN_AGE` (1h, ≥1m) + .env.example
- [x] wiring в cmd/server/main.go (ctx сигналов, остановка на shutdown)
- [x] unit: orphans_test.go (отбор, граница возраста, ошибка индекса, пачки, cancel, temp-префикс Put, тикер)
- [x] unit: config_test.go (дефолты, выключение, пороги, мусор)
- [x] integration BaseSuite: attachments_gc_integration_test.go (ExistingKeys, 3 происхождения сирот, идемпотентность, БД недоступна)
- [x] integration LiveSuite: attachments_gc_http_live_test.go (бинарь сервера, тикер 1s, upload через API)
- [x] прогон целевых integration-сьют
