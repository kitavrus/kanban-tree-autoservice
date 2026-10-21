package repository

// SQL закупок (§4–§6). Числа numeric читаются ::text (без потери точности); суммы строк —
// round(supplier_qty × supplier_price, 2), итог — сумма строк с ценой (DS-009, DS-071).

// orderColumns — шапка + агрегаты строк; алиасы: o — заказ, s — поставщик, u — автор.
const orderColumns = `
o.id::text, o.number, o.supplier_id::text, coalesce(s.name, ''), coalesce(s.phone, ''),
o.status, o.note, o.note_changed_at, o.version,
o.created_by::text, coalesce(nullif(u.name, ''), u.login), o.created_at, o.updated_at,
o.sent_at, o.ready_at, o.picked_up_at, o.cancelled_at, o.cancel_reason,
EXISTS (SELECT 1 FROM users su WHERE su.supplier_id = o.supplier_id AND su.role = 'supplier' AND su.is_active),
(SELECT count(*) FROM purchase_order_items i WHERE i.order_id = o.id),
(SELECT count(*) FROM purchase_order_items i WHERE i.order_id = o.id AND i.supplier_qty IS NOT NULL),
(SELECT sum(round(i.supplier_qty * i.supplier_price, 2)) FROM purchase_order_items i
  WHERE i.order_id = o.id AND i.supplier_qty IS NOT NULL AND i.supplier_price IS NOT NULL)::text,
(o.note_changed_at IS NOT NULL AND o.sent_at IS NOT NULL AND o.note_changed_at > o.sent_at),
EXISTS (SELECT 1 FROM purchase_order_items i WHERE i.order_id = o.id AND i.customer_changed_at IS NOT NULL
  AND (i.supplier_answered_at IS NULL OR i.supplier_answered_at < i.customer_changed_at))`

const orderFrom = `
FROM purchase_orders o
LEFT JOIN suppliers s ON s.id = o.supplier_id
JOIN users u ON u.id = o.created_by`

// listWhere — $1 статус (NULL — все), $2 автор (NULL — все: admin; executor — он сам, DS-060).
const listWhere = `
WHERE o.deleted_at IS NULL
  AND ($1::text IS NULL OR o.status = $1)
  AND ($2::uuid IS NULL OR o.created_by = $2)`

const listSQL = `SELECT ` + orderColumns + orderFrom + listWhere + `
ORDER BY o.created_at DESC, o.id
LIMIT $3 OFFSET $4`

const countSQL = `SELECT count(*) FROM purchase_orders o` + listWhere

// getSQL — карточка: $1 id, $2 автор (NULL — любой).
const getSQL = `SELECT ` + orderColumns + orderFrom + `
WHERE o.id = $1::uuid AND o.deleted_at IS NULL AND ($2::uuid IS NULL OR o.created_by = $2)`

// supplierWhere — кабинет: $1 поставщик из сессии, только sent/ready/picked_up (DS-019).
const supplierWhere = `
WHERE o.supplier_id = $1::uuid AND o.deleted_at IS NULL
  AND o.status IN ('sent', 'ready', 'picked_up')`

const supplierListSQL = `SELECT ` + orderColumns + orderFrom + supplierWhere + `
  AND ($2::text IS NULL OR o.status = $2)
ORDER BY (o.status = 'sent') DESC, o.sent_at DESC NULLS LAST, o.id
LIMIT $3 OFFSET $4`

const supplierCountSQL = `SELECT count(*) FROM purchase_orders o` + supplierWhere + `
  AND ($2::text IS NULL OR o.status = $2)`

const supplierGetSQL = `SELECT ` + orderColumns + orderFrom + supplierWhere + `
  AND o.id = $2::uuid`

// itemsSQL — строки заказов $1 (uuid[]) по порядку.
const itemsSQL = `
SELECT i.id::text, i.order_id::text, i.product_id::text, i.position, i.qty::text, i.reason,
       i.name, i.sku, i.unit, i.supplier_qty::text, i.supplier_price::text, i.supplier_note,
       i.supplier_answered_at, i.customer_changed_at, i.version,
       round(i.supplier_qty * i.supplier_price, 2)::text,
       (i.supplier_qty IS NOT NULL AND i.supplier_qty <> i.qty),
       (i.customer_changed_at IS NOT NULL
        AND (i.supplier_answered_at IS NULL OR i.supplier_answered_at < i.customer_changed_at))
FROM purchase_order_items i
WHERE i.order_id = ANY($1::uuid[])
ORDER BY i.order_id, i.position, i.id`

// headSQL — шапка под блокировкой (переходы §5, правка §6.4); $1 id, $2 поставщик (NULL —
// любой). LOCK — FOR UPDATE либо FOR SHARE.
const headSQL = `
SELECT o.id::text, o.number, o.supplier_id::text, o.status, o.version, o.created_by::text, o.note, o.sent_at,
       (SELECT count(*) FROM purchase_order_items i WHERE i.order_id = o.id),
       EXISTS (SELECT 1 FROM users su WHERE su.supplier_id = o.supplier_id AND su.role = 'supplier' AND su.is_active)
FROM purchase_orders o
WHERE o.id = $1::uuid AND o.deleted_at IS NULL AND ($2::uuid IS NULL OR o.supplier_id = $2)
`

const existingItemsSQL = `
SELECT id::text, product_id::text, qty::text, reason
FROM purchase_order_items WHERE order_id = $1::uuid ORDER BY position, id FOR UPDATE`

// productsSQL — снимок активных, не удалённых запчастей $1 (uuid[]) (DS-021, DS-055).
const productsSQL = `
SELECT id::text, name, sku, unit FROM products
WHERE id = ANY($1::uuid[]) AND is_active AND deleted_at IS NULL`

const supplierAvailableSQL = `
SELECT 1 FROM suppliers WHERE id = $1::uuid AND is_active AND deleted_at IS NULL FOR SHARE`

const insertOrderSQL = `
INSERT INTO purchase_orders (supplier_id, note, created_by) VALUES ($1::uuid, $2, $3::uuid)
RETURNING id::text`

// insertItemSQL — $6 = true: заказ уже отправлялся → «Изменено заказчиком» (DS-048).
const insertItemSQL = `
INSERT INTO purchase_order_items (order_id, product_id, position, qty, reason, name, sku, unit, customer_changed_at)
VALUES ($1::uuid, $2::uuid, $3, $4::numeric, $5, $7, $8, $9, CASE WHEN $6 THEN now() END)`

const deleteMissingItemsSQL = `
DELETE FROM purchase_order_items WHERE order_id = $1::uuid AND NOT (id = ANY($2::uuid[]))`

// updateItemQtySQL — смена количества сбрасывает ответ строки (DS-034, DS-048).
const updateItemQtySQL = `
UPDATE purchase_order_items SET
  qty = $3::numeric, reason = coalesce(nullif($4, ''), reason), position = $5,
  supplier_qty = NULL, supplier_price = NULL, supplier_note = '',
  customer_changed_at = CASE WHEN $6 THEN now() ELSE customer_changed_at END,
  version = version + 1
WHERE id = $1::uuid AND order_id = $2::uuid`

const updateItemPosSQL = `
UPDATE purchase_order_items SET reason = coalesce(nullif($3, ''), reason), position = $4
WHERE id = $1::uuid AND order_id = $2::uuid`

// updateHeadSQL — правка §6.4; в sent изменение примечания ставит note_changed_at (DS-048).
const updateHeadSQL = `
UPDATE purchase_orders SET
  note_changed_at = CASE WHEN status = 'sent' AND note <> $2 THEN now() ELSE note_changed_at END,
  note = $2, version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const refreshSnapshotSQL = `
UPDATE purchase_order_items i SET name = p.name, sku = p.sku, unit = p.unit
FROM products p WHERE i.order_id = $1::uuid AND p.id = i.product_id`

const sendSQL = `
UPDATE purchase_orders SET status = 'sent', sent_by = $2::uuid, sent_at = now(),
  version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const recallSQL = `
UPDATE purchase_orders SET status = 'draft', version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const pickUpSQL = `
UPDATE purchase_orders SET status = 'picked_up', picked_up_by = $2::uuid, picked_up_at = now(),
  version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const cancelSQL = `
UPDATE purchase_orders SET status = 'cancelled', cancelled_by = $2::uuid, cancelled_at = now(),
  cancel_reason = $3, version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const softDeleteSQL = `
UPDATE purchase_orders SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
  version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const restoreSQL = `
UPDATE purchase_orders SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NOT NULL`

const readySQL = `
UPDATE purchase_orders SET status = 'ready', ready_by = $2::uuid, ready_at = now(),
  version = version + 1, updated_at = now()
WHERE id = $1::uuid`

const answerStatesSQL = `
SELECT id::text, version, supplier_qty::text, supplier_price::text
FROM purchase_order_items WHERE order_id = $1::uuid ORDER BY position, id`

const answerItemSQL = `
SELECT id::text, version, supplier_qty::text, supplier_price::text
FROM purchase_order_items WHERE id = $1::uuid AND order_id = $2::uuid FOR UPDATE`

const answerSQL = `
UPDATE purchase_order_items SET
  supplier_qty = $3::numeric, supplier_price = $4::numeric, supplier_note = $5,
  supplier_answered_by = $6::uuid, supplier_answered_at = now(), version = version + 1
WHERE id = $1::uuid AND order_id = $2::uuid`

const orderTotalSQL = `
SELECT sum(round(supplier_qty * supplier_price, 2))::text FROM purchase_order_items
WHERE order_id = $1::uuid AND supplier_qty IS NOT NULL AND supplier_price IS NOT NULL`

// openOrdersSQL — «уже в открытом заказе» (sent/ready, не удалён; DS-049): $1 запчасти, $2 автор
// (NULL — все: admin; executor — свои, DS-078).
const openOrdersSQL = `
SELECT i.product_id::text, o.id::text, o.number, i.qty::text
FROM purchase_order_items i JOIN purchase_orders o ON o.id = i.order_id
WHERE i.product_id = ANY($1::uuid[]) AND o.status IN ('sent', 'ready') AND o.deleted_at IS NULL
  AND ($2::uuid IS NULL OR o.created_by = $2)
ORDER BY o.number`
