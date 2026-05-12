-- Wave 2 cleanup. Resolves the last three pieces of drift surfaced by the
-- parity test after waves 2b + 2c land.
--
-- 1. announcements.require_acknowledgement was the original 000001 column
--    name. GORM's default snake-case of the model field RequireAck produced
--    require_ack, which Wave 1 (000016) added. Both columns coexisted after
--    Wave 1; Wave 2c's per-domain agent kept require_acknowledgement on the
--    strength of references in handlers/announcements.go and the model
--    file. Those references all touch the Go field name and JSON tag —
--    no Go code accesses the require_acknowledgement column directly —
--    so the legacy column is safe to drop here.
--
-- 2. idx_portfolio_artifacts_section_id. Wave 1 added the section_id
--    column to portfolio_artifacts but missed the model-declared index
--    on it. Surfaced as a parity test failure once Wave 2c removed the
--    legacy portfolio_section_id and its index.
--
-- 3. idx_courses_enrollment_term_id. The Course model was missing the
--    EnrollmentTermID field even though the SQL chain has carried the
--    column since 000001 (and the enrollment_term_service issues raw
--    `Where("enrollment_term_id = ?", …)` queries against it). The
--    accompanying course.go edit adds the field as `*uint gorm:"index"`,
--    so AutoMigrate now creates this index too — the SQL chain has to
--    follow.

BEGIN;

ALTER TABLE announcements DROP COLUMN IF EXISTS require_acknowledgement;

CREATE INDEX IF NOT EXISTS idx_portfolio_artifacts_section_id
    ON portfolio_artifacts USING btree (section_id);

CREATE INDEX IF NOT EXISTS idx_courses_enrollment_term_id
    ON courses USING btree (enrollment_term_id);

COMMIT;
