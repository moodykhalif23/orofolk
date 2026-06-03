-- name: GetUserByEmail :one
SELECT id, organization_id, email, password_hash, full_name, is_active
FROM users
WHERE organization_id = $1 AND email = $2 AND is_active = true;

-- name: GetUserPermissions :many
SELECT DISTINCT rp.permission
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE ur.user_id = $1;

-- name: TouchUserLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- CreateUser provisions a seller-side user (used by SSO JIT provisioning).
-- name: CreateUser :one
INSERT INTO users (organization_id, email, password_hash, full_name)
VALUES ($1, $2, $3, $4)
RETURNING id, organization_id, email, full_name, is_active;
