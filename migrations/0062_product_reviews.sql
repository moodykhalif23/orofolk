-- 0062_product_reviews.sql — storefront product reviews (buyer ratings).
-- Verified-purchase only (the write handler checks the buyer's company has a
-- delivered order containing the product) and moderated: a review enters
-- 'pending' and stays hidden until a staff member approves it. The aggregate
-- rating shown on the storefront counts only 'approved' reviews.

CREATE TABLE product_reviews (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id  BIGINT NOT NULL REFERENCES organizations(id),
  product_id       BIGINT NOT NULL REFERENCES products(id),
  customer_id      BIGINT NOT NULL REFERENCES customers(id),       -- buying company
  customer_user_id BIGINT NOT NULL REFERENCES customer_users(id),  -- author
  rating           int  NOT NULL CHECK (rating BETWEEN 1 AND 5),
  title            text NOT NULL DEFAULT '',
  body             text NOT NULL DEFAULT '',
  status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'approved', 'rejected')),
  verified         boolean NOT NULL DEFAULT true,
  created_at       timestamptz NOT NULL DEFAULT now(),
  reviewed_at      timestamptz,
  reviewed_by      BIGINT,                                         -- staff user who moderated
  UNIQUE (product_id, customer_user_id)                            -- one review per author per product
);
-- Approved-reviews-for-a-product listing + aggregate.
CREATE INDEX idx_product_reviews_product ON product_reviews (organization_id, product_id, status);
-- Moderation queue (by status, recent first).
CREATE INDEX idx_product_reviews_moderation ON product_reviews (organization_id, status, created_at DESC);

-- Tenant-isolation net (mirrors 0054/0059): FORCEd, fail-open org_isolation.
ALTER TABLE product_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_reviews FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS org_isolation ON product_reviews;
CREATE POLICY org_isolation ON product_reviews FOR ALL
  USING (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint)
  WITH CHECK (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint);

-- Moderating reviews is its own permission (granted to the demo admin role;
-- assign to the relevant roles per tenant).
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'review.moderate'
  FROM roles r
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
