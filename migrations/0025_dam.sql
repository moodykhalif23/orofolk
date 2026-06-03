-- Digital Asset Management (Pack 3 §2): extends media_assets with a public id,
-- checksum-based dedupe, size, and a processing lifecycle; adds transformation
-- presets, derived renditions, and tags.

ALTER TABLE media_assets
  ADD COLUMN IF NOT EXISTS public_id  UUID NOT NULL DEFAULT gen_random_uuid(),
  ADD COLUMN IF NOT EXISTS checksum   text,
  ADD COLUMN IF NOT EXISTS size_bytes bigint,
  ADD COLUMN IF NOT EXISTS status     text NOT NULL DEFAULT 'ready'
               CHECK (status IN ('uploading','processing','ready','error'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_media_public_id ON media_assets(public_id);
-- One asset per (org, checksum): re-uploading identical bytes reuses the asset.
CREATE UNIQUE INDEX IF NOT EXISTS uq_media_checksum
  ON media_assets(organization_id, checksum) WHERE checksum IS NOT NULL;

CREATE TABLE IF NOT EXISTS transformation_presets (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  name            text NOT NULL,
  width           int,
  height          int,
  fit             text NOT NULL DEFAULT 'cover' CHECK (fit IN ('cover','contain','fill','inside')),
  format          text NOT NULL DEFAULT 'jpeg' CHECK (format IN ('webp','avif','jpeg','png')),
  quality         int NOT NULL DEFAULT 82,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, name)
);

CREATE TABLE IF NOT EXISTS media_renditions (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  media_asset_id  BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
  preset          text NOT NULL,
  url             text NOT NULL,
  width           int,
  height          int,
  format          text,
  size_bytes      bigint,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (media_asset_id, preset)
);
CREATE INDEX IF NOT EXISTS idx_renditions_asset ON media_renditions(media_asset_id);

CREATE TABLE IF NOT EXISTS media_tags (
  media_asset_id  BIGINT NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
  tag             text NOT NULL,
  PRIMARY KEY (media_asset_id, tag)
);

-- Default responsive presets for the demo org. The pure-Go processor emits
-- JPEG; swap in a libvips processor to serve WebP/AVIF from the same presets.
INSERT INTO transformation_presets (organization_id, name, width, height, fit, format, quality)
VALUES
  (1, 'thumb', 200, 200, 'cover', 'jpeg', 80),
  (1, 'card',  600, 400, 'cover', 'jpeg', 82),
  (1, 'hero', 1600, 600, 'cover', 'jpeg', 82)
ON CONFLICT (organization_id, name) DO NOTHING;
