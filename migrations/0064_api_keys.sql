-- 0064_api_keys.sql — programmatic API keys (Platform roadmap, Phase 0).
-- Third parties and back-office integrations authenticate with a bearer key
-- ("tgk_…") instead of a user JWT. Only the SHA-256 hash of the key is stored;
-- the raw secret is shown exactly once at creation/rotation. A key carries a set
-- of scopes drawn from the same permission catalog as roles, so the existing
-- RequirePermission middleware gates key-authenticated requests unchanged.

CREATE TABLE api_keys (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text   NOT NULL,
  prefix          text   NOT NULL,              -- shown in the UI to identify a key (e.g. tgk_a1b2c3d4)
  key_hash        text   NOT NULL,              -- sha256(raw key), hex — the raw key is never stored
  scopes          text[] NOT NULL DEFAULT '{}', -- permission strings the key may exercise
  last_used_at    timestamptz,
  expires_at      timestamptz,                  -- optional expiry; null = no expiry
  revoked_at      timestamptz,                  -- soft revoke; a revoked key never authenticates
  created_by      BIGINT,                       -- user id that minted it (null for system)
  created_at      timestamptz NOT NULL DEFAULT now()
);
-- Authentication looks a key up by its hash; it must be globally unique.
CREATE UNIQUE INDEX uq_api_keys_hash ON api_keys (key_hash);
-- Listing a tenant's keys, newest first.
CREATE INDEX idx_api_keys_org ON api_keys (organization_id, created_at DESC);

-- Tenant-isolation net (mirrors 0054/0059): FORCEd, fail-open org_isolation.
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS org_isolation ON api_keys;
CREATE POLICY org_isolation ON api_keys FOR ALL
  USING (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint)
  WITH CHECK (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint);

-- Managing API keys is its own sensitive permission (granted to the demo admin
-- role; the tenant-provisioning template copies org-1 admin perms to new orgs).
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('apikey.view'), ('apikey.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
