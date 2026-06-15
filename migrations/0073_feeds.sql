-- 0073_feeds.sql — Phase 4 slice 1: syndication feeds. A feed projects a data
-- source (products or any custom object type) through a field MAPPING into a
-- channel format (CSV / JSON / XML) — the outbound twin of the import engine,
-- which brings data IN through a mapping. The definition lives here; generation
-- is on demand (scheduled regeneration + delivery come in a later slice).

CREATE TABLE feeds (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id       UUID   NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text   NOT NULL,
  source          text   NOT NULL,                       -- 'products' | 'object:<code>'
  channel         text   NOT NULL DEFAULT 'custom',       -- destination preset (Phase 4 slice 2)
  format          text   NOT NULL DEFAULT 'csv'
                    CHECK (format IN ('csv', 'json', 'xml')),
  mapping         JSONB  NOT NULL DEFAULT '[]'::jsonb,     -- [{out, src, const}] — ordered projection
  is_active       boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_feeds_org ON feeds (organization_id, created_at DESC);

-- Tenant-isolation net (mirrors 0064/0069/0070): FORCEd, fail-open org_isolation.
ALTER TABLE feeds ENABLE ROW LEVEL SECURITY;
ALTER TABLE feeds FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS org_isolation ON feeds;
CREATE POLICY org_isolation ON feeds FOR ALL
  USING (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint)
  WITH CHECK (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint);

-- Permissions: feed.manage (author + generate) for admin; feed.view (list +
-- preview + output) for admin/staff/viewer. Granted across every org's roles
-- (seed + backfill in one, like 0069/0070); org-1 admin seeds the template.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r
  CROSS JOIN (VALUES ('feed.view'), ('feed.manage')) AS p(perm)
 WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'feed.view'
  FROM roles r
 WHERE r.name IN ('staff', 'viewer')
ON CONFLICT DO NOTHING;
