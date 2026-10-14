-- users — базовая таблица auth-каркаса (kanban-tree).
-- version — оптимистичная блокировка (§FE-1.1: version в body, не If-Match).
-- gen_random_uuid() требует pgcrypto — создаём ДО таблицы, иначе DEFAULT не резолвится.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name          text NOT NULL DEFAULT '',
    is_active     boolean NOT NULL DEFAULT true,
    version       bigint NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
