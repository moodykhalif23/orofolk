-- ERP / accounting sync (Pack 2 §4.6).

-- ===== Connections =========================================================

-- name: CreateIntegrationConnection :one
INSERT INTO integration_connections (organization_id, provider, kind, endpoint, secret, config, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListIntegrationConnections :many
SELECT * FROM integration_connections WHERE organization_id = $1 ORDER BY id;

-- name: GetIntegrationConnection :one
SELECT * FROM integration_connections WHERE organization_id = $1 AND id = $2;

-- GetIntegrationConnectionByID resolves a connection without org (inbound
-- webhook is connection-scoped; org comes from the row).
-- name: GetIntegrationConnectionByID :one
SELECT * FROM integration_connections WHERE id = $1;

-- ListActiveIntegrationConnections (all orgs) drives the periodic sweep.
-- name: ListActiveIntegrationConnections :many
SELECT * FROM integration_connections WHERE is_active = true ORDER BY id;

-- name: UpdateIntegrationConnection :one
UPDATE integration_connections
   SET provider = $3, kind = $4, endpoint = $5, secret = $6, config = $7, is_active = $8
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- ===== External refs + sync logs ===========================================

-- name: CreateExternalRef :one
INSERT INTO external_refs (connection_id, entity_type, entity_id, external_id, synced_at)
VALUES ($1, $2, $3, $4, now())
RETURNING *;

-- name: CreateSyncLog :one
INSERT INTO sync_logs (organization_id, connection_id, direction, entity_type, entity_id, operation, status, idempotency_key, external_id, error, detail)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id;

-- name: ListSyncLogs :many
SELECT id, connection_id, direction, entity_type, entity_id, operation, status, external_id, error, created_at
FROM sync_logs
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT 200;

-- ===== Outbound work lists (idempotent: skip already-synced) ================

-- name: ListOrdersToSync :many
SELECT o.* FROM orders o
WHERE o.organization_id = $1
  AND o.status IN ('confirmed','processing','shipped','delivered','closed')
  AND NOT EXISTS (
    SELECT 1 FROM external_refs er
    WHERE er.connection_id = $2 AND er.entity_type = 'order' AND er.entity_id = o.id)
ORDER BY o.id
LIMIT $3;

-- name: ListInvoicesToSync :many
SELECT i.* FROM invoices i
JOIN orders o ON o.id = i.order_id
WHERE o.organization_id = $1
  AND i.status IN ('issued','paid','overdue')
  AND NOT EXISTS (
    SELECT 1 FROM external_refs er
    WHERE er.connection_id = $2 AND er.entity_type = 'invoice' AND er.entity_id = i.id)
ORDER BY i.id
LIMIT $3;

-- ===== Inbound master-data apply ===========================================

-- SetInventoryOnHand upserts a stock level (ERP → commerce inventory sync).
-- name: SetInventoryOnHand :exec
INSERT INTO inventory_levels (product_id, warehouse_id, quantity_on_hand)
VALUES ($1, $2, $3)
ON CONFLICT (product_id, warehouse_id)
DO UPDATE SET quantity_on_hand = EXCLUDED.quantity_on_hand, updated_at = now();
