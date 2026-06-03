-- Configure-Price-Quote (PRD §8). Loads the configuration model and stores
-- priced, configured lines on quotes.

-- ===== Product config (base price + configurable flag) =====================

-- name: UpsertProductConfig :one
INSERT INTO product_configs (product_id, base_price, currency, is_active)
VALUES ($1, $2, $3, $4)
ON CONFLICT (product_id) DO UPDATE
  SET base_price = EXCLUDED.base_price, currency = EXCLUDED.currency, is_active = EXCLUDED.is_active
RETURNING *;

-- name: GetProductConfig :one
SELECT * FROM product_configs WHERE product_id = $1;

-- GetCpqProduct authorizes a product-scoped operation against the caller's org.
-- name: GetCpqProduct :one
SELECT id FROM products WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL;

-- GetProductIDByPublicIDGlobal resolves a product id from its (globally unique)
-- public_id without org context, for the unauthenticated storefront configurator.
-- name: GetProductIDByPublicIDGlobal :one
SELECT id FROM products WHERE public_id = $1 AND deleted_at IS NULL;

-- ===== Option groups + options =============================================

-- name: CreateOptionGroup :one
INSERT INTO product_option_groups (product_id, code, name, required, min_select, max_select, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListOptionGroups :many
SELECT * FROM product_option_groups WHERE product_id = $1 ORDER BY sort_order, id;

-- name: GetOptionGroup :one
SELECT * FROM product_option_groups WHERE id = $1;

-- name: CreateOption :one
INSERT INTO product_options (group_id, code, name, price_delta, is_default, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- ListOptionsForProduct returns every option of a product's groups, ordered for
-- stable rendering.
-- name: ListOptionsForProduct :many
SELECT o.* FROM product_options o
JOIN product_option_groups g ON g.id = o.group_id
WHERE g.product_id = $1
ORDER BY g.sort_order, o.sort_order, o.id;

-- ===== Rules ===============================================================

-- name: CreateConfigRule :one
INSERT INTO config_rules (product_id, kind, option_id, related_option_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListConfigRules :many
SELECT * FROM config_rules WHERE product_id = $1 ORDER BY id;

-- ===== Quote integration ===================================================

-- name: GetCpqQuote :one
SELECT id, organization_id, currency, status FROM quotes WHERE organization_id = $1 AND id = $2;

-- name: AddConfiguredQuoteItem :one
INSERT INTO quote_items (quote_id, product_id, quantity, unit, unit_price, discount, row_total, configuration)
VALUES ($1, $2, $3, $4, $5, 0, $6, $7)
RETURNING *;

-- name: SumQuoteItems :one
SELECT COALESCE(sum(row_total), 0)::numeric(15,4) AS subtotal FROM quote_items WHERE quote_id = $1;
