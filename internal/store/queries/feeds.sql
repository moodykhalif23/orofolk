-- Syndication feeds (Platform roadmap, Phase 4). A feed projects a source
-- through a field mapping into a channel format; the row generation reads the
-- source via the queries below.

-- name: CreateFeed :one
INSERT INTO feeds (organization_id, name, source, channel, format, mapping, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE organization_id = $1 AND id = $2;

-- name: ListFeeds :many
SELECT * FROM feeds WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: UpdateFeed :one
UPDATE feeds
   SET name = $3, source = $4, channel = $5, format = $6, mapping = $7, is_active = $8, updated_at = now()
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- name: DeleteFeed :exec
DELETE FROM feeds WHERE organization_id = $1 AND id = $2;

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
