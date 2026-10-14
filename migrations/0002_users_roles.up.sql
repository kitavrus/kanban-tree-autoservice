-- Роли и телефон сотрудника (plan.md §2, D19). login храним в нижнем регистре —
-- вход регистронезависимый (как в прототипе); уникальность — по lower(login).
ALTER TABLE users
    ADD COLUMN role  text NOT NULL DEFAULT 'executor' CHECK (role IN ('admin', 'executor')),
    ADD COLUMN phone text NOT NULL DEFAULT '';

-- CM-1 = MVP: до этой миграции существовал только seed-админ каркаса — делаем его admin.
UPDATE users SET role = 'admin', login = lower(login);

CREATE UNIQUE INDEX users_login_lower_uq ON users (lower(login));
