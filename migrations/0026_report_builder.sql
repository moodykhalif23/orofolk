-- Custom report builder (Pack 3 §1, V2): saved report definitions compiled to
-- safe SQL by internal/report, on-demand + scheduled runs, and run artifacts.
-- Dashboards/widgets (also in §1.3) are deferred; the Phase-2 operational
-- dashboards already cover the manager view.

CREATE TABLE report_definitions (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text NOT NULL,
  entity          text NOT NULL,
  dimensions      JSONB NOT NULL DEFAULT '[]'::jsonb,
  measures        JSONB NOT NULL DEFAULT '[]'::jsonb,
  filters         JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by      BIGINT REFERENCES users(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_report_definitions_org ON report_definitions(organization_id);
CREATE TRIGGER trg_report_definitions_updated BEFORE UPDATE ON report_definitions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE report_schedules (
  id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  report_definition_id  BIGINT NOT NULL REFERENCES report_definitions(id) ON DELETE CASCADE,
  cadence               text NOT NULL CHECK (cadence IN ('daily','weekly','monthly')),
  format                text NOT NULL DEFAULT 'csv' CHECK (format IN ('csv','xlsx')),
  recipients            JSONB NOT NULL DEFAULT '[]'::jsonb,
  is_active             boolean NOT NULL DEFAULT true,
  last_run_at           timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_report_schedules_def ON report_schedules(report_definition_id);

CREATE TABLE report_runs (
  id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  report_definition_id  BIGINT NOT NULL REFERENCES report_definitions(id) ON DELETE CASCADE,
  status                text NOT NULL DEFAULT 'running'
                          CHECK (status IN ('running','ok','error')),
  trigger               text NOT NULL DEFAULT 'manual' CHECK (trigger IN ('manual','schedule')),
  row_count             int,
  file_name             text,
  content_type          text,
  file_bytes            bytea,        -- artifact stored in-DB (object storage is a V2 swap)
  file_url              text,         -- authenticated download path
  error                 text,
  started_at            timestamptz NOT NULL DEFAULT now(),
  finished_at           timestamptz
);
CREATE INDEX idx_report_runs_def ON report_runs(report_definition_id, started_at DESC);

-- Report-builder management permission for the demo admin role (report.view
-- already exists from 0022 for running/reading).
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'report.manage'
  FROM roles r
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
