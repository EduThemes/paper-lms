-- 000128_inbox_completeness.up.sql
--
-- G092: inbox parity with Canvas conversations.
--   * conversations.context_type/context_id — the course or group a
--     conversation was sent in (inbox course/group filter).
--   * conversation_participants.starred / cleared_at and the per-participant
--     workflow_state values unread / read / archived (Canvas semantics).
--     The legacy value 'active' is backfilled to unread or read.
--   * conversation_message_attachments — files attached to a message.
--   * conversation_message_recipients — a message restricted to a subset of
--     the participants ("Reply" vs "Reply all").

BEGIN;

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS context_type text;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS context_id bigint;
CREATE INDEX IF NOT EXISTS idx_conversations_context
    ON conversations (context_type, context_id);

ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS starred boolean NOT NULL DEFAULT false;
ALTER TABLE conversation_participants ADD COLUMN IF NOT EXISTS cleared_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_conversation_participants_user_state
    ON conversation_participants (user_id, workflow_state);

UPDATE conversation_participants p
   SET workflow_state = CASE
         WHEN EXISTS (
             SELECT 1
               FROM conversation_messages m
              WHERE m.conversation_id = p.conversation_id
                AND m.user_id <> p.user_id
                AND COALESCE(m.workflow_state, 'active') <> 'deleted'
                AND (p.last_read_at IS NULL OR m.created_at > p.last_read_at))
         THEN 'unread'
         ELSE 'read'
       END
 WHERE p.workflow_state IS NULL OR p.workflow_state = 'active';

ALTER TABLE conversation_participants ALTER COLUMN workflow_state SET DEFAULT 'read';

CREATE TABLE IF NOT EXISTS conversation_message_attachments (
    id                      bigserial PRIMARY KEY,
    conversation_message_id bigint NOT NULL REFERENCES conversation_messages(id) ON DELETE CASCADE,
    attachment_id           bigint NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    created_at              timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conv_msg_attachment
    ON conversation_message_attachments (conversation_message_id, attachment_id);
CREATE INDEX IF NOT EXISTS idx_conversation_message_attachments_attachment_id
    ON conversation_message_attachments (attachment_id);

CREATE TABLE IF NOT EXISTS conversation_message_recipients (
    id                      bigserial PRIMARY KEY,
    conversation_message_id bigint NOT NULL REFERENCES conversation_messages(id) ON DELETE CASCADE,
    user_id                 bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at              timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conv_msg_recipient
    ON conversation_message_recipients (conversation_message_id, user_id);
CREATE INDEX IF NOT EXISTS idx_conversation_message_recipients_user_id
    ON conversation_message_recipients (user_id);

COMMIT;
