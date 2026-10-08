-- 000128_inbox_completeness.down.sql

BEGIN;

DROP TABLE IF EXISTS conversation_message_recipients;
DROP TABLE IF EXISTS conversation_message_attachments;

ALTER TABLE conversation_participants ALTER COLUMN workflow_state SET DEFAULT 'active';
UPDATE conversation_participants
   SET workflow_state = 'active'
 WHERE workflow_state IN ('unread', 'read', 'archived');

DROP INDEX IF EXISTS idx_conversation_participants_user_state;
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS cleared_at;
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS starred;

DROP INDEX IF EXISTS idx_conversations_context;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_type;

COMMIT;
