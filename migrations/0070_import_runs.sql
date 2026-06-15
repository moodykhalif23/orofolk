-- 0070_import_runs.sql — Phase 3 slice 1: the generic import engine. An upload is
-- parsed, column-mapped and validated into an import_run + per-row import_rows
-- WITHOUT touching the target — a dry run. Committing then applies the
-- create/update rows. One pipeline serves products and any custom object type,
-- every row validated by the same engine that guards live writes.

CREATE TABLE import_runs (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  target          text   NOT NULL,            -- 'products' | 'object:<code>'
  format          text   NOT NULL DEFAULT 'csv',
  source_filename text   NOT NULL DEFAULT '',
  status          text   NOT NULL DEFAULT 'validated'
                    CHECK (status IN ('validated', 'committed', 'failed')),
  total_rows      int    NOT NULL DEFAULT 0,
  create_rows     int    NOT NULL DEFAULT 0,
  update_rows     int    NOT NULL DEFAULT 0,
  error_rows      int    NOT NULL DEFAULT 0,
  created_by      BIGINT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  committed_at    timestamptz
);
CREATE INDEX idx_import_runs_org ON import_runs (organization_id, created_at DESC);

CREATE TABLE import_rows (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  import_run_id   BIGINT NOT NULL REFERENCES import_runs(id) ON DELETE CASCADE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),  -- denormalized for RLS
  row_number      int    NOT NULL,
  data            JSONB  NOT NULL DEFAULT '{}'::jsonb,   -- mapped + normalized row, ready to apply
  status          text   NOT NULL,            -- 'create' | 'update' | 'error'
  message         text   NOT NULL DEFAULT ''
);
CREATE INDEX idx_import_rows_run ON import_rows (import_run_id, row_number);

-- Tenant-isolation net (mirrors 0054/0069): FORCEd, fail-open org_isolation.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['import_runs', 'import_rows']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('DROP POLICY IF EXISTS org_isolation ON %I', t);
    EXECUTE format(
      'CREATE POLICY org_isolation ON %I FOR ALL '
      || 'USING (COALESCE(current_setting(''app.org_id'', true), '''') = '''' '
      || '   OR organization_id = current_setting(''app.org_id'', true)::bigint) '
      || 'WITH CHECK (COALESCE(current_setting(''app.org_id'', true), '''') = '''' '
      || '   OR organization_id = current_setting(''app.org_id'', true)::bigint)',
      t);
  END LOOP;
END $$;

-- Permissions: import.manage (upload + commit) for admin; import.view (review
-- runs + templates) for admin/staff/viewer. org-1 admin seeds the template.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r
  CROSS JOIN (VALUES ('import.view'), ('import.manage')) AS p(perm)
 WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'import.view'
  FROM roles r
 WHERE r.name IN ('staff', 'viewer')
ON CONFLICT DO NOTHING;
