-- 0052_org_payment_configs.sql — per-tenant payment gateway selection (SAAS.md #4).
-- Tenants are their own merchants of record: each org picks a gateway and stores
-- its credentials encrypted at rest (AES-GCM, app-level key — never plaintext,
-- never in config_settings where the admin list endpoint would echo them).
-- Branding + email sender identity need no schema: they live in config_settings
-- under the branding.* / email.* keys.

CREATE TABLE org_payment_configs (
  organization_id  BIGINT PRIMARY KEY REFERENCES organizations(id),
  gateway          text NOT NULL DEFAULT 'mock',
  credentials_enc  bytea,            -- AES-GCM sealed JSON object; NULL = none stored
  updated_at       timestamptz NOT NULL DEFAULT now()
);
