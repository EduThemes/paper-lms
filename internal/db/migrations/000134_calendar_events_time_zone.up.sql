-- 000134_calendar_events_time_zone.up.sql
--
-- Each calendar event remembers the IANA time zone it was created in.
-- An all-day event is stored as the instant of its local midnight, so its
-- date (in the calendar views and as DTSTART;VALUE=DATE in ICS feeds) is
-- read in this zone. Without it the date was read in the viewer's zone and
-- an all-day event created in America/New_York moved to the previous day
-- for a viewer further west.
--
-- Nullable with no default, so adding it does not rewrite the table. NULL
-- keeps the old behaviour (the viewer's profile zone).

ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS time_zone text;
