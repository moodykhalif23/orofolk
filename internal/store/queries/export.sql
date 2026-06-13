-- Raw-record exports for the data-export center. Distinct from the report
-- builder (which aggregates): these dump full rows of the core entities, joined
-- to human-readable names, org-scoped and capped, for the customer's own
-- finance/BI/budgeting systems. Ordered newest-first; the LIMIT is the export
-- cap applied by the handler.

-- name: ExportOrders :many
SELECT o.public_id, c.name AS customer, o.status, o.currency,
       o.po_number, o.subtotal, o.tax_total, o.shipping_total, o.grand_total,
       o.created_at
FROM orders o
JOIN customers c ON c.id = o.customer_id
WHERE o.organization_id = $1
ORDER BY o.created_at DESC, o.id DESC
LIMIT $2;

-- name: ExportOrderItems :many
SELECT o.public_id AS order_public_id, c.name AS customer,
       oi.sku, oi.name, oi.quantity, oi.unit, oi.unit_price, oi.tax_amount, oi.row_total,
       o.status AS order_status, o.created_at
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
JOIN customers c ON c.id = o.customer_id
WHERE o.organization_id = $1
ORDER BY o.created_at DESC, oi.id
LIMIT $2;

-- name: ExportCustomers :many
SELECT c.public_id, c.name, c.tax_id, c.payment_terms_days, c.credit_limit,
       COALESCE(g.name, '') AS customer_group, c.is_active, c.created_at
FROM customers c
LEFT JOIN customer_groups g ON g.id = c.customer_group_id
WHERE c.organization_id = $1 AND c.deleted_at IS NULL
ORDER BY c.created_at DESC, c.id DESC
LIMIT $2;

-- Invoices carry no organization_id of their own — they are org-scoped through
-- their customer (the same indirect scoping the AR-aging query relies on).
-- name: ExportInvoices :many
SELECT i.public_id, c.name AS customer, i.status, i.currency,
       i.subtotal, i.tax_total, i.grand_total, i.issued_at, i.due_at, i.created_at
FROM invoices i
JOIN customers c ON c.id = i.customer_id
WHERE c.organization_id = $1
ORDER BY i.created_at DESC, i.id DESC
LIMIT $2;
