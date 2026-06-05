-- Hierarchical configuration cascade (PRD §4.3): a single key can be set at the
-- org, website, customer-group or customer scope; resolution returns the most
-- specific value (customer > group > website > org). Generic JSON values so any
-- setting (flags, limits, defaults) can use the same cascade.
CREATE TABLE config_settings (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  scope           text NOT NULL CHECK (scope IN ('org','website','group','customer')),
  scope_id        BIGINT,                       -- null for 'org'; else website/group/customer id
  key             text NOT NULL,
  value           jsonb NOT NULL,
  updated_at      timestamptz NOT NULL DEFAULT now(),
  -- 'org' scope is org-wide (no scope_id); the others require one.
  CHECK ((scope = 'org') = (scope_id IS NULL)),
  UNIQUE (organization_id, scope, scope_id, key)
);
CREATE INDEX idx_config_settings_lookup ON config_settings(organization_id, key);

-- Grant the config permissions to the demo admin role.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r CROSS JOIN (VALUES ('settings.view'), ('settings.manage')) AS p(perm)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
