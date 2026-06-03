-- Field-sales offline sync (Pack 3 §4).

-- ===== Devices =============================================================

-- UpsertFieldDevice registers a device on first contact and refreshes its
-- last-seen on every sync.
-- name: UpsertFieldDevice :one
INSERT INTO field_devices (user_id, device_uuid, platform, last_seen_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (user_id, device_uuid)
DO UPDATE SET last_seen_at = now(), platform = COALESCE(EXCLUDED.platform, field_devices.platform)
RETURNING *;

-- name: SetDeviceCursor :exec
UPDATE field_devices SET last_sync_cursor = $2 WHERE id = $1;

-- name: ListFieldDevices :many
SELECT d.id, d.user_id, u.email AS user_email, d.device_uuid, d.platform,
       d.last_sync_cursor, d.last_seen_at, d.created_at
FROM field_devices d
JOIN users u ON u.id = d.user_id
WHERE u.organization_id = $1
ORDER BY d.last_seen_at DESC NULLS LAST;

-- ===== Change log (cursor outbox) ==========================================

-- name: CreateChangeLog :one
INSERT INTO change_log (organization_id, scope_rep_id, entity_type, entity_id, op, payload)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- PullChanges returns the rep's scoped delta after a cursor (rep-scoped rows
-- plus globals), bounded by a batch limit and resumable by id.
-- name: PullChanges :many
SELECT id, entity_type, entity_id, op, payload, created_at
FROM change_log
WHERE organization_id = $1
  AND (scope_rep_id = $2 OR scope_rep_id IS NULL)
  AND id > $3
ORDER BY id
LIMIT $4;

-- MaxScopedCursor is the current high-water mark for the rep's scope (used when
-- a pull returns no rows so the client still advances its cursor).
-- name: MaxScopedCursor :one
SELECT COALESCE(max(id), $3)::bigint AS cursor
FROM change_log
WHERE organization_id = $1 AND (scope_rep_id = $2 OR scope_rep_id IS NULL);

-- ===== Push idempotency log ================================================

-- name: GetPushLog :one
SELECT * FROM sync_push_log WHERE device_id = $1 AND client_change_id = $2;

-- name: CreatePushLog :one
INSERT INTO sync_push_log (device_id, client_change_id, entity_type, op, status, server_entity_id, detail)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- ===== Activity get/update (writable on device) ============================

-- name: GetActivity :one
SELECT * FROM activities WHERE organization_id = $1 AND id = $2;

-- name: UpdateActivity :one
UPDATE activities SET subject = $3, body = $4, status = $5
WHERE organization_id = $1 AND id = $2
RETURNING *;
