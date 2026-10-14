# Phase 04: Печать актов SSR (ADR-102)

**Layer:** backend
**Commit**: `feat(phase-6): SSR-печать акта приёма/выдачи, FIRM_* конфиг`

## Files to CREATE
- `internal/features/board/print_ssr.go` — `PrintVM` (design-go-layers.md §Handler Print), `(*SSR).print(c fiber.Ctx) error`: `kind` ∉ `PrintKinds` или `TASK_NOT_FOUND` → `StatusPages.NotFound`; иначе `r.Render(..., "print-"+kind, "print", vm)`, `Content-Type: text/html; charset=utf-8`
- `internal/features/board/print_http_live_test.go` — `PrintLiveSuite` (`testutil.LiveSuite`, TD-8): `TestPrintLiveSuite_Intake_FullContent` (LV-180), `TestPrintLiveSuite_Handover_FullContent` (LV-181), `TestPrintLiveSuite_EmptyFirmAndEdgeCases` (LV-182: `FIRM_*` пустые → блок опущен, `print/other` → 404, нет заказа → 404, `<script>` в description экранирован)
- `web/ssr/board/templates/layouts/print.html` — `{{ define "print" }}`, без сайдбара/`theme.js`, `data-bs-theme="light"`, подключает `/assets/ssr-shared/print.css`
- `web/ssr/board/templates/pages/print-intake.html` — реквизиты (по `{{ if }}` на каждое `FIRM_*`), `#N`, дата, клиент+телефоны, оборудование (тип/бренд/модель/серийный/комплектность/состояние), описание, подписи «Сдал»/«Принял»
- `web/ssr/board/templates/pages/print-handover.html` — то же + услуги со статусом и ценой («по факту» — прочерк), товары qty×цена, итог `task.total_amount`, оплата (`is_paid`/`paid_at`), подписи «Выдал»/«Получил»
- `web/ssr/_shared/print.css` — A4, `@media print` скрывает `.no-print`

## Files to MODIFY
- `internal/features/board/enums.go` — `PrintKindIntake="intake"`, `PrintKindHandover="handover"`, `PrintKinds` (design-go-layers.md §Enum PrintKind)
- `internal/features/board/enums_test.go` — MODIFY: `TestPrintKinds_Values` (UT-003)
- `internal/features/board/card_service.go` — MODIFY: добавить `PrintData(ctx, a, taskID) (*PrintVM, error)` (`Card()` + `CardRepo.PrintHeader`, design-go-layers.md §Service Interface)
- `internal/config/config.go` — MODIFY: `Firm FirmInfo` из `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE` (пустые допустимы, старт не падает)
- `internal/features/board/ssr.go` — MODIFY: сигнатура `NewSSR(r, shell, status *ssr.StatusPages, firm config.FirmInfo, card *CardService)`; регистрация `app.Get("/tasks/:id/print/:kind", a.RequireSSR(), s.print)`

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `TestPrintKinds_Values` (UT-003)
- Live (`PrintLiveSuite`, TD-8): LV-180, LV-181, LV-182

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (скоуп-дефолт §EE-5, BASE=9580596; FULL — миграция 0006 из фазы 01 в диапазоне, весь `./...` с `-race` зелёный)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `print_http_live_test.go` (`testutil.LiveSuite`) — LV-180, LV-181, LV-182 зелёные (TD-8)
- [x] AC-015 (акт приёма), AC-016 (акт выдачи: все услуги со статусом — SD-009, итог = `task.total_amount`) — покрыты
- [x] `FIRM_*` пустые → приложение стартует, блок реквизитов не рендерится (`{{ if }}` на каждое поле, не на весь блок сразу)
- [x] `description_html` рендерится через `template.HTML` (ADR-006: хранится только серверно-экранированный HTML) — `<script>` в LV-182 подтверждённо экранирован
- [x] `RequireSSR` + `Cache-Control: no-store` — на печатном роуте (`/tasks/:id/print/:kind`, оба kind — один роут, as-is каркас)
