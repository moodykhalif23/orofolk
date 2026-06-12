-- Per-tenant payment gateway config (SAAS.md #4).

-- name: GetOrgPaymentConfig :one
SELECT * FROM org_payment_configs WHERE organization_id = $1;

-- name: UpsertOrgPaymentConfig :one
INSERT INTO org_payment_configs (organization_id, gateway, credentials_enc)
VALUES ($1, $2, $3)
ON CONFLICT (organization_id)
DO UPDATE SET gateway = EXCLUDED.gateway,
              credentials_enc = COALESCE(EXCLUDED.credentials_enc, org_payment_configs.credentials_enc),
              updated_at = now()
RETURNING *;
