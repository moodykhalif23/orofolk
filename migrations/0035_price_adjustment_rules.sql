-- Rule-based pricing (PRD §7.2): adjust a resolved base price by a percent or
-- fixed amount, scoped by customer group and/or a product attribute match. This
-- is additive on top of the existing tiered (min_quantity) prices + price-list
-- inheritance: with no rules, prices are unchanged. The highest-priority active
-- matching rule wins.
CREATE TABLE price_adjustment_rules (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id   BIGINT NOT NULL REFERENCES organizations(id),
  name              text NOT NULL,
  customer_group_id BIGINT REFERENCES customer_groups(id) ON DELETE CASCADE,  -- null = all buyers
  attribute_key     text,                                                     -- null = all products
  attribute_value   text,                                                     -- matched against products.attributes[key]
  adjustment_type   text NOT NULL CHECK (adjustment_type IN ('percent','amount')),
  adjustment_value  NUMERIC(15,4) NOT NULL,   -- percent: -10 => -10%; amount: signed currency delta
  priority          int NOT NULL DEFAULT 0,   -- higher wins
  is_active         boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  -- An attribute match needs both key and value, or neither.
  CHECK ((attribute_key IS NULL) = (attribute_value IS NULL))
);
CREATE INDEX idx_price_adjustment_rules_org ON price_adjustment_rules(organization_id, is_active, priority DESC);
