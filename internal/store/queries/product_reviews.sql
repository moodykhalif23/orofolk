-- Product reviews — verified-purchase, moderated buyer ratings (0062).

-- CustomerHasDeliveredProduct gates who may review: true only when the buyer's
-- company has a delivered/closed order containing the product.
-- name: CustomerHasDeliveredProduct :one
SELECT EXISTS (
  SELECT 1 FROM orders o
  JOIN order_items oi ON oi.order_id = o.id
  WHERE o.organization_id = $1 AND o.customer_id = $2 AND oi.product_id = $3
    AND o.status IN ('delivered', 'closed')
) AS purchased;

-- CreateReview writes (or re-submits) a buyer's review. A re-submission resets
-- it to 'pending' so edits are re-moderated. One row per (product, author).
-- name: CreateReview :one
INSERT INTO product_reviews (organization_id, product_id, customer_id, customer_user_id, rating, title, body, verified)
VALUES ($1, $2, $3, $4, $5, $6, $7, true)
ON CONFLICT (product_id, customer_user_id) DO UPDATE
  SET rating = EXCLUDED.rating, title = EXCLUDED.title, body = EXCLUDED.body,
      status = 'pending', created_at = now(), reviewed_at = NULL, reviewed_by = NULL
RETURNING id, status;

-- ListApprovedReviews powers the storefront PDP reviews list.
-- name: ListApprovedReviews :many
SELECT pr.id, pr.rating, pr.title, pr.body, pr.verified, pr.created_at, cu.full_name AS author
FROM product_reviews pr
JOIN customer_users cu ON cu.id = pr.customer_user_id
WHERE pr.organization_id = $1 AND pr.product_id = $2 AND pr.status = 'approved'
ORDER BY pr.created_at DESC
LIMIT $3 OFFSET $4;

-- GetReviewAggregate returns the approved-only average + count for a product
-- (storefront PDP/PLP stars).
-- name: GetReviewAggregate :one
SELECT COALESCE(ROUND(AVG(rating), 2), 0)::numeric AS average, count(*)::bigint AS total
FROM product_reviews
WHERE organization_id = $1 AND product_id = $2 AND status = 'approved';

-- ListReviewsByStatus is the admin moderation queue (status = pending, etc.).
-- name: ListReviewsByStatus :many
SELECT pr.id, pr.rating, pr.title, pr.body, pr.verified, pr.created_at, pr.status,
       p.name AS product_name, p.slug AS product_slug, cu.full_name AS author
FROM product_reviews pr
JOIN products p ON p.id = pr.product_id
JOIN customer_users cu ON cu.id = pr.customer_user_id
WHERE pr.organization_id = $1 AND pr.status = $2
ORDER BY pr.created_at DESC
LIMIT $3 OFFSET $4;

-- ModerateReview approves/rejects a pending review.
-- name: ModerateReview :one
UPDATE product_reviews
SET status = $3, reviewed_at = now(), reviewed_by = $4
WHERE id = $1 AND organization_id = $2
RETURNING id, status;
