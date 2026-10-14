### Phase 2: DB Migration 0003

**Layer:** backend
**Commit**: `feat(db): миграция 0003 — clients, client_phones, equipment_types, equipment`

**SQL / Migrations:**
- `migrations/0003_clients_equipment.up.sql` — DDL: `clients`, `client_phones` (`UNIQUE(client_id,position)`), `equipment_types` (+ `equipment_types_name_lower_uq` на `lower(btrim(name))`, ADR-007), `equipment` (+ `equipment_client_id_idx`, `equipment_type_id_idx`). Полная DDL — design-sql.md `## Clients Migration`.
- `migrations/0003_clients_equipment.down.sql` — простой `DROP TABLE IF EXISTS` (equipment → equipment_types → client_phones → clients), CM-1 MVP, без backfill/rollback-шагов.

**Run after:**
```bash
make build
make migrate-up
```

**Definition of Done:**
- [x] `make build` — OK
- [x] `make migrate-up` — применяется без ошибок на чистой и уже мигрированной (0001/0002) БД (проверено up→down→up на одноразовом контейнере postgres:18-alpine; оркестратор применит на локальный стенд отдельно)
- [x] `make migrate-down` (ручная проверка отката 0003) — таблицы удаляются в правильном порядке FK
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы (миграция сама по себе — гейт следующих backend-фаз, LV-001+) зелёные до перехода (TD-1): `make test` OK, `internal/features/users` integration (dockertest, применяет 0001-0003) — PASS
