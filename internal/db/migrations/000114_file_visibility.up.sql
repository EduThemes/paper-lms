-- 000114_file_visibility.up.sql
--
-- File and folder visibility (G070).
--
-- attachments.locked     unpublished: learners can't list or download
-- attachments.hidden     left out of learner listings, readable by link
-- attachments.lock_at /
--   unlock_at            learner availability window
-- attachments.usage_rights_*  copyright justification / holder / license
-- folders.locked / hidden     same meaning, applied to the folder's tree
--
-- 000026 dropped earlier, never-used columns with some of these names;
-- these are new columns with NOT NULL defaults, so existing files stay
-- published and visible.

ALTER TABLE attachments ADD COLUMN IF NOT EXISTS locked boolean NOT NULL DEFAULT false;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS hidden boolean NOT NULL DEFAULT false;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS lock_at timestamptz;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS unlock_at timestamptz;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS usage_rights_justification text NOT NULL DEFAULT ''::text;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS usage_rights_legal_copyright text NOT NULL DEFAULT ''::text;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS usage_rights_license text NOT NULL DEFAULT ''::text;

ALTER TABLE folders ADD COLUMN IF NOT EXISTS locked boolean NOT NULL DEFAULT false;
ALTER TABLE folders ADD COLUMN IF NOT EXISTS hidden boolean NOT NULL DEFAULT false;
