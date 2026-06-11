-- Subscriptions / recurring & standing orders (Roadmap Tier 2 #4). A subscription
-- holds a set of line items and a cadence; a daily job materializes due ones into
-- real orders (priced at run time from the customer's combined prices), records a
-- run, and advances the next run date. Buyers (or reps) can pause, skip, or cancel.
CREATE TABLE subscriptions (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id        UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id  BIGINT NOT NULL REFERENCES organizations(id),
  website_id       BIGINT NOT NULL REFERENCES websites(id),
  customer_id      BIGINT NOT NULL REFERENCES customers(id),
  customer_user_id BIGINT REFERENCES customer_users(id),
  name             text,
  currency         CHAR(3) NOT NULL,
  cadence          text NOT NULL CHECK (cadence IN ('weekly','biweekly','monthly','quarterly')),
  next_run_date    date NOT NULL,
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','cancelled')),
  po_number        text,
  created_by       text,
  last_run_at      timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_org ON subscriptions(organization_id, status);
CREATE INDEX idx_subscriptions_due ON subscriptions(next_run_date) WHERE status = 'active';

CREATE TABLE subscription_items (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  product_id      BIGINT NOT NULL REFERENCES products(id),
  quantity        NUMERIC(15,4) NOT NULL,
  unit            text NOT NULL DEFAULT 'each'
);
CREATE INDEX idx_subscription_items_sub ON subscription_items(subscription_id);

CREATE TABLE subscription_runs (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  order_id        BIGINT REFERENCES orders(id) ON DELETE SET NULL,
  run_date        date NOT NULL,
  status          text NOT NULL CHECK (status IN ('success','skipped','failed')),
  note            text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscription_runs_sub ON subscription_runs(subscription_id, created_at DESC);

CREATE TRIGGER trg_subscriptions_updated BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Subscription permissions for the demo admin role.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('subscription.view'), ('subscription.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
