-- 0042_seed_demo_vendor.sql — development seed so the vendor portal works on
-- first boot, mirroring the demo admin (0003) and demo buyer (0040). Creates one
-- marketplace vendor with a portal login, and hands it one of the seeded demo
-- products so the storefront shows "Sold by" and the vendor sees a live catalog.
-- Idempotent: only inserts when vendor@demo.test is absent. The password hash is
-- bcrypt('vendor1234'). Change/remove for production.

WITH new_vendor AS (
  INSERT INTO vendors (organization_id, name, slug, contact_email, status, commission_rate, payout_terms_days)
  SELECT 1, 'Demo Vendor Co', 'demo-vendor-co', 'sales@demo-vendor.test', 'active', 10, 30
  WHERE NOT EXISTS (SELECT 1 FROM vendor_users WHERE email = 'vendor@demo.test')
  RETURNING id
)
INSERT INTO vendor_users (vendor_id, email, password_hash, full_name, role)
SELECT id, 'vendor@demo.test',
       '$2a$10$WiaHQjLY8ZKiN6iL6UGzae9jGuGa0FLMSsT7NLWR.3BM598e4r/i6',  -- bcrypt('vendor1234')
       'Demo Vendor', 'admin'
  FROM new_vendor;

-- Hand the seeded PIPE-200 product to the demo vendor (approved so it stays
-- visible on the storefront). No-op if the vendor wasn't just created.
UPDATE products p
   SET vendor_id = v.id, approval_status = 'approved'
  FROM vendors v
 WHERE v.organization_id = 1 AND v.slug = 'demo-vendor-co'
   AND p.organization_id = 1 AND p.sku = 'PIPE-200'
   AND p.vendor_id IS NULL;
