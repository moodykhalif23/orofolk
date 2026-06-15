-- Outbound webhooks / event subscriptions (Platform roadmap, Phase 0).

-- ===== Endpoints ===========================================================

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (organization_id, url, secret, description, event_types, is_active, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListWebhookEndpoints :many
SELECT * FROM webhook_endpoints WHERE organization_id = $1 ORDER BY id;

-- name: GetWebhookEndpoint :one
SELECT * FROM webhook_endpoints WHERE organization_id = $1 AND id = $2;

-- GetWebhookEndpointByID resolves an endpoint for delivery (org from the row).
-- name: GetWebhookEndpointByID :one
SELECT * FROM webhook_endpoints WHERE id = $1;

-- ListActiveWebhookEndpointsForEvent returns the active endpoints in an org that
-- subscribe to an event (an empty event_types array means "all events").
-- name: ListActiveWebhookEndpointsForEvent :many
SELECT * FROM webhook_endpoints
WHERE organization_id = $1
  AND is_active = true
  AND (cardinality(event_types) = 0 OR sqlc.arg(event)::text = ANY(event_types));

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints
   SET url = $3, description = $4, event_types = $5, is_active = $6
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- RotateWebhookSecret swaps the signing secret.
-- name: RotateWebhookSecret :one
UPDATE webhook_endpoints SET secret = $3
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteWebhookEndpoint :exec
DELETE FROM webhook_endpoints WHERE organization_id = $1 AND id = $2;

-- ===== Deliveries (per-attempt log) ========================================

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (organization_id, endpoint_id, event_type, payload, status, attempt, response_status, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE organization_id = $1 AND endpoint_id = $2
ORDER BY created_at DESC
LIMIT 200;

-- name: GetWebhookDelivery :one
SELECT * FROM webhook_deliveries WHERE organization_id = $1 AND id = $2;
