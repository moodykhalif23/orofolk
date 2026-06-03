-- Tax/VAT + shipping adapters (Pack 2 §4.3, §4.4). Both ship a rules-based local
-- provider (config-driven, no external calls); external Avalara/TaxJar/carrier
-- adapters plug in behind the same Go interfaces later.

-- Products carry a tax class; the local VAT provider keys rates on (region, class).
ALTER TABLE products ADD COLUMN tax_class text NOT NULL DEFAULT 'standard';

-- Per-(region, tax-class) VAT rate. rate is a fraction, e.g. 0.1600 = 16%.
-- An 'exempt' class (or a 0 rate row) models exemptions.
CREATE TABLE tax_rates (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  country         text NOT NULL,                 -- ISO-3166 alpha-2, e.g. 'KE'
  tax_class       text NOT NULL DEFAULT 'standard',
  rate            NUMERIC(6,4) NOT NULL DEFAULT 0,
  name            text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, country, tax_class)
);
CREATE INDEX idx_tax_rates_lookup ON tax_rates(organization_id, country);

-- Table-rate shipping by (region, service). free_over waives the fee above a
-- subtotal threshold (NULL = never free).
CREATE TABLE shipping_rates (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  country         text NOT NULL,
  service         text NOT NULL,                 -- 'standard','express',...
  carrier         text NOT NULL DEFAULT 'local',
  amount          NUMERIC(15,4) NOT NULL DEFAULT 0,
  free_over       NUMERIC(15,4),
  is_active       boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, country, service)
);
CREATE INDEX idx_shipping_rates_lookup ON shipping_rates(organization_id, country);

-- Tax + shipping admin permissions for the demo admin role.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES ('tax.view'), ('tax.manage'), ('shipping.view'), ('shipping.manage')) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
