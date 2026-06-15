-- Generic data modeling (Platform roadmap, Phase 2): custom object types, their
-- field schema, and records validated against that schema.

-- ===== Object types ========================================================

-- name: CreateObjectType :one
INSERT INTO object_types (organization_id, code, label, label_plural, description, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListObjectTypes :many
SELECT * FROM object_types WHERE organization_id = $1 ORDER BY label;

-- name: GetObjectTypeByCode :one
SELECT * FROM object_types WHERE organization_id = $1 AND code = $2;

-- name: GetObjectType :one
SELECT * FROM object_types WHERE organization_id = $1 AND id = $2;

-- name: UpdateObjectType :one
UPDATE object_types
SET label = $3, label_plural = $4, description = $5, is_active = $6, updated_at = now()
WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteObjectType :exec
DELETE FROM object_types WHERE organization_id = $1 AND id = $2;

-- name: CountObjectRecordsForType :one
SELECT count(*) FROM object_records WHERE object_type_id = $1 AND deleted_at IS NULL;

-- ===== Object fields =======================================================

-- name: CreateObjectField :one
INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, options, validation, is_required, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListObjectFieldsForType :many
SELECT * FROM object_fields WHERE object_type_id = $1 ORDER BY sort_order, label;

-- name: GetObjectField :one
SELECT * FROM object_fields WHERE organization_id = $1 AND id = $2;

-- name: UpdateObjectField :one
UPDATE object_fields
SET label = $3, data_type = $4, options = $5, validation = $6, is_required = $7, sort_order = $8
WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteObjectField :exec
DELETE FROM object_fields WHERE organization_id = $1 AND id = $2;

-- ===== Object records ======================================================

-- name: CreateObjectRecord :one
INSERT INTO object_records (object_type_id, organization_id, data)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListObjectRecords :many
SELECT * FROM object_records
WHERE organization_id = $1 AND object_type_id = $2 AND deleted_at IS NULL
ORDER BY id DESC
LIMIT $3 OFFSET $4;

-- name: CountObjectRecords :one
SELECT count(*) FROM object_records
WHERE organization_id = $1 AND object_type_id = $2 AND deleted_at IS NULL;

-- name: GetObjectRecord :one
SELECT * FROM object_records
WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: UpdateObjectRecord :one
UPDATE object_records
SET data = $3, updated_at = now()
WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteObjectRecord :execrows
UPDATE object_records
SET deleted_at = now()
WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL;

-- GetObjectRecordIDByField finds a live record of a type whose data has a given
-- value at a field code — the import engine's upsert match (slice 3).
-- name: GetObjectRecordIDByField :one
SELECT id FROM object_records
WHERE organization_id = $1 AND object_type_id = $2 AND deleted_at IS NULL
  AND data ->> sqlc.arg(field)::text = sqlc.arg(value)::text
ORDER BY id
LIMIT 1;
