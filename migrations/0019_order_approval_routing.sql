-- Full order-approval routing (Pack 2 §3.6). An over-limit order can't be
-- confirmed directly (amount_lte_limit guard on `confirm`); the rep routes it to
-- on_hold (request approval), and only an approver — someone with the
-- `order.approve` permission — can `resume` it out of on_hold to confirmed,
-- which bypasses the amount gate. The has_permission guard enforces that.

UPDATE workflow_transitions t
SET guards = '[{"key":"has_permission","params":{"permission":"order.approve"}}]'::jsonb
FROM workflow_definitions d
WHERE t.definition_id = d.id
  AND d.organization_id = 1 AND d.code = 'order_default'
  AND t.code = 'resume';

-- Grant the new approval permission to the demo admin role.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'order.approve'
  FROM roles r
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
