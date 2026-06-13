-- 0056_live_reporting.sql — make dashboards real-time (SAAS / Toyoka sim §5).
-- The reporting materialized views (mv_daily_sales, mv_top_products) were
-- refreshed hourly by a background job, so dashboards lagged reality by up to an
-- hour and the hourly REFRESH ... CONCURRENTLY was an unbounded full recompute.
-- Reporting now aggregates live over orders/order_items, bounded by the date
-- window + org and riding the composite index below — same philosophy as the
-- read-time pricing resolution (migration 0055). A freshly placed order shows in
-- the dashboard immediately; no refresh job, no staleness.

CREATE INDEX IF NOT EXISTS idx_orders_org_created ON orders (organization_id, created_at);

DROP MATERIALIZED VIEW IF EXISTS mv_daily_sales;
DROP MATERIALIZED VIEW IF EXISTS mv_top_products;
