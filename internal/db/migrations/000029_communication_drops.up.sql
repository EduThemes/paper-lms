-- Wave 2c: drop legacy columns on communication + collaboration domain tables.
-- DATA-DESTRUCTIVE.
--
-- Deprecation window: added 2026-05-11. Operators with production data that
-- predates this migration MUST have run 000022 first; that migration copies
-- legacy data into the new GORM-model columns. Once this migration runs the
-- dropped columns are gone — the .down.sql can recreate the shape but not
-- the data.
--
-- Column categories dropped per table:
--
--   WAVE_2B_SOURCE — data was copied to a new GORM column in 000022:
--     discussion_entry_participants.read           → read_at
--     announcement_read_receipts.read              → read_at
--     lti_tool_configurations.oidc_initiation_url  → o_id_c_initiation_url
--     calendar_events.user_id                      → (context_type='User', context_id)
--     page_views.asset_type / asset_id             → context_type / context_id
--
--   SOFT_DELETE_LEFTOVER — deleted_at not modelled on any of these structs.
--
--   UNKNOWN / NO_GORM_FIELD — column is absent from the GORM model struct and
--     has zero non-test Go references:
--
--     discussion_topics:           published, position
--     discussion_entries:          depth
--     discussion_entry_ratings:    created_at, updated_at
--     discussion_entry_participants: forced_read_state (boolean; *string field
--                                   lives on discussion_topic_participants, not here)
--     discussion_topic_participants: unread_entry_count
--     discussion_entry_versions:   updated_at
--     announcements:               context_type, context_id, position,
--                                  is_section_specific, locked
--     announcement_read_receipts:  created_at, updated_at
--     lti_tool_configurations:     disabled, settings
--     context_external_tools:      settings
--     lti_resource_links:          custom (replaced by custom_parameters added
--                                  in 000016), description
--     lti_line_items:              context_external_tool_id
--     conversations:               context_type, context_id, message_count
--     conversation_participants:   last_message_at (model field is last_read_at)
--     conversation_messages:       author_id (model uses user_id), generated,
--                                  forwarded_message_ids
--     notification_preferences:    frequency, notification_category
--     notifications:               category, read, subject, url
--     notification_deliveries:     communication_channel_id, delivery_type,
--                                  workflow_state, error_message, next_retry_at,
--                                  digest_batch_id
--     conferences:                 conference_key, long_running, recording_url
--     page_views:                  updated_at, user_agent, http_method, remote_ip
--
--   INTENTIONAL_OMISSION — column exists but model deliberately excludes it;
--     no non-test Go code references it.
--
-- Tables with NO stale columns (no-op):
--   discussion_checkpoints, discussion_checkpoint_submissions,
--   lti_results, collaborations, conference_participants,
--   appointment_groups, appointment_slots, appointment_reservations

BEGIN;

-- ============================================================================
-- discussion_topics
-- ============================================================================
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS published;
ALTER TABLE discussion_topics DROP COLUMN IF EXISTS position;

-- ============================================================================
-- discussion_entries
-- ============================================================================
ALTER TABLE discussion_entries DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE discussion_entries DROP COLUMN IF EXISTS depth;

-- ============================================================================
-- discussion_entry_ratings
-- ============================================================================
ALTER TABLE discussion_entry_ratings DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE discussion_entry_ratings DROP COLUMN IF EXISTS created_at;
ALTER TABLE discussion_entry_ratings DROP COLUMN IF EXISTS updated_at;

-- ============================================================================
-- discussion_entry_participants
-- ============================================================================
-- Wave 2b source: read (bool) → read_at (timestamptz)
ALTER TABLE discussion_entry_participants DROP COLUMN IF EXISTS read;
-- UNKNOWN boolean column; the text version lives on discussion_topic_participants
ALTER TABLE discussion_entry_participants DROP COLUMN IF EXISTS forced_read_state;
ALTER TABLE discussion_entry_participants DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- discussion_topic_participants
-- ============================================================================
ALTER TABLE discussion_topic_participants DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE discussion_topic_participants DROP COLUMN IF EXISTS unread_entry_count;

-- ============================================================================
-- discussion_entry_versions
-- ============================================================================
ALTER TABLE discussion_entry_versions DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE discussion_entry_versions DROP COLUMN IF EXISTS updated_at;

-- ============================================================================
-- discussion_checkpoints  (no stale columns — no-op)
-- ============================================================================

-- ============================================================================
-- discussion_checkpoint_submissions  (no stale columns — no-op)
-- ============================================================================

-- ============================================================================
-- announcements
-- ============================================================================
ALTER TABLE announcements DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE announcements DROP COLUMN IF EXISTS context_type;
ALTER TABLE announcements DROP COLUMN IF EXISTS context_id;
ALTER TABLE announcements DROP COLUMN IF EXISTS position;
ALTER TABLE announcements DROP COLUMN IF EXISTS is_section_specific;
ALTER TABLE announcements DROP COLUMN IF EXISTS locked;

-- ============================================================================
-- announcement_read_receipts
-- ============================================================================
-- Wave 2b source: read (bool) → read_at (timestamptz)
ALTER TABLE announcement_read_receipts DROP COLUMN IF EXISTS read;
ALTER TABLE announcement_read_receipts DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE announcement_read_receipts DROP COLUMN IF EXISTS created_at;
ALTER TABLE announcement_read_receipts DROP COLUMN IF EXISTS updated_at;

-- ============================================================================
-- lti_tool_configurations
-- ============================================================================
-- Wave 2b source: oidc_initiation_url → o_id_c_initiation_url
ALTER TABLE lti_tool_configurations DROP COLUMN IF EXISTS oidc_initiation_url;
ALTER TABLE lti_tool_configurations DROP COLUMN IF EXISTS disabled;
ALTER TABLE lti_tool_configurations DROP COLUMN IF EXISTS settings;
ALTER TABLE lti_tool_configurations DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- context_external_tools
-- ============================================================================
ALTER TABLE context_external_tools DROP COLUMN IF EXISTS settings;
ALTER TABLE context_external_tools DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- lti_resource_links
-- ============================================================================
-- custom was replaced by custom_parameters (added in 000016)
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS custom;
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS description;
ALTER TABLE lti_resource_links DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- lti_line_items
-- ============================================================================
ALTER TABLE lti_line_items DROP COLUMN IF EXISTS context_external_tool_id;
ALTER TABLE lti_line_items DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- lti_results  (no stale UNKNOWN columns — only deleted_at)
-- ============================================================================
ALTER TABLE lti_results DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- calendar_events
-- ============================================================================
-- Wave 2b source: user_id → (context_type='User', context_id)
ALTER TABLE calendar_events DROP COLUMN IF EXISTS user_id;
ALTER TABLE calendar_events DROP COLUMN IF EXISTS all_day_date;
ALTER TABLE calendar_events DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- conversations
-- ============================================================================
ALTER TABLE conversations DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_type;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS message_count;

-- ============================================================================
-- conversation_participants
-- ============================================================================
-- last_message_at is not in model (model has last_read_at, added in 000016)
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS last_message_at;
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- conversation_messages
-- ============================================================================
-- author_id replaced by user_id (added in 000016); generated and
-- forwarded_message_ids have no GORM model fields
ALTER TABLE conversation_messages DROP COLUMN IF EXISTS author_id;
ALTER TABLE conversation_messages DROP COLUMN IF EXISTS generated;
ALTER TABLE conversation_messages DROP COLUMN IF EXISTS forwarded_message_ids;
ALTER TABLE conversation_messages DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- notification_preferences
-- ============================================================================
-- frequency replaced by policy; notification_category has no model field
ALTER TABLE notification_preferences DROP COLUMN IF EXISTS frequency;
ALTER TABLE notification_preferences DROP COLUMN IF EXISTS notification_category;
ALTER TABLE notification_preferences DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- notifications
-- ============================================================================
-- category, read, subject, url have no GORM model fields
-- (model has notification_type, is_read, title, message, context_type, context_id)
ALTER TABLE notifications DROP COLUMN IF EXISTS category;
ALTER TABLE notifications DROP COLUMN IF EXISTS read;
ALTER TABLE notifications DROP COLUMN IF EXISTS subject;
ALTER TABLE notifications DROP COLUMN IF EXISTS url;
ALTER TABLE notifications DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- notification_deliveries
-- ============================================================================
-- All six legacy columns replaced by new GORM model columns added in 000016
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS communication_channel_id;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS delivery_type;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS workflow_state;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS error_message;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS next_retry_at;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS digest_batch_id;
ALTER TABLE notification_deliveries DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- collaborations  (no stale UNKNOWN columns — only deleted_at)
-- ============================================================================
ALTER TABLE collaborations DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- conferences
-- ============================================================================
ALTER TABLE conferences DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE conferences DROP COLUMN IF EXISTS conference_key;
ALTER TABLE conferences DROP COLUMN IF EXISTS long_running;
ALTER TABLE conferences DROP COLUMN IF EXISTS recording_url;

-- ============================================================================
-- conference_participants  (no stale UNKNOWN columns — only deleted_at)
-- ============================================================================
ALTER TABLE conference_participants DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- page_views
-- ============================================================================
-- Wave 2b sources: asset_type → context_type, asset_id → context_id
ALTER TABLE page_views DROP COLUMN IF EXISTS asset_type;
ALTER TABLE page_views DROP COLUMN IF EXISTS asset_id;
-- UNKNOWN / no model field
ALTER TABLE page_views DROP COLUMN IF EXISTS updated_at;
ALTER TABLE page_views DROP COLUMN IF EXISTS user_agent;
ALTER TABLE page_views DROP COLUMN IF EXISTS http_method;
ALTER TABLE page_views DROP COLUMN IF EXISTS remote_ip;
ALTER TABLE page_views DROP COLUMN IF EXISTS deleted_at;

-- ============================================================================
-- appointment_groups  (no stale columns — created clean in 000007)
-- ============================================================================

-- ============================================================================
-- appointment_slots  (no stale columns — created clean in 000007)
-- ============================================================================

-- ============================================================================
-- appointment_reservations  (no stale columns — created clean in 000007)
-- ============================================================================

COMMIT;
