-- Reporting queries — live aggregates over orders/order_items, org-scoped. No
-- materialized views: dashboards are real-time. Every query is bounded by a date
-- window and rides idx_orders_org_created, so a just-placed order is reflected
-- immediately (no hourly refresh, no staleness). Mirrors the read-time pricing
-- model (migration 0055/0056).

-- DailySales returns the daily revenue/order series within a date range
-- (summed across currencies for the dashboard line chart — a known V1
-- simplification when an org trades in several currencies).
-- name: DailySales :many
SELECT date_trunc('day', o.created_at)::date AS day,
       count(*)::bigint AS order_count,
       COALESCE(sum(o.grand_total), 0)::numeric(15,4) AS revenue
FROM orders o
WHERE o.organization_id = sqlc.arg(organization_id)
  AND o.status <> 'cancelled'
  AND o.created_at >= sqlc.arg(from_date)::date
  AND o.created_at < (sqlc.arg(to_date)::date + 1)
GROUP BY 1
ORDER BY 1;

-- SalesSummary is the headline KPI rollup since a date.
-- name: SalesSummary :one
SELECT count(*)::bigint AS order_count,
       COALESCE(sum(o.grand_total), 0)::numeric(15,4) AS revenue
FROM orders o
WHERE o.organization_id = sqlc.arg(organization_id)
  AND o.status <> 'cancelled'
  AND o.created_at >= sqlc.arg(since)::date;

-- TopProducts ranks products by revenue in a calendar month, joined to product
-- names. The month is the first day of the target month.
-- name: TopProducts :many
SELECT oi.product_id, p.sku, p.name,
       sum(oi.quantity)::numeric(15,4) AS qty,
       sum(oi.row_total)::numeric(15,4) AS revenue
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN products p ON p.id = oi.product_id
WHERE o.organization_id = sqlc.arg(organization_id)
  AND o.status <> 'cancelled'
  AND o.created_at >= sqlc.arg(month)::date
  AND o.created_at < (sqlc.arg(month)::date + interval '1 month')
GROUP BY oi.product_id, p.sku, p.name
ORDER BY revenue DESC, p.name
LIMIT sqlc.arg(lim);
