-- Reverse of 000031.
--
-- announcements.require_acknowledgement is re-added with its original type
-- and default from 000001. Row data is LOST — operators rolling back must
-- restore from a backup if they need the column populated.

BEGIN;

DROP INDEX IF EXISTS idx_courses_enrollment_term_id;
DROP INDEX IF EXISTS idx_portfolio_artifacts_section_id;

ALTER TABLE announcements ADD COLUMN IF NOT EXISTS require_acknowledgement boolean DEFAULT false;

COMMIT;
