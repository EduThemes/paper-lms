-- 000105_calendar_series_and_section_events.up.sql
--
-- Calendar parity (G095): recurring event series and section-specific
-- child events.
--
--   * parent_calendar_event_id — a CourseSection event is a child of a
--     Course event. A parent with live children is a container the
--     calendar never shows directly.
--   * series_uuid / rrule / series_head — every occurrence of a
--     recurring event is its own row sharing series_uuid; the first is
--     the head and each row keeps the RFC 5545 RRULE it came from.

ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS parent_calendar_event_id bigint;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS series_uuid text;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS rrule text;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS series_head boolean DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_cal_event_parent ON calendar_events (parent_calendar_event_id);
CREATE INDEX IF NOT EXISTS idx_cal_event_series ON calendar_events (series_uuid);
