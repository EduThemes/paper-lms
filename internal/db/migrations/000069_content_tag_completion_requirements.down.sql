-- 000069_content_tag_completion_requirements.down.sql

BEGIN;

ALTER TABLE content_tags
    DROP COLUMN IF EXISTS completion_type,
    DROP COLUMN IF EXISTS min_score;

COMMIT;
