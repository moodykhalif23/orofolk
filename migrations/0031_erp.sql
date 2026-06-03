-- ERP / accounting sync (Pack 2 §4.6). A pattern, not a single adapter: idempotent
-- OUTBOUND (commerce → ERP) on confirmed orders + issued invoices, keyed by our
-- public_id; INBOUND (ERP → commerce) master-data via a signed webhook. Every
-- call is logged; external_refs maps our entities to ERP ids.

CREATE TABLE integration_connections (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  provider        text NOT NULL,                 -- 'generic_webhook','netsuite',...
  kind            text NOT NULL DEFAULT 'erp' CHECK (kind IN ('erp','accounting')),
  endpoint        text,                          -- outbound POST target
  secret          text,                          -- HMAC signing secret (in + out)
  config          JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active       boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_integration_connections_org ON integration_connections(organization_id);

CREATE TABLE external_refs (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  connection_id BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  entity_type   text NOT NULL,                   -- 'order','customer','product','invoice'
  entity_id     BIGINT NOT NULL,
  external_id   text NOT NULL,
  synced_at     timestamptz,
  UNIQUE (connection_id, entity_type, entity_id),
  UNIQUE (connection_id, entity_type, external_id)
);

CREATE TABLE sync_logs (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  connection_id   BIGINT NOT NULL REFERENCES integration_connections(id) ON DELETE CASCADE,
  direction       text NOT NULL CHECK (direction IN ('outbound','inbound')),
  entity_type     text NOT NULL,
  entity_id       BIGINT,
  operation       text NOT NULL,                 -- 'upsert','delete'
  status          text NOT NULL CHECK (status IN ('sent','error','processed','skipped')),
  idempotency_key text,
  external_id     text,
  error           text,
  detail          JSONB,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sync_logs_conn ON sync_logs(connection_id, created_at DESC);
-- Inbound dedupe: an ERP event id is processed at most once per connection.
CREATE UNIQUE INDEX uq_sync_logs_inbound_event
  ON sync_logs(connection_id, idempotency_key)
  WHERE direction = 'inbound' AND idempotency_key IS NOT NULL;

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('erp.view'), ('erp.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
