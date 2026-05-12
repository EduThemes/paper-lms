-- Wave 2b: data migration for the communication + collaboration domain.
--
-- Tables in scope:
--   discussion_topics, discussion_entries, discussion_entry_ratings,
--   discussion_entry_participants, discussion_topic_participants,
--   discussion_entry_versions, discussion_checkpoints,
--   discussion_checkpoint_submissions, announcements,
--   announcement_read_receipts, lti_tool_configurations,
--   context_external_tools, lti_resource_links, lti_line_items,
--   lti_results, calendar_events, conversations, conversation_participants,
--   conversation_messages, notification_preferences, notifications,
--   notification_deliveries, collaborations, conferences,
--   conference_participants, page_views, appointment_groups,
--   appointment_slots, appointment_reservations
--
-- COPY operations (stale column → new GORM column):
--
--   1. lti_tool_configurations.oidc_initiation_url
--        → o_id_c_initiation_url      (RENAME_CANDIDATE, text NOT NULL)
--   2. announcement_read_receipts.read (bool)
--        → read_at                    (BOOL_TO_TIMESTAMP_REFACTOR, timestamptz nullable)
--   3. discussion_entry_participants.read (bool)
--        → read_at                    (BOOL_TO_TIMESTAMP_REFACTOR, timestamptz nullable)
--   4. calendar_events.user_id
--        → context_id + context_type  (POLYMORPHIC_REFACTOR; context was a User)
--   5. page_views.asset_id / asset_type
--        → context_id / context_type  (POLYMORPHIC_REFACTOR)
--
-- DEFERRED (soft-deletes, dead UNKNOWN columns, semantically ambiguous):
--   All deleted_at columns           → SOFT_DELETE_LEFTOVER (Wave 2c drops)
--   announcements.*                  → UNKNOWN, no matching GORM column; defer
--   announcement_read_receipts.{created_at,updated_at}  → UNKNOWN; defer
--   calendar_events.all_day_date     → UNKNOWN; defer
--   conferences.{conference_key,long_running,recording_url} → UNKNOWN; defer
--   context_external_tools.settings  → UNKNOWN; defer
--   conversation_messages.{author_id,forwarded_message_ids,generated} → UNKNOWN
--   conversation_participants.last_message_at → UNKNOWN; defer
--   conversations.{context_id,context_type,message_count} → UNKNOWN; defer
--   discussion_entries.depth         → UNKNOWN; defer
--   discussion_entry_participants.forced_read_state → UNKNOWN; defer
--   discussion_entry_ratings.{created_at,updated_at} → UNKNOWN; defer
--   discussion_entry_versions.updated_at → UNKNOWN; defer
--   discussion_topic_participants.unread_entry_count → UNKNOWN; defer
--   discussion_topics.{position,published} → UNKNOWN; defer
--   lti_line_items.context_external_tool_id → UNKNOWN (dead for LTILineItem); defer
--   lti_resource_links.{custom,description} → UNKNOWN; defer
--   lti_tool_configurations.{disabled,settings} → UNKNOWN; defer
--   notification_deliveries.*        → UNKNOWN; defer
--   notification_preferences.{frequency,notification_category} → UNKNOWN; defer
--   notifications.{category,read,subject,url} → UNKNOWN; defer
--   page_views.{http_method,remote_ip,updated_at,user_agent} → UNKNOWN; defer
--   appointment_groups, appointment_slots, appointment_reservations
--                                    → no stale columns; no-op
--
-- All guards use GORM zero-values. Every statement is idempotent.

BEGIN;

-- ============================================================================
-- 1. lti_tool_configurations: oidc_initiation_url → o_id_c_initiation_url
--    GORM maps OIDCInitiationURL (Go field) to o_id_c_initiation_url via its
--    default snake_case converter. Wave 1 added the new NOT NULL column.
-- ============================================================================
UPDATE lti_tool_configurations
SET o_id_c_initiation_url = oidc_initiation_url
WHERE o_id_c_initiation_url = ''
  AND oidc_initiation_url IS NOT NULL
  AND oidc_initiation_url <> '';

-- ============================================================================
-- 2. announcement_read_receipts: read (bool) → read_at (timestamptz)
--    The new model uses read_at; presence of a non-NULL timestamp means true.
--    Seed with created_at to preserve when the read was first recorded.
-- ============================================================================
UPDATE announcement_read_receipts
SET read_at = COALESCE(created_at, now())
WHERE read = true
  AND read_at IS NULL;

-- ============================================================================
-- 3. discussion_entry_participants: read (bool) → read_at (timestamptz)
--    Same bool→timestamp pattern. read_at is nullable in the GORM model.
-- ============================================================================
UPDATE discussion_entry_participants
SET read_at = COALESCE(created_at, now())
WHERE read = true
  AND read_at IS NULL;

-- ============================================================================
-- 4. calendar_events: user_id → (context_type='User', context_id)
--    The old schema stored a creator user FK in user_id. The polymorphic
--    model uses (context_type, context_id) for the owning context. Rows that
--    already have context_id populated are left untouched; rows where
--    context_id is still 0 but user_id is set are migrated with type='User'.
-- ============================================================================
UPDATE calendar_events
SET context_type = 'User',
    context_id   = user_id
WHERE context_id = 0
  AND user_id IS NOT NULL
  AND user_id > 0;

-- ============================================================================
-- 5. page_views: (asset_type, asset_id) → (context_type, context_id)
--    The old schema stored asset_type / asset_id for the viewed resource.
--    The GORM model uses the polymorphic (context_type, context_id) pair.
--    Only copy when context_id is zero to avoid overwriting already-migrated
--    rows. Both columns are nullable in the original schema.
-- ============================================================================
UPDATE page_views
SET context_type = asset_type,
    context_id   = asset_id
WHERE context_id = 0
  AND asset_id IS NOT NULL
  AND asset_id > 0
  AND asset_type IS NOT NULL
  AND asset_type <> '';

COMMIT;
