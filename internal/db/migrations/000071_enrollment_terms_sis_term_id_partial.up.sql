-- 000091_enrollment_terms_sis_term_id_partial.up.sql
--
-- B071: idx_enrollment_terms_sis_term_id was a plain UNIQUE index on
-- sis_term_id. The admin UI sends sis_term_id = '' for a term without a
-- SIS id, so the second such term collided on the index. The index was
-- also deployment-wide, so two tenants could not share a SIS term id.
--
-- Blank ids become NULL, and the index is rebuilt (same name, so the
-- GORM model tag and the schema-parity test stay aligned) as a partial
-- UNIQUE on (account_id, sis_term_id) that skips NULL rows — Canvas
-- scopes SIS ids to the root account the same way.

BEGIN;

UPDATE enrollment_terms SET sis_term_id = NULL WHERE btrim(sis_term_id) = '';

DROP INDEX IF EXISTS idx_enrollment_terms_sis_term_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_terms_sis_term_id
    ON enrollment_terms (account_id, sis_term_id)
    WHERE sis_term_id IS NOT NULL;

COMMIT;
