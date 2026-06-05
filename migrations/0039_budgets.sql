-- Procurement budgets & cost-center spend controls (pain point #6): a buying
-- company can cap spend per cost-center over a period; orders are tagged with a
-- cost center and checked against the remaining budget at placement. Additive —
-- with no budget configured, ordering is unchanged.
ALTER TABLE orders ADD COLUMN cost_center text;

CREATE TABLE customer_budgets (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  cost_center text NOT NULL DEFAULT '',          -- '' = company-wide
  period      text NOT NULL CHECK (period IN ('monthly','quarterly','annual')),
  amount      NUMERIC(15,4) NOT NULL,
  currency    CHAR(3) NOT NULL,
  is_active   boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (customer_id, cost_center)
);
CREATE INDEX idx_customer_budgets_customer ON customer_budgets(customer_id);
