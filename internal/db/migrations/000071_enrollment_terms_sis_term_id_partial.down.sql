-- 000071_enrollment_terms_sis_term_id_partial.down.sql
--
-- Restores the original deployment-wide UNIQUE index on sis_term_id.
-- NULL rows are allowed by a plain UNIQUE index, so blank ids stay NULL.
-- This fails if two tenants now share a SIS term id; resolve those
-- duplicates before rolling back.

BEGIN;

DROP INDEX IF EXISTS idx_enrollment_terms_sis_term_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_terms_sis_term_id
    ON enrollment_terms USING btree (sis_term_id);

COMMIT;
