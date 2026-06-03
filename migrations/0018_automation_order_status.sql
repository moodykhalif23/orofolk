-- Seed an order.status_changed automation rule (Pack 2 §3.5 per-entity event):
-- whenever an order changes status, email the customer a status update. No
-- conditions (fires on every change); admins can edit/disable it in the
-- automation rule editor. Demonstrates the dispatcher's per-entity half.
INSERT INTO automation_rules (organization_id, name, trigger_event, conditions, actions, is_active)
VALUES (
  1,
  'Notify customer on order status change',
  'order.status_changed',
  '[]'::jsonb,
  '[{"key": "email_customer", "params": {"template": "order_status_update"}}]'::jsonb,
  true
);
