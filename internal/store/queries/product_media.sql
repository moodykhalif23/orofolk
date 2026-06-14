-- Product images — gallery photos for a product, linked to DAM media_assets.
-- A product is capped at 5 images (enforced in the handler). Only type='image'
-- rows are treated as gallery photos here.

-- name: ListProductImages :many
SELECT pm.id, pm.product_id, pm.media_asset_id, pm.url, pm.alt, pm.sort_order,
       ma.status AS asset_status, ma.width, ma.height
FROM product_media pm
LEFT JOIN media_assets ma ON ma.id = pm.media_asset_id
WHERE pm.product_id = $1 AND pm.type = 'image'
ORDER BY pm.sort_order, pm.id;

-- ListStorefrontProductImagesBySlug returns a product's gallery images for the
-- storefront PDP, keyed on slug+org so the internal id never leaves the API.
-- name: ListStorefrontProductImagesBySlug :many
SELECT pm.url, pm.alt
FROM product_media pm
JOIN products p ON p.id = pm.product_id
WHERE p.organization_id = $1 AND p.slug = $2
  AND p.approval_status = 'approved' AND p.deleted_at IS NULL
  AND pm.type = 'image'
ORDER BY pm.sort_order, pm.id;

-- PrimaryImagesForProducts returns the first gallery image per product for a set
-- of products (admin list thumbnails) — one round-trip, no N+1.
-- name: PrimaryImagesForProducts :many
SELECT DISTINCT ON (pm.product_id) pm.product_id, pm.url
FROM product_media pm
WHERE pm.product_id = ANY($1::bigint[]) AND pm.type = 'image'
ORDER BY pm.product_id, pm.sort_order, pm.id;

-- name: CountProductImages :one
SELECT count(*) FROM product_media WHERE product_id = $1 AND type = 'image';

-- name: MaxProductImageSort :one
SELECT COALESCE(MAX(sort_order), -1)::int FROM product_media WHERE product_id = $1 AND type = 'image';

-- name: CreateProductImage :one
INSERT INTO product_media (product_id, media_asset_id, url, type, alt, sort_order)
VALUES ($1, $2, $3, 'image', $4, $5)
RETURNING id, product_id, media_asset_id, url, alt, sort_order;

-- name: DeleteProductImage :execrows
DELETE FROM product_media WHERE id = $1 AND product_id = $2 AND type = 'image';

-- GetMediaAssetForOrg validates that a media asset belongs to the caller's org
-- and returns its servable URL (so the product image can denormalize it).
-- name: GetMediaAssetForOrg :one
SELECT id, url FROM media_assets WHERE id = $1 AND organization_id = $2;

-- ExportProductsAdmin streams the full (non-deleted) catalog for CSV export.
-- name: ExportProductsAdmin :many
SELECT sku, type, name, slug, description, status, unit, attributes, cost_price
FROM products
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY sku;
