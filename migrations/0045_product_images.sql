-- Product images: link the (previously unused) product_media rows to DAM
-- media_assets so product photos reuse the upload pipeline (dedupe, validation,
-- renditions) instead of storing loose URLs. The url column is kept and
-- populated from the asset for convenient storefront reads without a join.
ALTER TABLE product_media
  ADD COLUMN IF NOT EXISTS media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_product_media_asset ON product_media(media_asset_id);
