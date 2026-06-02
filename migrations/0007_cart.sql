-- Cart & shopping lists — Implementation Pack 1 §5.
-- Cart line unit_price is snapshotted from combined_prices at add-time and
-- re-validated at checkout. A customer may have many shopping lists but at most
-- one default (partial unique index).

CREATE TABLE shopping_lists (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  customer_id      BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  customer_user_id BIGINT REFERENCES customer_users(id),
  name             text NOT NULL,
  is_default       boolean NOT NULL DEFAULT false,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_one_default_list ON shopping_lists(customer_id) WHERE is_default;
CREATE INDEX idx_shopping_lists_customer ON shopping_lists(customer_id);
CREATE TRIGGER trg_shopping_lists_updated BEFORE UPDATE ON shopping_lists
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE shopping_list_items (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shopping_list_id BIGINT NOT NULL REFERENCES shopping_lists(id) ON DELETE CASCADE,
  product_id       BIGINT NOT NULL REFERENCES products(id),
  quantity         NUMERIC(15,4) NOT NULL DEFAULT 1,
  unit             text NOT NULL DEFAULT 'each',
  UNIQUE (shopping_list_id, product_id, unit)
);

CREATE TABLE carts (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id        UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  customer_id      BIGINT NOT NULL REFERENCES customers(id),
  customer_user_id BIGINT REFERENCES customer_users(id),
  website_id       BIGINT NOT NULL REFERENCES websites(id),
  currency         CHAR(3) NOT NULL,
  status           text NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active','converted','abandoned')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_carts_customer ON carts(customer_id);
-- At most one active cart per customer/website keeps "the cart" unambiguous.
CREATE UNIQUE INDEX uq_one_active_cart ON carts(customer_id, website_id) WHERE status = 'active';
CREATE TRIGGER trg_carts_updated BEFORE UPDATE ON carts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE cart_items (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  cart_id     BIGINT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
  product_id  BIGINT NOT NULL REFERENCES products(id),
  quantity    NUMERIC(15,4) NOT NULL,
  unit        text NOT NULL DEFAULT 'each',
  unit_price  NUMERIC(15,4) NOT NULL,        -- snapshot at add-time
  UNIQUE (cart_id, product_id, unit)
);
CREATE INDEX idx_cart_items_cart ON cart_items(cart_id);
