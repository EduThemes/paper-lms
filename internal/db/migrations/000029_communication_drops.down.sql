-- Reverse of 000029: re-add the dropped columns with their original types
-- and defaults from migrations 000001, 000007, 000011, and 000016.
-- Structural only — the row data is LOST.
--
-- IMPORTANT: rows that existed when 000029 ran no longer have legacy-column
-- values. After this down runs, all legacy columns are NULL/default. If you
-- need the legacy data back, you must restore from a backup taken before
-- 000029 ran; the .up.sql is data-destructive by design.
--
-- Wave 2b source columns are re-added as nullable (the original NOT NULL
-- constraint where applicable cannot be re-enforced without data; operators
-- rolling back must restore data separately).

BEGIN;

-- ============================================================================
-- page_views
-- ============================================================================
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS remote_ip text;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS http_method text;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS user_agent text;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS asset_id bigint;
ALTER TABLE page_views ADD COLUMN IF NOT EXISTS asset_type text;

-- ============================================================================
-- conference_participants
-- ============================================================================
ALTER TABLE conference_participants ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- conferences
-- ============================================================================
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS recording_url text;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS long_running boolean DEFAULT false;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS conference_key text;
ALTER TABLE conferences ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- collaborations
-- ============================================================================
ALTER TABLE collaborations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- notification_deliveries
-- ============================================================================
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS digest_batch_id text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS next_retry_at timestamptz;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS error_message text;
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS workflow_state text DEFAULT 'pending';
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS delivery_type text DEFAULT 'email';
ALTER TABLE notification_deliveries ADD COLUMN IF NOT EXISTS communication_channel_id bigint;

-- ============================================================================
-- notifications
-- ============================================================================
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS url text;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS subject text;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS read boolean DEFAULT false;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category text;

-- ============================================================================
-- notification_preferences
-- ============================================================================
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS notification_category text;
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS frequency text DEFAULT 'immediately';

-- ============================================================================
-- conversation_messages
-- ============================================================================
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS forwarded_message_ids text;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS generated boolean DEFAULT false;
ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS author_id bigint;

-- ============================================================================
-- conversation_participants
-- ============================================================================
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS last_message_at timestamptz;

-- ============================================================================
-- conversations
-- ============================================================================
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS message_count bigint DEFAULT 0;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS context_id bigint;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS context_type text;

-- ============================================================================
-- calendar_events
-- ============================================================================
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS all_day_date timestamptz;
ALTER TABLE calendar_events ADD COLUMN IF NOT EXISTS user_id bigint;

-- ============================================================================
-- lti_results
-- ============================================================================
ALTER TABLE lti_results ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- ============================================================================
-- lti_line_items
-- ============================================================================
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE lti_line_items ADD COLUMN IF NOT EXISTS context_external_tool_id bigint;

-- ============================================================================
-- lti_resource_links
-- ============================================================================
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE lti_resource_links ADD COLUMN IF NOT EXISTS custom text;

-- ============================================================================
-- context_external_tools
-- ============================================================================
ALTER TABLE context_external_tools ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE context_external_tools ADD COLUMN IF NOT EXISTS settings text;

-- ============================================================================
-- lti_tool_configurations
-- ============================================================================
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS settings text;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS disabled boolean DEFAULT false;
ALTER TABLE lti_tool_configurations ADD COLUMN IF NOT EXISTS oidc_initiation_url text;

-- ============================================================================
-- announcement_read_receipts
-- ============================================================================
ALTER TABLE announcement_read_receipts ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE announcement_read_receipts ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE announcement_read_receipts ADD COLUMN IF NOT EXISTS created_at timestamptz;
ALTER TABLE announcement_read_receipts ADD COLUMN IF NOT EXISTS read boolean DEFAULT false;

-- ============================================================================
-- announcements
-- ============================================================================
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS locked boolean DEFAULT false;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS is_section_specific boolean DEFAULT false;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS position bigint;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS context_id bigint;
ALTER TABLE announcements ADD COLUMN IF NOT EXISTS context_type text DEFAULT 'Course';

-- ============================================================================
-- discussion_entry_versions
-- ============================================================================
ALTER TABLE discussion_entry_versions ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_entry_versions ADD COLUMN IF NOT EXISTS updated_at timestamptz;

-- ============================================================================
-- discussion_topic_participants
-- ============================================================================
ALTER TABLE discussion_topic_participants ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_topic_participants ADD COLUMN IF NOT EXISTS unread_entry_count bigint DEFAULT 0;

-- ============================================================================
-- discussion_entry_participants
-- ============================================================================
ALTER TABLE discussion_entry_participants ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_entry_participants ADD COLUMN IF NOT EXISTS forced_read_state boolean DEFAULT false;
ALTER TABLE discussion_entry_participants ADD COLUMN IF NOT EXISTS read boolean DEFAULT false;

-- ============================================================================
-- discussion_entry_ratings
-- ============================================================================
ALTER TABLE discussion_entry_ratings ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_entry_ratings ADD COLUMN IF NOT EXISTS updated_at timestamptz;
ALTER TABLE discussion_entry_ratings ADD COLUMN IF NOT EXISTS created_at timestamptz;

-- ============================================================================
-- discussion_entries
-- ============================================================================
ALTER TABLE discussion_entries ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_entries ADD COLUMN IF NOT EXISTS depth bigint DEFAULT 0;

-- ============================================================================
-- discussion_topics
-- ============================================================================
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS position bigint;
ALTER TABLE discussion_topics ADD COLUMN IF NOT EXISTS published boolean DEFAULT true;

COMMIT;
