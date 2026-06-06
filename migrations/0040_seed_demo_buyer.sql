-- 0040_seed_demo_buyer.sql — development seed so the storefront is usable on
-- first boot, mirroring the admin login seeded in 0003. Creates one buying
-- company with a single buyer login and a default address. Idempotent: it only
-- inserts when buyer@demo.test is absent, so it is safe on existing installs.
-- The password hash below is bcrypt('buyer1234'). Change/remove for production.

-- Company + buyer login (role 'admin' so the demo buyer can exercise the full
-- self-service surface: company users, addresses, approvals, budgets).
WITH new_customer AS (
  INSERT INTO customers (organization_id, name, payment_terms_days, credit_limit, is_active)
  SELECT 1, 'Demo Buyer Co', 30, 100000, true
  WHERE NOT EXISTS (SELECT 1 FROM customer_users WHERE email = 'buyer@demo.test')
  RETURNING id
)
INSERT INTO customer_users (customer_id, email, password_hash, full_name, role)
SELECT id, 'buyer@demo.test',
       '$2a$10$47a22z83Al5E9bPzlsl0BOjj482kmLmJt94LMqhpGqdIBbhewbp7K',  -- bcrypt('buyer1234')
       'Demo Buyer', 'admin'
  FROM new_customer;

-- A default shipping + billing address so checkout works without typing one.
INSERT INTO customer_addresses (customer_id, type, is_default, line1, city, country)
SELECT cu.customer_id, t.type, true, '1 Demo Street', 'Nairobi', 'KE'
  FROM customer_users cu
  CROSS JOIN (VALUES ('shipping'), ('billing')) AS t(type)
 WHERE cu.email = 'buyer@demo.test'
   AND NOT EXISTS (
     SELECT 1 FROM customer_addresses a
      WHERE a.customer_id = cu.customer_id AND a.type = t.type
   );
