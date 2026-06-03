-- Configurable workflow & automation engine — Pack 2 §3. A state-machine engine
-- (entity lifecycles) plus automation rules (event → conditions → actions), both
-- config-driven and extensible via Go-registered guards/actions. This replaces
-- the hardcoded order/shipment state machines: allowed transitions now live in
-- the database (workflow_definitions/states/transitions), not in Go maps.

CREATE TABLE workflow_definitions (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  code            text NOT NULL,            -- 'order_default','quote_default'
  entity_type     text NOT NULL,            -- 'order','quote','rfq'
  name            text NOT NULL,
  is_active       boolean NOT NULL DEFAULT true,
  UNIQUE (organization_id, code)
);

CREATE TABLE workflow_states (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  definition_id BIGINT NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
  code          text NOT NULL,
  label         text NOT NULL,
  is_initial    boolean NOT NULL DEFAULT false,
  is_final      boolean NOT NULL DEFAULT false,
  sort_order    int NOT NULL DEFAULT 0,
  UNIQUE (definition_id, code)
);

CREATE TABLE workflow_transitions (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  definition_id BIGINT NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
  code          text NOT NULL,
  label         text NOT NULL,
  from_state_id BIGINT REFERENCES workflow_states(id),  -- null = from any state
  to_state_id   BIGINT NOT NULL REFERENCES workflow_states(id),
  guards        JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [{key, params}]
  actions       JSONB NOT NULL DEFAULT '[]'::jsonb,   -- [{key, params}]
  sort_order    int NOT NULL DEFAULT 0,
  UNIQUE (definition_id, code)
);

CREATE TABLE workflow_instances (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  definition_id    BIGINT NOT NULL REFERENCES workflow_definitions(id),
  entity_type      text NOT NULL,
  entity_id        BIGINT NOT NULL,
  current_state_id BIGINT NOT NULL REFERENCES workflow_states(id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (definition_id, entity_type, entity_id)
);
CREATE INDEX idx_wf_instances_entity ON workflow_instances(entity_type, entity_id);

CREATE TABLE workflow_transition_log (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  instance_id   BIGINT NOT NULL REFERENCES workflow_instances(id) ON DELETE CASCADE,
  transition_id BIGINT NOT NULL REFERENCES workflow_transitions(id),
  from_state_id BIGINT,
  to_state_id   BIGINT NOT NULL,
  actor_type    text NOT NULL,
  actor_id      BIGINT,
  context       JSONB,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE automation_rules (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text NOT NULL,
  trigger_event   text NOT NULL,            -- 'order.status_changed','quote.created','schedule.hourly'
  conditions      JSONB NOT NULL DEFAULT '[]'::jsonb,
  actions         JSONB NOT NULL DEFAULT '[]'::jsonb,
  is_active       boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_automation_event ON automation_rules(trigger_event) WHERE is_active;

CREATE TABLE automation_executions (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  rule_id       BIGINT NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
  event_payload JSONB NOT NULL,
  status        text NOT NULL CHECK (status IN ('ok','error')),
  result        JSONB,
  created_at    timestamptz NOT NULL DEFAULT now()
);

-- ===== Seed the default ORDER workflow for the demo org =====================
-- Mirrors the previously-hardcoded order state machine. Allowed transitions now
-- live here; the engine reads them at runtime.
INSERT INTO workflow_definitions (organization_id, code, entity_type, name)
VALUES (1, 'order_default', 'order', 'Default order lifecycle');

INSERT INTO workflow_states (definition_id, code, label, is_initial, is_final, sort_order)
SELECT d.id, s.code, s.label, s.is_initial, s.is_final, s.sort_order
  FROM workflow_definitions d
  CROSS JOIN (VALUES
    ('pending',    'Pending',    true,  false, 1),
    ('confirmed',  'Confirmed',  false, false, 2),
    ('processing', 'Processing', false, false, 3),
    ('shipped',    'Shipped',    false, false, 4),
    ('delivered',  'Delivered',  false, false, 5),
    ('closed',     'Closed',     false, true,  6),
    ('on_hold',    'On hold',    false, false, 7),
    ('cancelled',  'Cancelled',  false, true,  8)
  ) AS s(code, label, is_initial, is_final, sort_order)
 WHERE d.organization_id = 1 AND d.code = 'order_default';

-- Transitions (from_state, to_state) reproduce the old map exactly. Codes are
-- unique per definition; the engine resolves a move by (current_state → target).
INSERT INTO workflow_transitions (definition_id, code, label, from_state_id, to_state_id, sort_order)
SELECT d.id, t.code, t.label, fs.id, ts.id, t.sort_order
  FROM workflow_definitions d
  CROSS JOIN (VALUES
    ('confirm',          'Confirm',         'pending',    'confirmed',  1),
    ('resume',           'Resume',          'on_hold',    'confirmed',  2),
    ('process',          'Process',         'confirmed',  'processing', 3),
    ('ship',             'Ship',            'processing', 'shipped',    4),
    ('deliver',          'Deliver',         'shipped',    'delivered',  5),
    ('close',            'Close',           'delivered',  'closed',     6),
    ('hold_pending',     'Hold',            'pending',    'on_hold',    7),
    ('hold_confirmed',   'Hold',            'confirmed',  'on_hold',    8),
    ('hold_processing',  'Hold',            'processing', 'on_hold',    9),
    ('cancel_pending',   'Cancel',          'pending',    'cancelled', 10),
    ('cancel_confirmed', 'Cancel',          'confirmed',  'cancelled', 11),
    ('cancel_processing','Cancel',          'processing', 'cancelled', 12),
    ('cancel_onhold',    'Cancel',          'on_hold',    'cancelled', 13)
  ) AS t(code, label, from_code, to_code, sort_order)
  JOIN workflow_states fs ON fs.definition_id = d.id AND fs.code = t.from_code
  JOIN workflow_states ts ON ts.definition_id = d.id AND ts.code = t.to_code
 WHERE d.organization_id = 1 AND d.code = 'order_default';
