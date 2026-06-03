-- SSO / federated identity (PRD §15 [V2]). OIDC now (OAuth2 authorization-code);
-- the model also carries SAML config for a follow-up. A provider provisions
-- either seller-side users (audience 'admin') or buyer customer_users (audience
-- 'storefront', under a mapped customer). JIT provisioning links an IdP subject
-- to a local identity.

CREATE TABLE identity_providers (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  type            text NOT NULL DEFAULT 'oidc' CHECK (type IN ('oidc','saml')),
  name            text NOT NULL,
  audience        text NOT NULL CHECK (audience IN ('admin','storefront')),
  customer_id     BIGINT REFERENCES customers(id),     -- buying company (storefront audience)
  is_active       boolean NOT NULL DEFAULT true,
  config          JSONB NOT NULL DEFAULT '{}'::jsonb,   -- issuer, endpoints, client id/secret, scopes
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_identity_providers_org ON identity_providers(organization_id);

-- Short-lived login state for the code flow (CSRF + replay protection).
CREATE TABLE sso_states (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider_id BIGINT NOT NULL REFERENCES identity_providers(id) ON DELETE CASCADE,
  state       text NOT NULL UNIQUE,
  nonce       text NOT NULL,
  redirect_to text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL
);

-- Maps an IdP subject to a local identity (exactly one of user / customer_user).
CREATE TABLE external_identities (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider_id      BIGINT NOT NULL REFERENCES identity_providers(id) ON DELETE CASCADE,
  subject          text NOT NULL,
  user_id          BIGINT REFERENCES users(id),
  customer_user_id BIGINT REFERENCES customer_users(id),
  email            text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider_id, subject)
);

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('sso.view'), ('sso.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
