-- Workflow engine queries — Pack 2 §3.

-- ===== Definitions / states / transitions ==================================

-- name: GetWorkflowDefByCode :one
SELECT * FROM workflow_definitions
WHERE organization_id = $1 AND code = $2 AND is_active;

-- name: ListWorkflowDefinitions :many
SELECT * FROM workflow_definitions WHERE organization_id = $1 ORDER BY code;

-- name: GetWorkflowDefinition :one
SELECT * FROM workflow_definitions WHERE organization_id = $1 AND id = $2;

-- UpdateWorkflowTransitionConfig edits a transition's guards/actions JSONB,
-- org-scoped via its definition (admin low-code editing).
-- name: UpdateWorkflowTransitionConfig :one
UPDATE workflow_transitions t
SET guards = $3, actions = $4
FROM workflow_definitions d
WHERE t.id = $2 AND t.definition_id = d.id AND d.organization_id = $1
RETURNING t.*;

-- name: GetWorkflowState :one
SELECT * FROM workflow_states WHERE id = $1;

-- name: GetWorkflowStateByCode :one
SELECT * FROM workflow_states WHERE definition_id = $1 AND code = $2;

-- name: ListWorkflowStates :many
SELECT * FROM workflow_states WHERE definition_id = $1 ORDER BY sort_order, id;

-- name: ListWorkflowTransitions :many
SELECT * FROM workflow_transitions WHERE definition_id = $1 ORDER BY sort_order, id;

-- GetTransitionToState resolves the move from the instance's current state to a
-- target state code: a transition whose to_state has that code and whose
-- from_state is the current state (or null = from any). Specific from-states
-- win over wildcard ones.
-- name: GetTransitionToState :one
SELECT t.* FROM workflow_transitions t
JOIN workflow_states ts ON ts.id = t.to_state_id
WHERE t.definition_id = $1
  AND ts.code = $2
  AND (t.from_state_id = $3 OR t.from_state_id IS NULL)
ORDER BY (t.from_state_id IS NULL), t.sort_order
LIMIT 1;

-- name: GetTransitionByCode :one
SELECT * FROM workflow_transitions WHERE definition_id = $1 AND code = $2;

-- ===== Instances ===========================================================

-- name: GetWorkflowInstance :one
SELECT * FROM workflow_instances WHERE id = $1;

-- name: GetInstanceForEntity :one
SELECT * FROM workflow_instances
WHERE definition_id = $1 AND entity_type = $2 AND entity_id = $3;

-- name: CreateWorkflowInstance :one
INSERT INTO workflow_instances (definition_id, entity_type, entity_id, current_state_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: SetWorkflowInstanceState :exec
UPDATE workflow_instances SET current_state_id = $2, updated_at = now() WHERE id = $1;

-- name: AddWorkflowTransitionLog :exec
INSERT INTO workflow_transition_log
  (instance_id, transition_id, from_state_id, to_state_id, actor_type, actor_id, context)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CountTransitionLog :one
SELECT count(*) FROM workflow_transition_log WHERE instance_id = $1;
