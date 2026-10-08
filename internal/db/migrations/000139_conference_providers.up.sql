-- 000139_conference_providers.up.sql
--
-- G098: conferences held on a provider (BigBlueButton or Zoom).
--
--   recording_enabled    the instructor asked for the meeting to be
--                        recordable (BBB record=true / Zoom cloud
--                        auto-recording).
--   provider_meeting_id  the provider's meeting id (the Zoom meeting id;
--                        BBB meetings use a derived id and leave it empty).
--   provider_join_url    the attendee join link the provider handed back
--                        (Zoom join_url). Never the host start link, which
--                        carries a host token and is fetched fresh on join.

ALTER TABLE conferences ADD COLUMN IF NOT EXISTS recording_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS provider_meeting_id text;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS provider_join_url text;
