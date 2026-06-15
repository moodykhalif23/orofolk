-- Programmatic API keys (Platform roadmap, Phase 0). The raw key is never
-- stored; authentication resolves a key by the SHA-256 hash of the bearer token.

-- name: CreateAPIKey :one
INSERT INTO api_keys (organization_id, name, prefix, key_hash, scopes, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, organization_id, name, prefix, scopes, last_used_at, expires_at, revoked_at, created_by, created_at;

-- name: ListAPIKeys :many
SELECT id, organization_id, name, prefix, scopes, last_used_at, expires_at, revoked_at, created_by, created_at
FROM api_keys
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: GetAPIKey :one
SELECT id, organization_id, name, prefix, scopes, last_used_at, expires_at, revoked_at, created_by, created_at
FROM api_keys
WHERE organization_id = $1 AND id = $2;

-- GetAPIKeyByHash resolves a key for authentication (org comes from the row).
-- Excludes revoked and expired keys.
-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- RotateAPIKey swaps in a new secret (hash + prefix) for an existing key and
-- clears any prior revocation.
-- name: RotateAPIKey :one
UPDATE api_keys
   SET key_hash = $3, prefix = $4, revoked_at = NULL
 WHERE organization_id = $1 AND id = $2
RETURNING id, organization_id, name, prefix, scopes, last_used_at, expires_at, revoked_at, created_by, created_at;

-- RevokeAPIKey soft-revokes a key (it stops authenticating immediately).
-- name: RevokeAPIKey :exec
UPDATE api_keys SET revoked_at = now()
WHERE organization_id = $1 AND id = $2 AND revoked_at IS NULL;

-- TouchAPIKey records last use, debounced to at most once a minute so an
-- authenticated request does not write on every call.
-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');
