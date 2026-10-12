-- Point quiz questions back at the original item-bank items, then remove
-- the copied rows and the added columns. Banks/entries created after the
-- consolidation (no legacy id) are kept. bank_item_id values the up
-- migration cleared (their legacy item had been deleted) stay NULL: the old
-- id referenced nothing.
UPDATE quiz_questions q
   SET bank_item_id = e.legacy_quiz_item_bank_item_id
  FROM question_bank_entries e
 WHERE e.id = q.bank_item_id
   AND e.legacy_quiz_item_bank_item_id IS NOT NULL;

DELETE FROM question_bank_entries WHERE legacy_quiz_item_bank_item_id IS NOT NULL;
DELETE FROM question_banks WHERE legacy_quiz_item_bank_id IS NOT NULL;

DROP INDEX IF EXISTS uniq_question_bank_entries_legacy_item_id;
ALTER TABLE question_bank_entries DROP COLUMN IF EXISTS legacy_quiz_item_bank_item_id;
ALTER TABLE question_bank_entries DROP COLUMN IF EXISTS neutral_comments;
ALTER TABLE question_bank_entries DROP COLUMN IF EXISTS incorrect_comments;
ALTER TABLE question_bank_entries DROP COLUMN IF EXISTS correct_comments;

DROP INDEX IF EXISTS uniq_question_banks_legacy_quiz_item_bank_id;
DROP INDEX IF EXISTS idx_question_banks_account_id;
ALTER TABLE question_banks DROP COLUMN IF EXISTS legacy_quiz_item_bank_id;
ALTER TABLE question_banks DROP COLUMN IF EXISTS shared_with_account;
ALTER TABLE question_banks DROP COLUMN IF EXISTS account_id;
ALTER TABLE question_banks DROP COLUMN IF EXISTS created_by_user_id;
ALTER TABLE question_banks DROP COLUMN IF EXISTS description;
