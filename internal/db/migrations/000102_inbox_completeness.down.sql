-- 000102_inbox_completeness.down.sql

BEGIN;

DROP TABLE IF EXISTS conversation_message_recipients;
DROP TABLE IF EXISTS conversation_message_attachments;

-- Only the values the up migration introduced from 'active' (unread / read)
-- are folded back. 'archived' is left as it is: the up migration never
-- produced it from 'active', so an archived row is either pre-existing (the
-- old code must not lose it) or archived since, and the pre-000102 code
-- treats every state other than 'deleted' as visible, so 'archived' reads
-- the same as 'active' there.
ALTER TABLE conversation_participants ALTER COLUMN workflow_state SET DEFAULT 'active';
UPDATE conversation_participants
   SET workflow_state = 'active'
 WHERE workflow_state IN ('unread', 'read');

DROP INDEX IF EXISTS idx_conversation_participants_user_state;
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS cleared_at;
ALTER TABLE conversation_participants DROP COLUMN IF EXISTS starred;

DROP INDEX IF EXISTS idx_conversations_context;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS context_type;

COMMIT;
