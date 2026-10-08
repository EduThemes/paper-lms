-- 000129_notification_policies.up.sql
--
-- G091: per-category x per-channel notification preferences (Canvas
-- notification_policies). One row per (user, channel, category) the user
-- has set explicitly; anything else falls back to notification_preferences.
-- communication_channel_id 0 means the user's account (login) email, which
-- has no communication_channels row.

BEGIN;

CREATE TABLE IF NOT EXISTS notification_policies (
    id                       bigserial PRIMARY KEY,
    user_id                  bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    communication_channel_id bigint NOT NULL DEFAULT 0,
    category                 text   NOT NULL,
    frequency                text   NOT NULL DEFAULT 'daily',
    created_at               timestamptz,
    updated_at               timestamptz,
    CONSTRAINT notification_policies_frequency_check
        CHECK (frequency IN ('immediately', 'daily', 'weekly', 'never'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_policy_user_channel_category
    ON notification_policies (user_id, communication_channel_id, category);

COMMIT;
