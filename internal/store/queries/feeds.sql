-- Syndication feeds (Platform roadmap, Phase 4). A feed projects a source
-- through a field mapping into a channel format; the row generation reads the
-- source via the queries below.

-- name: CreateFeed :one
INSERT INTO feeds (organization_id, name, source, channel, format, mapping, is_active, schedule, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE organization_id = $1 AND id = $2;

-- name: GetFeedByPublicID :one
SELECT * FROM feeds WHERE public_id = $1;

-- name: ListFeeds :many
SELECT * FROM feeds WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: UpdateFeed :one
UPDATE feeds
   SET name = $3, source = $4, channel = $5, format = $6, mapping = $7, is_active = $8,
       schedule = $9, next_run_at = $10, updated_at = now()
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteFeed :exec
DELETE FROM feeds WHERE organization_id = $1 AND id = $2;

-- ListDueFeeds returns scheduled feeds whose next run has arrived (cross-org; the
-- scheduler sweep runs unscoped). Bounded so one sweep can't run unboundedly.
-- name: ListDueFeeds :many
SELECT * FROM feeds
WHERE is_active AND schedule <> 'manual' AND next_run_at IS NOT NULL AND next_run_at <= $1
ORDER BY next_run_at
LIMIT 500;

-- MarkFeedBuilt records a successful build: artifact location/size, the build
-- time, and the next scheduled run (null for a manual build).
-- name: MarkFeedBuilt :exec
UPDATE feeds
   SET last_built_at = now(), last_artifact_key = $3, last_bytes = $4, last_error = '', next_run_at = $5
 WHERE organization_id = $1 AND id = $2;

-- MarkFeedBuildError records a failed build and advances next_run_at so the
-- scheduler doesn't hot-loop on a broken feed.
-- name: MarkFeedBuildError :exec
UPDATE feeds
   SET last_error = $3, next_run_at = $4
 WHERE organization_id = $1 AND id = $2;

-- ListProductsForFeed streams a tenant's live products as a feed source: the
-- structural columns, the primary image, and the attributes JSONB (flattened to
-- attr.<code> source fields by the engine). Capped by $2.
-- name: ListProductsForFeed :many
SELECT p.public_id, p.sku, p.name, p.slug, p.description, p.status, p.type, p.unit, p.attributes,
       COALESCE((SELECT pm.url FROM product_media pm
         WHERE pm.product_id = p.id AND pm.type = 'image'
         ORDER BY pm.sort_order, pm.id LIMIT 1), '')::text AS image_url,
       p.created_at
FROM products p
WHERE p.organization_id = $1 AND p.deleted_at IS NULL
ORDER BY p.id
LIMIT $2;
