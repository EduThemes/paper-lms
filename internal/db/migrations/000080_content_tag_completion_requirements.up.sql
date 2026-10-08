-- 000080_content_tag_completion_requirements.up.sql
--
-- Module item completion requirements (Canvas ContextModule
-- completion_requirements). Each module item can carry one requirement:
-- must_view | must_submit | must_contribute | min_score | must_mark_done,
-- with min_score holding the threshold for the min_score type.
--
-- completion_type is NOT NULL DEFAULT '' (empty = no requirement), which
-- matches the Go zero value GORM writes, so no BeforeCreate hook is
-- needed. min_score is nullable.

BEGIN;

ALTER TABLE content_tags
    ADD COLUMN IF NOT EXISTS completion_type text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS min_score double precision;

COMMIT;
