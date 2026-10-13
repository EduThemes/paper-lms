-- 000135_calendar_events_time_zone_backfill.up.sql
--
-- Gives existing all-day events their creator's profile zone (see 000134).
-- The calendar form has always stored an all-day event's start as midnight
-- in the creator's profile zone, so this pins each one to the date it was
-- created for. Only all-day rows are touched (timed events are absolute
-- instants and need no zone), and only when the profile zone is a name
-- Postgres knows; anything else stays NULL and keeps the old behaviour.

UPDATE calendar_events ce
SET time_zone = u.time_zone
FROM users u
WHERE ce.created_by_user_id = u.id
  AND ce.all_day = true
  AND ce.time_zone IS NULL
  AND u.time_zone IS NOT NULL
  AND u.time_zone IN (SELECT name FROM pg_timezone_names);
