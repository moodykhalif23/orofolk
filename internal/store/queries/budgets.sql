-- Procurement budgets (migration 0039).

-- name: CreateBudget :one
INSERT INTO customer_budgets (customer_id, cost_center, period, amount, currency)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (customer_id, cost_center)
DO UPDATE SET period = EXCLUDED.period, amount = EXCLUDED.amount, currency = EXCLUDED.currency, is_active = true
RETURNING *;

-- name: ListBudgetsForCustomer :many
SELECT * FROM customer_budgets WHERE customer_id = $1 ORDER BY cost_center;

-- name: DeleteBudget :execrows
DELETE FROM customer_budgets WHERE id = $1 AND customer_id = $2;

-- GetActiveBudget finds the active budget governing a (customer, cost_center).
-- name: GetActiveBudget :one
SELECT * FROM customer_budgets
WHERE customer_id = $1 AND cost_center = $2 AND is_active = true;

-- SpendForCustomerPeriod totals non-cancelled order value for a customer and
-- cost center since the period start. $3 = period start timestamp.
-- name: SpendForCustomerPeriod :one
SELECT COALESCE(SUM(grand_total), 0)::numeric(15,4)
FROM orders
WHERE customer_id = $1 AND COALESCE(cost_center, '') = $2
  AND status <> 'cancelled' AND created_at >= $3;

-- name: SetOrderCostCenter :exec
UPDATE orders SET cost_center = $2 WHERE id = $1;
