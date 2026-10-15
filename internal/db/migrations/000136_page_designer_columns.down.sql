-- 000136_page_designer_columns.down.sql

ALTER TABLE courses DROP COLUMN IF EXISTS look;

ALTER TABLE wiki_page_revisions DROP COLUMN IF EXISTS design;
ALTER TABLE wiki_page_revisions DROP COLUMN IF EXISTS css;

ALTER TABLE wiki_pages DROP COLUMN IF EXISTS design;
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS css;
