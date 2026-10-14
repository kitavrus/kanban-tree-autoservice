### Phase 1: Shared regression guard

**Layer:** backend
**Commit**: `refactor(shared): вынос EscapeLike в pkg/sqlutil, Shell.NavActive, меню «Каталог»`

Риск: код используется существующими `users`/`shell` e2e — ломать нельзя (регресс-гейт AC-006 меню, UT-011).

**Files to CREATE:**
- `pkg/sqlutil/like.go` — `EscapeLike(s string) string` (экранирование `%`/`_`/`\` для ILIKE), перенос логики из `internal/features/users/repo.go`.
- `pkg/sqlutil/like_test.go` — `TestEscapeLike_SpecialChars` (UT-010, часть про `%`/`_`/`\`).

**Files to MODIFY:**
- `internal/features/users/repo.go` — заменить локальную реализацию escape на `sqlutil.EscapeLike`, поведение не меняется.
- `internal/ssr/shell.go` — добавить `func (s Shell) NavActive(url string) bool` (ADR-006: `url=="/"` точное совпадение, иначе `ActivePath==url || strings.HasPrefix(ActivePath, url+"/")`).
- `internal/ssr/shell_test.go` — `TestShell_NavActive` (UT-011: `/` точно; `/clients` на `/clients/abc` true; `/users` на `/users-x` false; `/catalog` на `/catalog/equipment-types` true).
- `internal/ssr/menu.go` — пункт «Услуги и товары» → `{Title: "Каталог", Icon: "🧰", URL: "/catalog", AdminOnly: true}` (ADR-006).
- `web/ssr/_layout/templates/layouts/base.html` — `{{ if $.Shell.NavActive .URL }} active{{ end }}` вместо `eq .URL $.Shell.ActivePath`; `aria-current` сохранить.
- `internal/ssr/render_test.go` — обновить снапшот/ассерт меню, если завязан на старый пункт «Услуги и товары».

**Run after:**
```bash
make build
```

**Tests in this phase:**
- Unit: `TestEscapeLike_SpecialChars`, `TestShell_NavActive`.

**Definition of Done:**
- [x] `make build` — OK
- [x] `make test` — unit OK (`pkg/sqlutil`, `internal/ssr`, `internal/features/users`)
- [x] `make lint` — 0 ошибок
- [ ] Регресс: `cd e2e && npx playwright test tests/shell.spec.ts tests/users.spec.ts --project=chromium --headed --workers=1` зелёный (подсветка `/users`/`/` не сломана, ADR-006 риск) — evidence TD-2 (гонит оркестратор)
- [x] Тесты этой фазы зелёные до перехода (TD-1)
