### Phase 9: Hardening — concurrency, права, XSS, adaptive/themes, полный регресс

**Layer:** frontend
**Commit**: `test(phase-3): конкурентность, executor-права, XSS, adaptive/themes, полный E2E-регресс`
**Покрывает:** AC-005 (CL-020), AC-013 (LV-040/041 полностью), AC-014 (adaptive:min + themes:both).

Функциональность уже реализована фазами 1–8 — эта фаза добавляет только кросс-cutting тесты и фиксы найденных дефектов (не первое появление функционального E2E, TD-1).

**Files to CREATE:**
- `internal/features/clients/clients_http_live_test.go` — MODIFY: добавить LV-020 (если не покрыт Phase 5 полностью — 2 Browser admin, `sync.WaitGroup`, одновременный PUT), LV-040/LV-041 полный набор (executor across API+SSR).
- `e2e/tests/clients.spec.ts` — MODIFY: добавить CL-011 (XSS: `<img src=x onerror>` в компании/телефоне/характеристике — текстом, не исполняется), CL-020 (2 browser-контекста, конфликт версий → «Открыть актуальную версию»), CL-021 (adaptive:min 360×640/1280×800 `scrollWidth<=clientWidth` + тёмная тема без overflow).
- `e2e/tests/equipment.spec.ts` — MODIFY: добавить EQ-021 (adaptive:min форма единицы 360/1280, перенос строк характеристик).
- `e2e/tests/equipment-types.spec.ts` — MODIFY: добавить ET-021 (adaptive:min список типов 360/1280).

**Files to MODIFY (только если хардening вскрывает дефект):**
- Точечные фиксы в файлах Phase 1–8 по факту найденного (без новых файлов) — фиксировать в этом же коммите (fix-before-next, TD-1).

**Run after:**
```bash
make test-integration-scoped
cd e2e && npx tsc --noEmit -p .
```

**Tests in this phase:**
- Live: LV-020, LV-040, LV-041 (полный набор executor-сценариев API+SSR).
- E2E: CL-011, CL-020, CL-021, EQ-021, ET-021.

**Definition of Done:**
- [ ] `make test-integration-scoped` — LV-020/040/041 зелёные
- [ ] `npx playwright test tests/clients.spec.ts tests/equipment.spec.ts tests/equipment-types.spec.ts --project=chromium --headed --workers=1` — CL-011/020/021, EQ-021, ET-021 зелёные, evidence TD-2 сохранён
- [ ] Concurrency (TD-4/CL-020): ровно один 200 / один 409, ввод проигравшего не потерян, «Открыть актуальную версию» ведёт к данным победителя
- [ ] XSS (CL-011): payload отображается как текст в списке/карточке/confirm, не исполняется
- [ ] Adaptive/themes (US-1 adaptive:min): нет horizontal overflow на 360px и 1280px; тёмная тема рендерится без overflow (themes:both)
- [ ] **Полный E2E-регресс сюиты (сводный гейт Executing, EE-1):** `make e2e-full` — 0 hard-fail (flaky — строка в отчёте, не блокер, EE-2)
- [ ] Полный `make test-integration` (§EE-5, если FULL-триггер не сработал ранее — иначе зафиксировать «скоуп: task» в Report)
- [ ] Тесты этой фазы зелёные — Phase 3 готова к Validation
