-- 000094_wiki_page_revisions.down.sql

ALTER TABLE wiki_pages DROP COLUMN IF EXISTS todo_date;

DROP TABLE IF EXISTS wiki_page_revisions;
