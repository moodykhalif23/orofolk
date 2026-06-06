-- name: ListActiveProducts :many
SELECT id, public_id, sku, name, slug, description, status, attributes, unit
FROM products
WHERE organization_id = $1
  AND status = 'active'
  AND approval_status = 'approved'
  AND deleted_at IS NULL
ORDER BY name
LIMIT $2 OFFSET $3;

-- GetProductBySlug is a storefront read: only approved products are visible
-- (operator products default to 'approved'; unapproved vendor listings are hidden).
-- name: GetProductBySlug :one
SELECT public_id, sku, name, slug, description, status, attributes, unit
FROM products
WHERE organization_id = $1 AND slug = $2 AND approval_status = 'approved' AND deleted_at IS NULL;

-- name: CountActiveProducts :one
SELECT count(*) FROM products
WHERE organization_id = $1 AND status = 'active' AND approval_status = 'approved' AND deleted_at IS NULL;

-- GetBuyableProductIDByPublicID resolves a product id only when the product is
-- buyable from the storefront: approved (operator products default approved;
-- unapproved vendor listings cannot be added to a cart) and not deleted.
-- name: GetBuyableProductIDByPublicID :one
SELECT id FROM products
WHERE organization_id = $1 AND public_id = $2 AND approval_status = 'approved' AND deleted_at IS NULL;
