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

-- ===== Custom-object completeness (Phase 2 slice 3) ========================
-- The same "present means a meaningful JSON value" rule, now against each custom
-- object type's REQUIRED fields — so data-health answers for every model, not
-- just products.

-- ObjectTypeCompleteness scores every object type for an org: how complete its
-- records are against the type's required fields. Types with no required fields
-- (or no records) report 100% / 0 scored.
-- name: ObjectTypeCompleteness :many
WITH req AS (
  SELECT object_type_id, code FROM object_fields
  WHERE organization_id = $1 AND is_required = true
),
rf AS (
  SELECT r.id, r.object_type_id,
         ((r.data -> req.code) IS NOT NULL
          AND (r.data -> req.code) NOT IN ('null'::jsonb, '""'::jsonb, '[]'::jsonb, '{}'::jsonb)) AS present
  FROM object_records r
  JOIN req ON req.object_type_id = r.object_type_id
  WHERE r.organization_id = $1 AND r.deleted_at IS NULL
),
per_record AS (
  SELECT id, object_type_id,
         count(*) AS req_total,
         count(*) FILTER (WHERE present) AS req_present
  FROM rf
  GROUP BY id, object_type_id
)
SELECT t.id, t.code, t.label,
  (SELECT count(*) FROM object_records orr
     WHERE orr.organization_id = $1 AND orr.object_type_id = t.id AND orr.deleted_at IS NULL)::int AS records_total,
  count(pr.id)::int AS records_scored,
  COALESCE(avg(pr.req_present::float8 / pr.req_total) * 100, 100)::float8 AS avg_completeness,
  count(pr.id) FILTER (WHERE pr.req_present = pr.req_total)::int AS complete_count,
  count(pr.id) FILTER (WHERE pr.req_present < pr.req_total)::int AS incomplete_count
FROM object_types t
LEFT JOIN per_record pr ON pr.object_type_id = t.id
WHERE t.organization_id = $1
GROUP BY t.id, t.code, t.label
ORDER BY t.label;

-- ObjectRecordCompletenessWorst lists the least-complete records of one type,
-- with exactly which required fields each is missing (the enrichment work-list).
-- name: ObjectRecordCompletenessWorst :many
WITH rf AS (
  SELECT r.id, r.public_id, f.code AS field_code,
         ((r.data -> f.code) IS NOT NULL
          AND (r.data -> f.code) NOT IN ('null'::jsonb, '""'::jsonb, '[]'::jsonb, '{}'::jsonb)) AS present
  FROM object_records r
  JOIN object_fields f ON f.object_type_id = r.object_type_id AND f.is_required = true
  WHERE r.organization_id = $1 AND r.object_type_id = $2 AND r.deleted_at IS NULL
)
SELECT id, public_id,
  count(*)::int AS required_total,
  count(*) FILTER (WHERE present)::int AS required_present,
  COALESCE(array_agg(field_code ORDER BY field_code) FILTER (WHERE NOT present), '{}')::text[] AS missing
FROM rf
GROUP BY id, public_id
HAVING count(*) FILTER (WHERE NOT present) > 0
ORDER BY count(*) FILTER (WHERE present)::float8 / count(*) ASC
LIMIT sqlc.arg(row_limit);
