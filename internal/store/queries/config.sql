-- Hierarchical config settings (migration 0036).

-- name: ListConfigSettings :many
SELECT * FROM config_settings
WHERE organization_id = $1
ORDER BY key, scope, scope_id NULLS FIRST;

-- UpsertConfigSetting sets (or replaces) a value at a specific scope.
-- name: UpsertConfigSetting :one
INSERT INTO config_settings (organization_id, scope, scope_id, key, value)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (organization_id, scope, scope_id, key)
DO UPDATE SET value = EXCLUDED.value, updated_at = now()
RETURNING *;

-- name: DeleteConfigSetting :execrows
DELETE FROM config_settings WHERE id = $1 AND organization_id = $2;

-- ResolveConfig returns the most specific value for a key given the optional
-- website/group/customer in scope (customer > group > website > org).
-- name: ResolveConfig :one
SELECT value, scope FROM config_settings
WHERE organization_id = $1 AND key = $2 AND (
  scope = 'org'
  OR (scope = 'website'  AND scope_id = sqlc.narg('website'))
  OR (scope = 'group'    AND scope_id = sqlc.narg('grp'))
  OR (scope = 'customer' AND scope_id = sqlc.narg('customer'))
)
ORDER BY CASE scope
  WHEN 'customer' THEN 4
  WHEN 'group'    THEN 3
  WHEN 'website'  THEN 2
  ELSE 1
END DESC
LIMIT 1;
