-- Откат части soft-delete-clients: отметки удаления стираются — мягко удалённые клиенты и
-- единицы оборудования снова становятся действующими; «кто, когда, почему» теряются
-- безвозвратно. Оживают ВСЕ удалённые единицы: и ушедшие вместе с клиентом
-- (deleted_with_client), и удалённые отдельно у действующего клиента — различие между ними
-- теряется вместе с колонкой. Откат выполним на любых данных: названия клиентов, серийные
-- номера и телефоны не уникальны; удалённые единицы были у клиента и не стояли в документах,
-- поэтому место хранения и документы оживление не затрагивает.
DROP INDEX IF EXISTS equipment_deleted_at_idx;
ALTER TABLE equipment
    DROP CONSTRAINT IF EXISTS equipment_deleted_with_client_chk,
    DROP CONSTRAINT IF EXISTS equipment_deleted_chk,
    DROP COLUMN IF EXISTS deleted_with_client,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS clients_deleted_at_idx;
ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;
