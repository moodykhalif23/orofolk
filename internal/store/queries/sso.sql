-- SSO / federated identity (PRD §15).

-- ===== Identity providers ==================================================

-- name: CreateIdentityProvider :one
INSERT INTO identity_providers (organization_id, type, name, audience, customer_id, config, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListIdentityProviders :many
SELECT * FROM identity_providers WHERE organization_id = $1 ORDER BY id;

-- name: GetIdentityProvider :one
SELECT * FROM identity_providers WHERE organization_id = $1 AND id = $2;

-- GetIdentityProviderByID resolves a provider without org (public login/callback).
-- name: GetIdentityProviderByID :one
SELECT * FROM identity_providers WHERE id = $1;

-- name: UpdateIdentityProvider :one
UPDATE identity_providers
   SET type = $3, name = $4, audience = $5, customer_id = $6, config = $7, is_active = $8
 WHERE organization_id = $1 AND id = $2
RETURNING *;

-- ===== Login state (CSRF/replay) ===========================================

-- name: CreateSSOState :one
INSERT INTO sso_states (provider_id, state, nonce, redirect_to, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSSOState :one
SELECT * FROM sso_states WHERE state = $1;

-- name: DeleteSSOState :exec
DELETE FROM sso_states WHERE id = $1;

-- ===== External identities (IdP subject ↔ local user) ======================

-- name: GetExternalIdentity :one
SELECT * FROM external_identities WHERE provider_id = $1 AND subject = $2;

-- name: CreateExternalIdentity :one
INSERT INTO external_identities (provider_id, subject, user_id, customer_user_id, email)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- GetCustomerUserByEmail links a buyer SSO subject to an existing customer-user.
-- name: GetCustomerUserByEmail :one
SELECT id, customer_id FROM customer_users WHERE customer_id = $1 AND email = $2;
