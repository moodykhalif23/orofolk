-- Configure-Price-Quote (PRD §8 V2): configurable products gain option groups,
-- options with price deltas, and pairwise rules. A chosen configuration is
-- validated + priced by internal/cpq and stored on the quote/order line.

CREATE TABLE product_configs (
  product_id  BIGINT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
  base_price  NUMERIC(15,4) NOT NULL DEFAULT 0,
  currency    CHAR(3) NOT NULL DEFAULT 'USD',
  is_active   boolean NOT NULL DEFAULT true,
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_product_configs_updated BEFORE UPDATE ON product_configs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE product_option_groups (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  code        text NOT NULL,
  name        text NOT NULL,
  required    boolean NOT NULL DEFAULT true,
  min_select  int NOT NULL DEFAULT 1,
  max_select  int NOT NULL DEFAULT 1,
  sort_order  int NOT NULL DEFAULT 0,
  UNIQUE (product_id, code)
);

CREATE TABLE product_options (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  group_id    BIGINT NOT NULL REFERENCES product_option_groups(id) ON DELETE CASCADE,
  code        text NOT NULL,
  name        text NOT NULL,
  price_delta NUMERIC(15,4) NOT NULL DEFAULT 0,
  is_default  boolean NOT NULL DEFAULT false,
  sort_order  int NOT NULL DEFAULT 0,
  UNIQUE (group_id, code)
);

CREATE TABLE config_rules (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id        BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  kind              text NOT NULL CHECK (kind IN ('requires','excludes')),
  option_id         BIGINT NOT NULL REFERENCES product_options(id) ON DELETE CASCADE,
  related_option_id BIGINT NOT NULL REFERENCES product_options(id) ON DELETE CASCADE
);
CREATE INDEX idx_config_rules_product ON config_rules(product_id);

-- Configured lines carry the resolved selection + price breakdown.
ALTER TABLE quote_items ADD COLUMN configuration JSONB;
ALTER TABLE order_items ADD COLUMN configuration JSONB;
