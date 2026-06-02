-- Catalog & PIM — Implementation Pack 1 §3.
-- The minimal products table already exists (0002_catalog.sql) with the GIN
-- attribute index and the slug unique index; here we add the attribute system,
-- the missing product columns (variant parent + attribute family), categories
-- (queried via the subtree CTE §12.3), media, translations, and visibility.

CREATE TABLE attribute_families (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text NOT NULL,
  UNIQUE (organization_id, name)
);

CREATE TABLE attributes (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  code            text NOT NULL,            -- machine key used in products.attributes JSONB
  label           text NOT NULL,
  data_type       text NOT NULL CHECK (data_type IN
                    ('text','number','boolean','select','multiselect','date','file','price')),
  options         JSONB,                    -- for select/multiselect: ["red","blue"]
  is_filterable   boolean NOT NULL DEFAULT false,
  is_variant_axis boolean NOT NULL DEFAULT false,
  UNIQUE (organization_id, code)
);

CREATE TABLE attribute_family_attributes (
  family_id    BIGINT NOT NULL REFERENCES attribute_families(id) ON DELETE CASCADE,
  attribute_id BIGINT NOT NULL REFERENCES attributes(id) ON DELETE CASCADE,
  is_required  boolean NOT NULL DEFAULT false,
  sort_order   int NOT NULL DEFAULT 0,
  PRIMARY KEY (family_id, attribute_id)
);

-- Bring products up to the full §3 shape: variant->configurable parent + family.
ALTER TABLE products
  ADD COLUMN parent_id           BIGINT REFERENCES products(id),
  ADD COLUMN attribute_family_id BIGINT REFERENCES attribute_families(id);
CREATE INDEX idx_products_parent ON products(parent_id);

CREATE TABLE product_translations (
  product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  locale      text NOT NULL,
  name        text NOT NULL,
  description text,
  PRIMARY KEY (product_id, locale)
);

CREATE TABLE product_media (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  url        text NOT NULL,
  type       text NOT NULL DEFAULT 'image' CHECK (type IN ('image','document','video')),
  alt        text,
  sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX idx_product_media_product ON product_media(product_id);

CREATE TABLE categories (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  parent_id       BIGINT REFERENCES categories(id),
  name            text NOT NULL,
  slug            text NOT NULL,
  sort_order      int NOT NULL DEFAULT 0,
  UNIQUE (organization_id, slug)
);
CREATE INDEX idx_categories_parent ON categories(parent_id);

CREATE TABLE product_categories (
  product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (product_id, category_id)
);

-- Per-customer / per-group visibility. No rows for an actor = full visibility.
CREATE TABLE catalog_visibility (
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id        BIGINT REFERENCES products(id) ON DELETE CASCADE,
  category_id       BIGINT REFERENCES categories(id) ON DELETE CASCADE,
  customer_id       BIGINT REFERENCES customers(id) ON DELETE CASCADE,
  customer_group_id BIGINT REFERENCES customer_groups(id) ON DELETE CASCADE,
  visible           boolean NOT NULL DEFAULT true,
  CHECK ( (product_id IS NOT NULL)::int + (category_id IS NOT NULL)::int = 1 ),
  CHECK ( (customer_id IS NOT NULL)::int + (customer_group_id IS NOT NULL)::int = 1 )
);

-- Catalog permissions for the demo admin role.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.permission
  FROM roles r
  CROSS JOIN (VALUES
    ('product.manage'), ('category.view'), ('category.manage'),
    ('attribute.view'), ('attribute.manage')
  ) AS p(permission)
 WHERE r.organization_id = 1 AND r.name = 'admin'
ON CONFLICT DO NOTHING;
