-- 0071_import_options.sql — Phase 3 slice 3: matching & cleansing. An import run
-- records its options — a match field (so custom-object records can upsert, not
-- just insert) and the value-cleansing applied on ingest — so a commit re-applies
-- the same matching the dry run used.
ALTER TABLE import_runs ADD COLUMN options JSONB NOT NULL DEFAULT '{}'::jsonb;
