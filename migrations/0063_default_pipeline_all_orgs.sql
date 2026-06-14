-- 0063_default_pipeline_all_orgs.sql — every org needs a default CRM pipeline so
-- lead conversion and the pipeline board work. Migration 0013 seeded only the
-- demo org (1), and tenant provisioning never created one — so every org created
-- via signup before this migration has no pipeline, and converting a lead fails
-- at GetDefaultPipeline. Idempotent: only orgs/pipelines lacking the data get it.

-- A default pipeline for any org that doesn't already have one.
INSERT INTO pipelines (organization_id, name, is_default)
SELECT o.id, 'Default', true
  FROM organizations o
 WHERE NOT EXISTS (
   SELECT 1 FROM pipelines p WHERE p.organization_id = o.id AND p.is_default
 );

-- The standard stage set for any default pipeline that has no stages yet (covers
-- the rows just inserted; org 1's already-seeded pipeline is skipped).
INSERT INTO pipeline_stages (pipeline_id, code, label, probability, is_won, is_lost, sort_order)
SELECT p.id, s.code, s.label, s.probability, s.is_won, s.is_lost, s.sort_order
  FROM pipelines p
  CROSS JOIN (VALUES
    ('new',         'New',         10.0, false, false, 1),
    ('qualified',   'Qualified',   25.0, false, false, 2),
    ('proposal',    'Proposal',    50.0, false, false, 3),
    ('negotiation', 'Negotiation', 75.0, false, false, 4),
    ('won',         'Won',        100.0, true,  false, 5),
    ('lost',        'Lost',         0.0, false, true,  6)
  ) AS s(code, label, probability, is_won, is_lost, sort_order)
 WHERE p.is_default
   AND NOT EXISTS (SELECT 1 FROM pipeline_stages st WHERE st.pipeline_id = p.id);
