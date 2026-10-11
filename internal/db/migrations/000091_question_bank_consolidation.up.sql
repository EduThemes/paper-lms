-- 000091_question_bank_consolidation.up.sql
--
-- One question-bank system (quiz-items lane, G060). Paper had two
-- parallel bank stores: question_banks / question_bank_entries (used by
-- question groups and the Canvas course import) and quiz_item_banks /
-- quiz_item_bank_items (used by the item-bank page and the QTI importer).
-- question_banks becomes the only store:
--
--   1. question_banks gains description, created_by_user_id, account_id
--      and shared_with_account (a bank other courses in the same account
--      may draw from); question_bank_entries gains the per-outcome
--      feedback columns item-bank items carried.
--   2. Every quiz_item_banks / quiz_item_bank_items row is copied in
--      (legacy_* columns record the source id so the copy is idempotent).
--   3. quiz_questions.bank_item_id is remapped onto the copied entry ids,
--      since the /quiz_item_banks API now reads and writes these tables.
--
-- quiz_item_banks / quiz_item_bank_items are left in place (Wave A of the
-- copy-then-drop pattern); a later migration drops them.

ALTER TABLE question_banks ADD COLUMN IF NOT EXISTS description text;
ALTER TABLE question_banks ADD COLUMN IF NOT EXISTS created_by_user_id bigint;
ALTER TABLE question_banks ADD COLUMN IF NOT EXISTS account_id bigint;
ALTER TABLE question_banks ADD COLUMN IF NOT EXISTS shared_with_account boolean NOT NULL DEFAULT false;
ALTER TABLE question_banks ADD COLUMN IF NOT EXISTS legacy_quiz_item_bank_id bigint;

CREATE INDEX IF NOT EXISTS idx_question_banks_account_id ON question_banks (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_question_banks_legacy_quiz_item_bank_id
    ON question_banks (legacy_quiz_item_bank_id) WHERE legacy_quiz_item_bank_id IS NOT NULL;

ALTER TABLE question_bank_entries ADD COLUMN IF NOT EXISTS correct_comments text;
ALTER TABLE question_bank_entries ADD COLUMN IF NOT EXISTS incorrect_comments text;
ALTER TABLE question_bank_entries ADD COLUMN IF NOT EXISTS neutral_comments text;
ALTER TABLE question_bank_entries ADD COLUMN IF NOT EXISTS legacy_quiz_item_bank_item_id bigint;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_question_bank_entries_legacy_item_id
    ON question_bank_entries (legacy_quiz_item_bank_item_id) WHERE legacy_quiz_item_bank_item_id IS NOT NULL;

-- Owning account of existing banks (the course's tenant).
UPDATE question_banks b
   SET account_id = c.account_id
  FROM courses c
 WHERE c.id = b.course_id
   AND b.account_id IS NULL;

-- Copy item banks.
INSERT INTO question_banks
    (course_id, title, description, created_by_user_id, account_id, workflow_state,
     shared_with_account, created_at, updated_at, legacy_quiz_item_bank_id)
SELECT ib.course_id, ib.title, ib.description, ib.created_by_user_id, c.account_id, 'active',
       false, ib.created_at, ib.updated_at, ib.id
  FROM quiz_item_banks ib
  LEFT JOIN courses c ON c.id = ib.course_id
 WHERE NOT EXISTS (
       SELECT 1 FROM question_banks qb WHERE qb.legacy_quiz_item_bank_id = ib.id);

-- Copy item-bank items.
INSERT INTO question_bank_entries
    (question_bank_id, question_name, question_type, question_text, points_possible,
     answers, feedback, position, correct_comments, incorrect_comments, neutral_comments,
     created_at, updated_at, legacy_quiz_item_bank_item_id)
SELECT qb.id, '', it.question_type, it.question_text, COALESCE(it.points_possible, 1),
       COALESCE(it.answers::text, '[]'), '', it.position,
       it.correct_comments, it.incorrect_comments, it.neutral_comments,
       it.created_at, it.updated_at, it.id
  FROM quiz_item_bank_items it
  JOIN question_banks qb ON qb.legacy_quiz_item_bank_id = it.bank_id
 WHERE NOT EXISTS (
       SELECT 1 FROM question_bank_entries e WHERE e.legacy_quiz_item_bank_item_id = it.id);

-- Questions added from an item bank now point at the copied entry.
UPDATE quiz_questions q
   SET bank_item_id = e.id
  FROM question_bank_entries e
 WHERE e.legacy_quiz_item_bank_item_id = q.bank_item_id
   AND q.bank_item_id IS NOT NULL;
