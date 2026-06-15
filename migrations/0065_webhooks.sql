-- 0065_webhooks.sql — outbound webhooks / event subscriptions (Platform
-- roadmap, Phase 0). A tenant registers endpoints that receive a signed POST
-- whenever a subscribed domain event fires. Delivery rides the existing
-- EmitEvent → dispatch_event path: after in-app notifications and automation
-- rules, the worker fans the event out to matching endpoints as river jobs, so
-- retries/backoff and a per-attempt delivery log come for free. This is the
-- primitive that unlocks Zapier / n8n / Make (Phase 4 distribution).

CREATE TABLE webhook_endpoints (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  url             text    NOT NULL,
  secret          text    NOT NULL,              -- HMAC-SHA256 signing secret (X-Teggo-Signature)
  description     text    NOT NULL DEFAULT '',
  event_types     text[]  NOT NULL DEFAULT '{}', -- subscribed events; empty = all events
  is_active       boolean NOT NULL DEFAULT true,
  created_by      BIGINT,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_endpoints_org ON webhook_endpoints (organization_id, id);

-- One row per delivery ATTEMPT (river may retry), so the log is a full history.
-- The payload is stored so a failed delivery can be replayed verbatim.
CREATE TABLE webhook_deliveries (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  endpoint_id     BIGINT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
  event_type      text   NOT NULL,
  payload         JSONB  NOT NULL DEFAULT '{}'::jsonb,
  status          text   NOT NULL,              -- 'success' | 'failed'
  attempt         int    NOT NULL DEFAULT 1,
  response_status int    NOT NULL DEFAULT 0,
  error           text   NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_deliveries_endpoint ON webhook_deliveries (endpoint_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_org ON webhook_deliveries (organization_id, created_at DESC);

-- Tenant-isolation net (mirrors 0054/0059) on both tables.
ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS org_isolation ON webhook_endpoints;
CREATE POLICY org_isolation ON webhook_endpoints FOR ALL
  USING (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint)
  WITH CHECK (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint);

ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS org_isolation ON webhook_deliveries;
CREATE POLICY org_isolation ON webhook_deliveries FOR ALL
  USING (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint)
  WITH CHECK (COALESCE(current_setting('app.org_id', true), '') = ''
     OR organization_id = current_setting('app.org_id', true)::bigint);

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('webhook.view'), ('webhook.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
