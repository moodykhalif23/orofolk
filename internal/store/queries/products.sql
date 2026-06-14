-- name: ListActiveProducts :many
SELECT p.id, p.public_id, p.sku, p.name, p.slug, p.description, p.status, p.attributes, p.unit,
       COALESCE((SELECT pm.url FROM product_media pm
        WHERE pm.product_id = p.id AND pm.type = 'image'
        ORDER BY pm.sort_order, pm.id LIMIT 1), '')::text AS image_url,
       COALESCE((SELECT ROUND(AVG(pr.rating), 2) FROM product_reviews pr
        WHERE pr.product_id = p.id AND pr.status = 'approved'), 0)::numeric AS rating_avg,
       COALESCE((SELECT count(*) FROM product_reviews pr
        WHERE pr.product_id = p.id AND pr.status = 'approved'), 0)::bigint AS rating_count
FROM products p
WHERE p.organization_id = $1
  AND p.status = 'active'
  AND p.approval_status = 'approved'
  AND p.deleted_at IS NULL
ORDER BY p.name
LIMIT $2 OFFSET $3;

-- SuggestProducts powers the storefront search typeahead: name/SKU substring
-- matches on visible products. $2 is the raw term; $3 the result cap.
-- name: SuggestProducts :many
SELECT name, slug, sku
FROM products
WHERE organization_id = $1
  AND status = 'active' AND approval_status = 'approved' AND deleted_at IS NULL
  AND (name ILIKE '%' || $2 || '%' OR sku ILIKE '%' || $2 || '%')
ORDER BY name
LIMIT $3;

-- GetProductIDBySlug resolves a visible product's internal id from its slug
-- (storefront reviews: list/create keyed on slug).
-- name: GetProductIDBySlug :one
SELECT id FROM products
WHERE organization_id = $1 AND slug = $2 AND approval_status = 'approved' AND deleted_at IS NULL;

-- GetProductBySlug is a storefront read: only approved products are visible
-- (operator products default to 'approved'; unapproved vendor listings are hidden).
-- name: GetProductBySlug :one
SELECT public_id, sku, name, slug, description, status, attributes, unit
FROM products
WHERE organization_id = $1 AND slug = $2 AND approval_status = 'approved' AND deleted_at IS NULL;

-- name: CountActiveProducts :one
SELECT count(*) FROM products
WHERE organization_id = $1 AND status = 'active' AND approval_status = 'approved' AND deleted_at IS NULL;

-- GetProductVendorBySlug returns the marketplace vendor name for a product, when
-- it is vendor-owned (no row for operator/house products). Storefront "sold by".
-- name: GetProductVendorBySlug :one
SELECT v.name AS vendor_name
FROM products p
JOIN vendors v ON v.id = p.vendor_id
WHERE p.organization_id = $1 AND p.slug = $2 AND p.deleted_at IS NULL;

-- GetBuyableProductIDByPublicID resolves a product id only when the product is
-- buyable from the storefront: approved (operator products default approved;
-- unapproved vendor listings cannot be added to a cart) and not deleted.
-- name: GetBuyableProductIDByPublicID :one
SELECT id FROM products
WHERE organization_id = $1 AND public_id = $2 AND approval_status = 'approved' AND deleted_at IS NULL;
