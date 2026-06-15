-- 0074_feed_schedule.sql — Phase 4 slice 3: scheduled regeneration + delivery.
-- A feed can regenerate on a cadence (hourly/daily); each build stores an
-- artifact in the blob store and stamps when/where/how-big, so a channel polls a
-- stable signed URL that serves the last-built document. 'manual' feeds (the
-- default, and every slice-1/2 feed) only build on demand and have no next_run_at.

ALTER TABLE feeds
  ADD COLUMN schedule          text   NOT NULL DEFAULT 'manual'
                                 CHECK (schedule IN ('manual', 'hourly', 'daily')),
  ADD COLUMN next_run_at        timestamptz,            -- when the sweep should next build (null = manual)
  ADD COLUMN last_built_at      timestamptz,            -- when the artifact was last generated
  ADD COLUMN last_artifact_key  text   NOT NULL DEFAULT '', -- blob key of the last build ('' = never built)
  ADD COLUMN last_bytes         bigint NOT NULL DEFAULT 0,  -- size of the last artifact
  ADD COLUMN last_error         text   NOT NULL DEFAULT ''; -- last build failure ('' = ok)

-- The scheduler sweeps feeds due to run, oldest first.
CREATE INDEX idx_feeds_due ON feeds (next_run_at)
  WHERE schedule <> 'manual' AND next_run_at IS NOT NULL;
