-- Attach the order-approval gate (Pack 2 §3.6) to the `confirm` transition of
-- the default order workflow: the amount_lte_limit guard blocks confirming an
-- order whose grand_total exceeds the placing buyer's spending_limit. With no
-- limit set (most orders), the guard is inert and confirm proceeds as before.
UPDATE workflow_transitions t
SET guards = '[{"key":"amount_lte_limit","params":{"field":"grand_total","limit_field":"spending_limit"}}]'::jsonb
FROM workflow_definitions d
WHERE t.definition_id = d.id
  AND d.organization_id = 1 AND d.code = 'order_default'
  AND t.code = 'confirm';
