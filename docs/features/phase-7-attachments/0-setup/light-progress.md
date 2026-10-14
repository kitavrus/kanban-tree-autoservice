# phase-7-attachments — light-progress (анти-компакт чеклист)

## Работы
- [x] Миграция 0007_task_attachments (up/down)
- [x] internal/filestore: FileStore (Put/Open/Delete) + LocalFileStore + unit-тесты
- [x] config: UPLOADS_DIR (дефолт ./uploads), .env.example
- [x] middleware.BodyLimit (дефолт 4 МБ для всех, upload-роут — 510 МБ) + unit-тесты; fiber StreamRequestBody
- [x] board: sniff/kind/limits/filename (attachments_validate.go) + unit-тесты
- [x] board: repo (insert/owner/delete/list/keys by task) + card.attachments в GET card
- [x] board: сервис Upload/Delete/File + каскад файлов в TaskService.Delete
- [x] board: handler (multipart stream, отдача Range/nosniff/disposition/cache) + routes + router wiring
- [x] Live-сьюта attachments_http_live_test.go (LV-2xx)
- [x] Frontend: раздел «Вложения» в карточке (board.html + task-card.js + app.css)
- [x] E2E: e2e/tests/order-attachments.spec.ts + POM OrderCardPage (AT-NNN)
- [x] Гейт: make build / go test ./... -race / make lint / integration board+server / node --check / tsc / playwright --list

## Тест-сценарии
Live (Go, LiveSuite):
- [x] LV-201 upload фото/видео/PDF → 201 {card,warnings}, card.attachments, файл на диске под uuid
- [x] LV-202 GET file: Content-Type из БД, inline + filename*, nosniff, Cache-Control private, тело совпадает
- [x] LV-203 Range: 206 + Content-Range; 416 на неудовлетворимый
- [x] LV-204 MIME-подмена (html под .jpg) → 400 ATTACHMENT_TYPE_UNSUPPORTED; нет строки/файла
- [x] LV-205 лимит фото > 20 МБ → 413 ATTACHMENT_TOO_LARGE, файл не остался
- [x] LV-206 Content-Length > лимита роута → 413 (без чтения тела); обычный роут > 4 МБ → 413
- [x] LV-207 удаление: чужой executor 403 ATTACHMENT_FORBIDDEN, автор 200, admin 200, повтор 404
- [x] LV-208 без сессии 401 (upload/get/delete); без CSRF 403
- [x] LV-209 невалидные id → 404; нет файла в multipart → 400
- [x] LV-210 удаление заказа → строки каскадом + файлы удалены с диска
- [x] LV-211 нет request desync: отказ 403 (CSRF)/401 (без сессии) на upload-роуте → Connection: close, контрабандный запрос из тела не исполнен; обычный роут — тело дочитано (мутационная проверка: без close тест падает)
- [x] LV-212 chunked-тело upload > лимита роута → 413 REQUEST_TOO_LARGE
Unit:
- [x] sniff (png/jpeg/gif/webp/mp4/mov/webm/pdf/html/svg/zip), лимиты, sanitize имени, Content-Disposition
- [x] parseRange
- [x] LocalFileStore Put/Open/Delete/too-large/invalid key
- [x] BodyLimit middleware — покрыт live LV-206 (app.Test для новых HTTP-тестов запрещён §TD-8); предикат IsAttachmentUpload — unit
E2E (AT-NNN):
- [x] AT-001 загрузка фото → миниатюра, открытие в полном размере
- [x] AT-002 загрузка видео → <video controls>
- [x] AT-003 загрузка PDF → ссылка
- [x] AT-004 неподдерживаемый тип → понятная ошибка, список не изменился
- [x] AT-005 удаление с confirm (отмена → осталось; подтвердить → исчезло)
- [x] AT-006 новый заказ — раздел вложений заблокирован до сохранения
- [x] AT-007 executor не видит кнопку удаления чужого вложения; своё — удаляет
- [x] AT-008 файл > лимита — отказ на клиенте без запроса
- [x] AT-009 во время загрузки кнопка заблокирована, прогресс виден
- [x] AT-010 нет горизонтального переполнения раздела на 360px
(AT-001…010 зелёные: полный E2E оркестратора 123/123, 9-validation/evidence/e2e-run.txt)

## Решения
- D-701 Ответ мутаций вложений — конверт фазы 6 `{card, warnings}` (warnings всегда []), вложения — `card.attachments[]` (id, kind, original_name, content_type, size, user_id, author_name, created_at, url, can_delete); отдельного списочного эндпоинта нет (карточка грузится одним GET).
- D-702 Роуты (§API-1): POST /api/v1/tasks/:id/attachments (multipart, поле `file`, 201), GET /api/v1/attachments/:id/file, DELETE /api/v1/attachments/:id.
- D-703 Права: загрузка/просмотр — любой авторизованный (как карточка фазы 6, D19); удаление — автор или admin, иначе 403 ATTACHMENT_FORBIDDEN.
- D-704 Лимит тела: fiber StreamRequestBody=true + DisablePreParseMultipartForm=true; общий 4 МБ для всех роутов держит новый middleware.BodyLimit (стоит ДО CSRF), для POST …/attachments — 501 МБ по Content-Length; файл пишется потоком в FileStore, в память не грузится.
- D-705 Поддерживаемые типы (только по сигнатуре): фото JPEG/PNG/GIF/WebP; видео MP4/WebM/MOV(ftyp qt); PDF. SVG/HTML/HEIC/прочее → 400 ATTACHMENT_TYPE_UNSUPPORTED. Лимит — по виду после сниффинга: 413 ATTACHMENT_TOO_LARGE с текстом по виду + details.max_bytes.
- D-706 Range — собственная реализация одного диапазона по семантике http.ServeContent (206/416/Accept-Ranges), тело потоком SetBodyStream: fasthttpadaptor+ServeContent либо буферизует ответ в памяти, либо теряет Content-Length. Несколько диапазонов → 200 целиком (RFC 7233 допускает).
- D-707 Cache-Control: `private, max-age=3600` (контент по id неизменяем). CSP sandbox не ставим (ломает встроенный PDF-вьюер); защита от XSS — белый список типов + nosniff.
- D-708 FileStore — пакет internal/filestore (Put/Open/Delete, ключ только uuid — защита от traversal), LocalFileStore пишет temp+rename. CardService/TaskService получают его сеттером WithFiles (конструкторы и тесты фазы 6 не меняются).
- D-709 Удаление заказа: ключи файлов читаются в той же tx под FOR UPDATE заказа, строки уходят каскадом FK, файлы удаляются после commit (ошибка — slog.Error, операция успешна). Параллельная загрузка в удаляемый заказ ждёт commit и получает 23503 → 404, её файл удаляется.
- D-710 Ошибка 413 на вложении: хендлер дочитывает остаток тела (≤ лимита роута), чтобы браузер получил ответ, а не обрыв; ранний отказ middleware — Connection: close.
- D-711 docker-compose.yml не содержит сервиса приложения (оно запускается go run) → volume uploads не добавлялся; uploads/ уже в .gitignore; UPLOADS_DIR добавлен в .env.example.
- D-712 Сиды/seed-demo вложений не получают (демо-файлов нет); fixture-sync — фикстуры генерируются в тестах (Go и E2E).
- D-713 Фото открывается в полный размер ссылкой target=_blank (inline Content-Disposition), без лайтбокса поверх модалки; видео — <video controls preload=metadata>; PDF — ссылка-иконка.
- D-714 Загрузка — по одному файлу за раз (XHR ради upload-прогресса); ранняя клиентская проверка размера (фото/PDF 20 МБ, видео 500 МБ по file.type), сервер — источник истины. Ответ загрузки устарел по seq (параллельные row-мутации) → loadCard.
- D-715 BodyLimit для chunked-тела сверх лимита дочитывает хвост ≤1 МБ перед 413 (иначе клиент ловит broken pipe вместо ответа); при Content-Length > лимита тело не читается вовсе.
- D-716 subtask_products_position_integration_test (фаза 6) опирался на «последняя миграция = 0006» (Steps(-1/+1)); переведён на явные Migrate(5)/Migrate(6) + Up() в defer — иначе 0007 ломал тест.
- D-717 (fix-loop 1, AT-005) ui-dialog.js: диалоги поставлены в очередь — следующий показывается после hidden.bs.modal предыдущего (узел удалён). Корень: повторный uiConfirm во время fade-out прошлого давал два [data-testid=ui-dialog] + два backdrop одновременно (временно, до конца анимации; постоянной утечки не было). Тест/POM не менялись.
- D-718 (review HIGH, CL.0 desync) upload-роут: Connection: close безусловно, ставится в BodyLimit ДО CSRF/auth; обычные роуты — тело читается целиком в BodyLimit до любых хендлеров, иначе (ошибка чтения/413) — close. Инвариант: ответ без полностью прочитанного тела всегда закрывает соединение.
- D-719 (review MEDIUM) Upload читает тело через capReader(AttachmentUploadMaxBody): chunked сверх лимита → 413 REQUEST_TOO_LARGE (+ дочитка хвоста ≤1 МБ, чтобы клиент получил ответ).
- D-720 (review LOW) sanitizeAttachmentName убирает также Unicode Cf (bidi RLO/isolates, zero-width, BOM).

## Известное (as-is, вне скоупа фазы)
- Нет сборщика осиротевших файлов (падение процесса между Put и Insert; ошибка удаления файла только логируется; `.upload-*` после краша). Follow-up: периодическая сверка UPLOADS_DIR ↔ task_attachments.storage_key + чистка `.upload-*` старше N часов.
- Прод-периметр (/infra-bootstrap): nginx `client_max_body_size 501m` для `/api/v1/tasks/*/attachments`, прочие роуты — прежний лимит.
