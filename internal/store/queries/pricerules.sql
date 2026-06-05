-- Price adjustment rules (migration 0035).

-- name: ListActivePriceAdjustmentRules :many
SELECT * FROM price_adjustment_rules
WHERE organization_id = $1 AND is_active = true
ORDER BY priority DESC, id;

-- name: ListPriceAdjustmentRules :many
SELECT * FROM price_adjustment_rules
WHERE organization_id = $1
ORDER BY priority DESC, id;

-- name: CreatePriceAdjustmentRule :one
INSERT INTO price_adjustment_rules (
  organization_id, name, customer_group_id, attribute_key, attribute_value,
  adjustment_type, adjustment_value, priority, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: DeletePriceAdjustmentRule :execrows
DELETE FROM price_adjustment_rules WHERE id = $1 AND organization_id = $2;
