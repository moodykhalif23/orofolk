-- Data quality / catalog completeness (Platform roadmap, Phase 1). A product's
-- "completeness" is how many of the REQUIRED attributes in its attribute family
-- carry a meaningful value in the product's attributes JSONB. "Meaningful" means
-- the key is present and the value is not JSON null, "", [] or {} — so a set
-- multiselect or boolean-false counts as filled, an empty one does not. Products
-- with no family, or a family with no required attributes, are simply not scored.

-- name: CatalogCompletenessSummary :one
WITH pa AS (
  SELECT p.id,
         ((p.attributes -> a.code) IS NOT NULL
          AND (p.attributes -> a.code) NOT IN ('null'::jsonb, '""'::jsonb, '[]'::jsonb, '{}'::jsonb)) AS present
  FROM products p
  JOIN attribute_family_attributes afa
    ON afa.family_id = p.attribute_family_id AND afa.is_required = true
  JOIN attributes a ON a.id = afa.attribute_id
  WHERE p.organization_id = $1 AND p.deleted_at IS NULL
),
per_product AS (
  SELECT id,
         count(*) AS req_total,
         count(*) FILTER (WHERE present) AS req_present
  FROM pa
  GROUP BY id
)
SELECT
  (SELECT count(*) FROM products pt
     WHERE pt.organization_id = $1 AND pt.deleted_at IS NULL)::int AS products_total,
  (SELECT count(*) FROM products pt
     WHERE pt.organization_id = $1 AND pt.deleted_at IS NULL AND pt.attribute_family_id IS NOT NULL)::int AS products_with_family,
  count(*)::int AS products_scored,
  COALESCE(avg(req_present::float8 / req_total) * 100, 100)::float8 AS avg_completeness,
  count(*) FILTER (WHERE req_present = req_total)::int AS complete_count,
  count(*) FILTER (WHERE req_present < req_total)::int AS incomplete_count
FROM per_product;

-- name: CatalogCompletenessWorst :many
WITH pa AS (
  SELECT p.id, p.sku, p.name, a.code AS attr_code,
         ((p.attributes -> a.code) IS NOT NULL
          AND (p.attributes -> a.code) NOT IN ('null'::jsonb, '""'::jsonb, '[]'::jsonb, '{}'::jsonb)) AS present
  FROM products p
  JOIN attribute_family_attributes afa
    ON afa.family_id = p.attribute_family_id AND afa.is_required = true
  JOIN attributes a ON a.id = afa.attribute_id
  WHERE p.organization_id = $1 AND p.deleted_at IS NULL
)
SELECT id, sku, name,
       count(*)::int AS required_total,
       count(*) FILTER (WHERE present)::int AS required_present,
       COALESCE(array_agg(attr_code ORDER BY attr_code) FILTER (WHERE NOT present), '{}')::text[] AS missing
FROM pa
GROUP BY id, sku, name
HAVING count(*) FILTER (WHERE NOT present) > 0
ORDER BY count(*) FILTER (WHERE present)::float8 / count(*) ASC, name
LIMIT sqlc.arg(row_limit);
