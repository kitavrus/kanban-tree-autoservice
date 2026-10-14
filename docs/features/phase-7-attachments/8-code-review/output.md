---
stage: 8-code-review
feature: phase-7-attachments
tier: S
verdict: APPROVED
high: 0
medium: 0
low: 3
---
# Code Review: phase-7-attachments — APPROVED
**Review level:** Slim (tier: S) + Sec 18 API-линза (spec-track merged; draft-spec нет → сверка с задачей + D-701…D-717)

Итерация 1 (re-review). Base e74edc5 → HEAD 9ea9f09 (fix-коммит поверх 1632127). Чекеры bundle: 9/9 ✅ (не перегонялись). E2E-улика: `9-validation/evidence/e2e-run.txt` [2026-09-27 17:29:08] — chromium headed, 123 passed / 0 failed. Прогоны оркестратора: unit 12/12, lint 0, `make test-integration` 15/15.

## Закрытие findings итерации 0
| # | Было | Фикс (9ea9f09) | Статус |
|---|---|---|---|
| HIGH-1 | CL.0 desync на upload-роуте: 403/401 до хендлера, остаток тела = следующий запрос | `internal/server/middleware/body_limit.go` — ветка `large(c)` безусловно `SetConnectionClose()` ДО CSRF/auth; обычные роуты читают тело целиком до CSRF, ошибка чтения → `SetConnectionClose()`. LV-211 `TestAttachmentsLiveSuite_NoRequestDesync`: raw TCP + `GET /health` в теле → 403 (без CSRF) и 401 (без сессии) с `Connection: close`, второго ответа нет; обычный POST — контрабанда остаётся телом | закрыт |
| MEDIUM-1 | chunked-тело загрузки без лимита | `internal/features/board/attachments_handler.go` — `capReader{left: AttachmentUploadMaxBody}`, превышение на любом шаге (NextPart/Close/Put) → 413 `REQUEST_TOO_LARGE` в defer + дочитка ≤1 МБ; LV-212 `TestAttachmentsLiveSuite_ChunkedBodyCapped413` + `TestCapReader` | закрыт |
| LOW-1 | AT-001…010 не отмечены | `light-progress.md` — 10/10 `[x]` | закрыт |
| LOW-2 | bidi/Cf в имени | `sanitizeAttachmentName`: `unicode.Is(unicode.Cf, r)` вырезается + unit | закрыт |
| LOW-3/4 | orphan-файлы; nginx `client_max_body_size` | перенесены в «Известное (as-is)» light-progress | принято как follow-up (ниже) |

Проверка фикса capReader: ровно лимит допустим (проба 1 байта при `left<=0`), при превышении частичный файл удаляет `LocalFileStore.Put` (defer по err), строка в БД не создаётся; `err` в defer перекрывается `ErrRequestTooLarge` только при `exceeded` — прочие ошибки (400/404/413 по виду) не маскируются.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/board/attachments_handler.go:~16]** Фича импортирует `internal/server/middleware` ради `ErrRequestTooLarge` — зависимость feature → server-слой. Цикла нет, но sentinel уместнее в `pkg/errorspkg` (общий для middleware и board).
2. **[internal/features/board/attachments_service.go]** Осиротевшие файлы (падение между `Put` и `Insert`, ошибка `removeAttachmentFiles`) — сборщика нет; зафиксировано в as-is, follow-up: сверка `UPLOADS_DIR` ↔ `task_attachments.storage_key` + чистка `.upload-*`.
3. **Прод-периметр (вне репо, /infra-bootstrap):** nginx-location загрузки — `client_max_body_size 501m`; зафиксировано в as-is.

## Секции (Slim + Sec 18)
- **Security (Sec 6):** path traversal — ключ только uuid (`LocalFileStore.path` → `uuid.Validate`), имя в путь не попадает ✅; MIME по сигнатуре, whitelist, HTML/SVG/zip/HEIC → 400, Content-Type из БД + `nosniff` + `private` кэш ✅; `Content-Disposition` ASCII-фолбэк + RFC 5987 `filename*` ✅; XSS — `esc()`/`textContent` ✅; authz 401/403/`ATTACHMENT_FORBIDDEN` ✅; лимиты: BodyLimit до CSRF, прочие роуты 4 МБ, upload 501 МБ по Content-Length и по потоку (capReader) ✅; desync закрыт (Connection: close на upload-роуте, полное чтение тела на прочих) ✅; частичные файлы удаляются ✅; Range single-range/416/поток без OOM ✅.
- **Architecture fit:** handler → service → repo, FileStore за интерфейсом, `WithFiles` без ломки конструкторов фазы 6 ✅ (LOW-1 — импорт middleware).
- **Error Handling:** `errorspkg` sentinel'ы, 23503 → `ErrTaskNotFound`, pgx-ошибки не текут ✅.
- **Tests:** unit (sniff/limits/sanitize+Cf/Content-Disposition/parseRange/IsAttachmentUpload/capReader/FileStore); live TD-8 LV-201…212; E2E AT-001…010 без `waitForTimeout`, overflow 360px ✅.
- **Rollback / миграции:** 0007 up/down (CM-1 MVP), каскад FK + удаление файлов после commit под `FOR UPDATE` (LV-210) ✅.
- **ui-dialog.js (D-717):** очередь не зависает (reject → `then(noop, noop)`), `form[data-confirm]` не изменён ✅.
- **Sec 18 (API-линза):** kebab-URL, snake_case JSON; `{card, warnings}`; `card.attachments` всегда массив, поля non-null ↔ NOT NULL; list-эндпоинта нет (D-701) → конверт n/a; N+1 нет; все ошибки — свои коды через `errorspkg` ✅.
- **Swagger (Sec 15):** `swagger:no` → n/a. **4.1 UUID:** `pkg/ids` в проекте нет, `google/uuid` — as-is стандарт репо → n/a.

## Соответствие коду дизайну
- Решения D-701…D-717 ↔ код — ДА
- Error handling по `errorspkg` — ДА
- Infrastructure (`UPLOADS_DIR` в `.env.example`) — ДА
- Swagger / Monitoring / Rollout / Runbook / Perf Budget — N/A (S-tier, swagger:no); spans `attachments.upload|delete` есть

## Статус фаз
- Все работы и тест-сценарии light-progress `[x]` — ДА.

## Итог
APPROVED: блокер desync и MEDIUM chunked-лимит закрыты фиксом 9ea9f09 с регресс-тестами LV-211/LV-212; остаются 3 LOW (follow-up).
