DROP INDEX IF EXISTS idx_cal_event_series;
DROP INDEX IF EXISTS idx_cal_event_parent;

ALTER TABLE calendar_events DROP COLUMN IF EXISTS series_head;
ALTER TABLE calendar_events DROP COLUMN IF EXISTS rrule;
ALTER TABLE calendar_events DROP COLUMN IF EXISTS series_uuid;
ALTER TABLE calendar_events DROP COLUMN IF EXISTS parent_calendar_event_id;
