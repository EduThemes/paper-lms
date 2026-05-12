-- Reverse the data copies from 000023_admin_misc_rename.up.sql.
--
-- Each statement back-populates the legacy column from the new one when the
-- legacy column is at its zero value, mirroring the up direction.
--
-- Notes:
--   portfolio_artifacts: source_type is used to disambiguate which legacy
--     FK (attachment_id vs submission_id) the source_id refers to. The
--     reversal is lossy if an artifact had both attachment_id and
--     submission_id set (impossible in the old schema — they were mutually
--     exclusive), so no data loss occurs.
--
--   portfolio_artifacts section_id → portfolio_section_id: lossy only for
--     rows where section_id was originally NULL and was not populated by up;
--     this is acceptable for Wave 2b additive migration.
--
--   role_overrides: reversal back-fills custom_role_id from role_id when
--     custom_role_id is still NULL.

BEGIN;

-- Reverse 5: role_id → custom_role_id.
UPDATE role_overrides
SET custom_role_id = role_id
WHERE (custom_role_id IS NULL OR custom_role_id = 0)
  AND role_id > 0;

-- Reverse 4: (source_type='course_submission', source_id) → submission_id.
UPDATE portfolio_artifacts
SET submission_id = source_id
WHERE submission_id IS NULL
  AND source_type = 'course_submission'
  AND source_id IS NOT NULL
  AND source_id > 0;

-- Reverse 3: (source_type='upload', source_id) → attachment_id.
UPDATE portfolio_artifacts
SET attachment_id = source_id
WHERE attachment_id IS NULL
  AND source_type = 'upload'
  AND source_id IS NOT NULL
  AND source_id > 0;

-- Reverse 2: section_id → portfolio_section_id.
UPDATE portfolio_artifacts
SET portfolio_section_id = section_id
WHERE (portfolio_section_id IS NULL OR portfolio_section_id = 0)
  AND section_id IS NOT NULL
  AND section_id > 0;

-- Reverse 1: is_under13 → is_under_13.
UPDATE age_verifications
SET is_under_13 = true
WHERE is_under13 = true
  AND (is_under_13 IS NULL OR is_under_13 = false);

COMMIT;
