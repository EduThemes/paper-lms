-- 000109_blackout_dates.up.sql
--
-- Course pacing blackout dates (Canvas BlackoutDate): spans of days on
-- which a published pace schedules no work. Course-scoped.

BEGIN;

CREATE TABLE IF NOT EXISTS blackout_dates (
    id          bigserial PRIMARY KEY,
    course_id   bigint NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    event_title text   NOT NULL,
    start_date  date   NOT NULL,
    end_date    date   NOT NULL,
    created_at  timestamptz,
    updated_at  timestamptz,
    CONSTRAINT blackout_dates_range_check CHECK (end_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_blackout_dates_course_id
    ON blackout_dates (course_id);

COMMIT;
