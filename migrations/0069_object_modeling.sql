-- 0069_object_modeling.sql — Phase 2 slice 1: generic data modeling. An org can
-- define custom object TYPES (Supplier, Location, Contract …) with a FIELD
-- schema, then store RECORDS of each type — the same JSONB + field-definition +
-- validation approach proven on products, generalized to any entity. Products
-- stay native; custom objects sit alongside them.

CREATE TABLE object_types (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  code            text   NOT NULL,            -- machine key, e.g. "supplier"
  label           text   NOT NULL,
  label_plural    text   NOT NULL DEFAULT '',
  description     text   NOT NULL DEFAULT '',
  is_active       boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, code)
);

CREATE TABLE object_fields (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  object_type_id  BIGINT NOT NULL REFERENCES object_types(id) ON DELETE CASCADE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),  -- denormalized for RLS
  code            text   NOT NULL,            -- key in the record's data JSONB
  label           text   NOT NULL,
  data_type       text   NOT NULL CHECK (data_type IN
                    ('text','number','boolean','select','multiselect','date','file','price')),
  options         JSONB,                       -- allowed values (select/multiselect)
  validation      JSONB  NOT NULL DEFAULT '{}'::jsonb,
  is_required     boolean NOT NULL DEFAULT false,
  sort_order      int    NOT NULL DEFAULT 0,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (object_type_id, code)
);
CREATE INDEX idx_object_fields_type ON object_fields (object_type_id);

CREATE TABLE object_records (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  object_type_id  BIGINT NOT NULL REFERENCES object_types(id) ON DELETE RESTRICT,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  data            JSONB  NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX idx_object_records_type ON object_records (organization_id, object_type_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_object_records_data_gin ON object_records USING GIN (data);

-- Tenant-isolation net (mirrors 0054/0064): FORCEd, fail-open org_isolation on
-- every one of the three tables (each carries organization_id).
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['object_types','object_fields','object_records']
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

-- Permissions: modeling (types/fields) + record CRUD. admin gets view+manage;
-- staff/viewer get read (matching the provisioning template's %.view rule), and
-- org-1 admin seeds the template so new tenants inherit it.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r
  CROSS JOIN (VALUES ('object.view'), ('object.manage')) AS p(perm)
 WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'object.view'
  FROM roles r
 WHERE r.name IN ('staff', 'viewer')
ON CONFLICT DO NOTHING;
