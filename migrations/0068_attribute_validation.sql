-- 0068_attribute_validation.sql — Phase 1: per-attribute validation rules.
-- An attribute can declare constraints its values must satisfy (regex + length
-- for text, numeric range for number/price, selection count for multiselect;
-- select/multiselect already constrain to `options`). Rules live on the
-- attribute definition alongside `options`, and the validation engine enforces
-- them on product writes and CSV import. Empty object = no constraints.
ALTER TABLE attributes
  ADD COLUMN validation JSONB NOT NULL DEFAULT '{}'::jsonb;
