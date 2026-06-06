-- 0041_marketplace.sql — Multi-vendor marketplace foundation.
--
-- Turns the single-seller platform into a marketplace: vendor records, vendor
-- portal logins (a THIRD token audience, 'vendor', alongside 'admin' and
-- 'storefront'), product ownership by vendor with operator moderation, per-vendor
-- order splitting, a commission ledger, and payout batches.
--
-- Fully additive. A product with vendor_id NULL stays operator-owned ("house"
-- product) and never spawns a vendor sub-order, so every existing single-seller
-- order placed before/after this migration behaves exactly as before.

-- A selling vendor on the marketplace. commission_rate is the operator's take as
-- a percent (10.0000 = 10%); net payable to the vendor is gross * (1 - rate/100).
CREATE TABLE vendors (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id         UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id   BIGINT NOT NULL REFERENCES organizations(id),
  name              text NOT NULL,
  slug              text NOT NULL,
  contact_email     text,
  status            text NOT NULL DEFAULT 'active'
                      CHECK (status IN ('pending','active','suspended')),
  commission_rate   NUMERIC(7,4) NOT NULL DEFAULT 0,    -- percent, e.g. 10.0000 = 10%
  payout_terms_days int NOT NULL DEFAULT 30,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  UNIQUE (organization_id, slug)
);
CREATE INDEX idx_vendors_org ON vendors(organization_id);
CREATE TRIGGER trg_vendors_updated BEFORE UPDATE ON vendors
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A login for the vendor self-service portal (audience 'vendor'). Subject in the
-- JWT is the vendor_user id; the token also carries vendor_id + org.
CREATE TABLE vendor_users (
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  vendor_id      BIGINT NOT NULL REFERENCES vendors(id) ON DELETE CASCADE,
  email          citext NOT NULL,
  password_hash  text NOT NULL,
  full_name      text NOT NULL,
  role           text NOT NULL DEFAULT 'member'
                   CHECK (role IN ('member','admin')),
  is_active      boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (vendor_id, email)
);
CREATE INDEX idx_vendor_users_vendor ON vendor_users(vendor_id);
CREATE TRIGGER trg_vendor_users_updated BEFORE UPDATE ON vendor_users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Product ownership. NULL = operator-owned (house product). approval_status lets
-- the operator moderate vendor-submitted listings; operator products default to
-- 'approved' so existing rows and admin-created products are live immediately.
ALTER TABLE products ADD COLUMN vendor_id BIGINT REFERENCES vendors(id);
ALTER TABLE products ADD COLUMN approval_status text NOT NULL DEFAULT 'approved'
  CHECK (approval_status IN ('draft','pending','approved','rejected'));
CREATE INDEX idx_products_vendor ON products(vendor_id);

-- Per-vendor sub-order: one buyer order fans out into one vendor_order per vendor
-- whose products appear on it. Carries its own fulfillment status + commission
-- snapshot. gross/commission/net are computed once at split time and frozen.
CREATE TABLE vendor_orders (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id         UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id   BIGINT NOT NULL REFERENCES organizations(id),
  order_id          BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  vendor_id         BIGINT NOT NULL REFERENCES vendors(id),
  status            text NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','accepted','shipped','delivered','cancelled')),
  currency          CHAR(3) NOT NULL,
  gross_total       NUMERIC(15,4) NOT NULL DEFAULT 0,  -- sum of line row_totals (ex-tax)
  commission_rate   NUMERIC(7,4)  NOT NULL DEFAULT 0,  -- snapshot of vendor rate at split
  commission_total  NUMERIC(15,4) NOT NULL DEFAULT 0,  -- operator's take
  net_total         NUMERIC(15,4) NOT NULL DEFAULT 0,  -- payable to vendor (gross - commission)
  payout_id         BIGINT,                            -- set when settled into a payout (FK below)
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (order_id, vendor_id)
);
CREATE INDEX idx_vendor_orders_vendor ON vendor_orders(vendor_id);
CREATE INDEX idx_vendor_orders_order ON vendor_orders(order_id);
CREATE INDEX idx_vendor_orders_payout ON vendor_orders(payout_id);
CREATE TRIGGER trg_vendor_orders_updated BEFORE UPDATE ON vendor_orders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Tag each order line with its owning vendor at split time so a vendor sees
-- exactly their lines. NULL = operator line (not part of any vendor_order).
ALTER TABLE order_items ADD COLUMN vendor_id BIGINT REFERENCES vendors(id);
CREATE INDEX idx_order_items_vendor ON order_items(vendor_id);

-- Payout batch: groups settled vendor_orders for a single disbursement to a
-- vendor. amount is the sum of the included vendor_orders' net_total.
CREATE TABLE vendor_payouts (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  vendor_id       BIGINT NOT NULL REFERENCES vendors(id),
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','paid','cancelled')),
  currency        CHAR(3) NOT NULL,
  amount          NUMERIC(15,4) NOT NULL DEFAULT 0,
  reference       text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  paid_at         timestamptz
);
CREATE INDEX idx_vendor_payouts_vendor ON vendor_payouts(vendor_id);

ALTER TABLE vendor_orders ADD CONSTRAINT fk_vendor_orders_payout
  FOREIGN KEY (payout_id) REFERENCES vendor_payouts(id);

-- Grant marketplace-management permissions to the demo admin role so the seeded
-- operator can manage vendors, commissions and payouts. Safe on existing installs.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('vendor.view'), ('vendor.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
