-- 0061_demo_orgs.sql — self-provisioned demo tenants. A prospect can request an
-- instant demo from the marketing landing page; the backend provisions a fresh,
-- isolated org (reusing tenant.Provision), activates it immediately (no email
-- verification), seeds representative data, and logs them straight in. Demos are
-- time-limited: a daily job suspends them once they expire (the org-status gate
-- then shuts them off). is_demo also lets operators tell demos apart from real
-- tenants in the platform overview.
ALTER TABLE organizations
  ADD COLUMN is_demo         boolean NOT NULL DEFAULT false,
  ADD COLUMN demo_expires_at timestamptz;

-- Drives the expiry sweep efficiently (only demo rows are indexed).
CREATE INDEX idx_organizations_demo_expiry ON organizations (demo_expires_at) WHERE is_demo;
