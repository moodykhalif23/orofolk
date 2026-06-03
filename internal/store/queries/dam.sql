-- Digital Asset Management (Pack 3 §2): media assets, tags, presets, renditions.

-- ===== Assets ==============================================================

-- name: CreateMediaAsset :one
INSERT INTO media_assets (organization_id, url, mime_type, width, height, alt, folder, checksum, size_bytes, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetMediaByChecksum :one
SELECT * FROM media_assets
WHERE organization_id = $1 AND checksum = $2;

-- name: GetMediaByPublicID :one
SELECT * FROM media_assets WHERE public_id = $1;

-- name: GetMediaByID :one
SELECT * FROM media_assets WHERE organization_id = $1 AND id = $2;

-- name: GetMediaByIDInternal :one
SELECT * FROM media_assets WHERE id = $1;

-- name: ListMedia :many
SELECT m.* FROM media_assets m
WHERE m.organization_id = sqlc.arg('org')
  AND (sqlc.narg('folder')::text IS NULL OR m.folder = sqlc.narg('folder'))
  AND (sqlc.narg('tag')::text IS NULL OR EXISTS (
        SELECT 1 FROM media_tags t WHERE t.media_asset_id = m.id AND t.tag = sqlc.narg('tag')))
ORDER BY m.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: UpdateMediaMeta :one
UPDATE media_assets SET alt = $3, folder = $4
WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: SetMediaStatus :exec
UPDATE media_assets SET status = $2 WHERE id = $1;

-- ===== Tags ================================================================

-- name: AddMediaTag :exec
INSERT INTO media_tags (media_asset_id, tag) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteMediaTags :exec
DELETE FROM media_tags WHERE media_asset_id = $1;

-- name: ListMediaTags :many
SELECT tag FROM media_tags WHERE media_asset_id = $1 ORDER BY tag;

-- ===== Presets =============================================================

-- name: ListPresets :many
SELECT * FROM transformation_presets WHERE organization_id = $1 ORDER BY name;

-- name: GetPreset :one
SELECT * FROM transformation_presets WHERE organization_id = $1 AND name = $2;

-- name: CountPresets :one
SELECT count(*) FROM transformation_presets WHERE organization_id = $1;

-- name: CreatePreset :one
INSERT INTO transformation_presets (organization_id, name, width, height, fit, format, quality)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- ===== Renditions ==========================================================

-- name: UpsertRendition :one
INSERT INTO media_renditions (media_asset_id, preset, url, width, height, format, size_bytes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (media_asset_id, preset) DO UPDATE
  SET url = EXCLUDED.url, width = EXCLUDED.width, height = EXCLUDED.height,
      format = EXCLUDED.format, size_bytes = EXCLUDED.size_bytes
RETURNING *;

-- name: GetRendition :one
SELECT * FROM media_renditions WHERE media_asset_id = $1 AND preset = $2;

-- name: ListRenditions :many
SELECT * FROM media_renditions WHERE media_asset_id = $1 ORDER BY preset;

-- name: CountRenditions :one
SELECT count(*) FROM media_renditions WHERE media_asset_id = $1;
