-- Tax/VAT adapter (Pack 2 §4.4): rate config + product tax-class lookup.

-- name: UpsertTaxRate :one
INSERT INTO tax_rates (organization_id, country, tax_class, rate, name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id, country, tax_class)
DO UPDATE SET rate = EXCLUDED.rate, name = EXCLUDED.name
RETURNING *;

-- name: ListTaxRates :many
SELECT * FROM tax_rates WHERE organization_id = $1 ORDER BY country, tax_class;

-- ListTaxRatesByCountry feeds the local VAT provider for a destination.
-- name: ListTaxRatesByCountry :many
SELECT tax_class, rate FROM tax_rates WHERE organization_id = $1 AND country = $2;

-- name: DeleteTaxRate :exec
DELETE FROM tax_rates WHERE organization_id = $1 AND id = $2;

-- GetProductTaxClasses returns the tax class for a set of products (order tax).
-- name: GetProductTaxClasses :many
SELECT id, tax_class FROM products WHERE organization_id = $1 AND id = ANY($2::bigint[]);
