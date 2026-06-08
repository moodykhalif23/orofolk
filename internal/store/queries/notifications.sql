-- name: CreateNotification :one
INSERT INTO notifications (
  organization_id, audience, recipient_id, customer_id, vendor_id,
  type, title, body, link, severity, data
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE organization_id = $1 AND audience = $2 AND recipient_id = $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications
WHERE organization_id = $1 AND audience = $2 AND recipient_id = $3
  AND read_at IS NULL;

-- name: MarkNotificationRead :one
UPDATE notifications SET read_at = now()
WHERE public_id = $1 AND organization_id = $2 AND audience = $3 AND recipient_id = $4
  AND read_at IS NULL
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = now()
WHERE organization_id = $1 AND audience = $2 AND recipient_id = $3
  AND read_at IS NULL;

-- Recipient fan-out helpers. Used by the worker to expand a domain event into
-- one notification row per recipient user.

-- name: ListActiveAdminUserIDs :many
SELECT id FROM users
WHERE organization_id = $1 AND is_active = true;

-- name: ListActiveCustomerUserIDs :many
SELECT id FROM customer_users
WHERE customer_id = $1 AND is_active = true;

-- name: ListActiveCustomerApproverIDs :many
SELECT id FROM customer_users
WHERE customer_id = $1 AND is_active = true AND role IN ('approver', 'admin');

-- name: ListActiveVendorUserIDs :many
SELECT id FROM vendor_users
WHERE vendor_id = $1 AND is_active = true;
