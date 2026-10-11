-- 000095_file_visibility.down.sql

ALTER TABLE folders DROP COLUMN IF EXISTS hidden;
ALTER TABLE folders DROP COLUMN IF EXISTS locked;

ALTER TABLE attachments DROP COLUMN IF EXISTS usage_rights_license;
ALTER TABLE attachments DROP COLUMN IF EXISTS usage_rights_legal_copyright;
ALTER TABLE attachments DROP COLUMN IF EXISTS usage_rights_justification;
ALTER TABLE attachments DROP COLUMN IF EXISTS unlock_at;
ALTER TABLE attachments DROP COLUMN IF EXISTS lock_at;
ALTER TABLE attachments DROP COLUMN IF EXISTS hidden;
ALTER TABLE attachments DROP COLUMN IF EXISTS locked;
