-- Field-sales offline sync (Pack 3 §4): devices hold a scoped local subset and
-- sync via a cursor-based delta protocol. change_log is the append-only outbox
-- whose id IS the sync cursor; sync_push_log gives idempotent client pushes.

CREATE TABLE field_devices (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id          BIGINT NOT NULL REFERENCES users(id),
  device_uuid      UUID NOT NULL,
  platform         text,
  last_sync_cursor BIGINT NOT NULL DEFAULT 0,
  last_seen_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, device_uuid)
);

CREATE TABLE change_log (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  scope_rep_id    BIGINT,                -- visible to this rep; NULL = global (e.g. catalog)
  entity_type     text NOT NULL,
  entity_id       BIGINT NOT NULL,
  op              text NOT NULL CHECK (op IN ('upsert','delete')),
  payload         JSONB NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_change_log_scope ON change_log(scope_rep_id, id);
CREATE INDEX idx_change_log_global ON change_log(id) WHERE scope_rep_id IS NULL;

CREATE TABLE sync_push_log (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  device_id        BIGINT NOT NULL REFERENCES field_devices(id),
  client_change_id UUID NOT NULL,
  entity_type      text NOT NULL,
  op               text NOT NULL,
  status           text NOT NULL CHECK (status IN ('applied','conflict','rejected')),
  server_entity_id BIGINT,
  detail           JSONB,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (device_id, client_change_id)
);

-- Activities gain updated_at so the sync conflict policy (last-write-wins by
-- updated_at) can detect a stale offline edit.
ALTER TABLE activities ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER trg_activities_updated BEFORE UPDATE ON activities
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Field-sync permission for the demo admin role (sales reps).
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'field.sync'
  FROM roles r
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
