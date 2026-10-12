-- 000109_conference_providers.down.sql

ALTER TABLE conferences DROP COLUMN IF EXISTS provider_join_url;
ALTER TABLE conferences DROP COLUMN IF EXISTS provider_meeting_id;
ALTER TABLE conferences DROP COLUMN IF EXISTS recording_enabled;
