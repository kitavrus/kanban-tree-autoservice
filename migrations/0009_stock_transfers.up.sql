-- Перемещения товаров между складами (подфаза B, docs/features/warehouses/draft-spec.md
-- DS-006/DS-007, D50–D55). Номер перемещения — сквозной bigserial (#N).

CREATE TABLE stock_transfers (
    id                bigserial PRIMARY KEY,
    from_warehouse_id uuid NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    to_warehouse_id   uuid NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    status            text NOT NULL DEFAULT 'requested'
                      CHECK (status IN ('requested', 'in_transit', 'received', 'cancelled')),
    note              text NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    created_by        uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    sent_by           uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    sent_at           timestamptz NULL,
    received_by       uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    received_at       timestamptz NULL,
    cancelled_by      uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    cancelled_at      timestamptz NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    version           int NOT NULL DEFAULT 1,
    CONSTRAINT stock_transfers_distinct_warehouses CHECK (from_warehouse_id <> to_warehouse_id)
);
CREATE INDEX stock_transfers_status_idx ON stock_transfers (status);
CREATE INDEX stock_transfers_from_warehouse_id_idx ON stock_transfers (from_warehouse_id);
CREATE INDEX stock_transfers_to_warehouse_id_idx ON stock_transfers (to_warehouse_id);

CREATE TABLE stock_transfer_items (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id  bigint NOT NULL REFERENCES stock_transfers(id) ON DELETE CASCADE,
    product_id   uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    position     int NOT NULL,
    qty_sent     numeric(12,3) NOT NULL CHECK (qty_sent > 0),
    qty_received numeric(12,3) NULL CHECK (qty_received >= 0 AND qty_received <= qty_sent),
    UNIQUE (transfer_id, product_id)
);
CREATE INDEX stock_transfer_items_product_id_idx ON stock_transfer_items (product_id);

-- DS-007: ссылка движения на перемещение (колонка добавлена в 0008).
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_transfer_id_fkey
    FOREIGN KEY (transfer_id) REFERENCES stock_transfers(id) ON DELETE SET NULL;
CREATE INDEX stock_movements_transfer_id_idx ON stock_movements (transfer_id);
