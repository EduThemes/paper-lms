-- 000136_page_designer_columns.up.sql
--
-- Page Designer (docs/page-designer/README.md, "Data model").
--
-- wiki_pages.css     sanitized, unscoped page stylesheet ('' = none)
-- wiki_pages.design  the PageDoc a designed page was made from (NULL = an
--                    ordinary page)
-- wiki_page_revisions.css / .design snapshot both with every revision.
-- courses.look       the course header look (NULL = Paper's generated
--                    header).
--
-- The text columns have a constant default and the jsonb columns are
-- nullable with no default, so none of these rewrite the table.

ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS css text NOT NULL DEFAULT '';
ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS design jsonb;

ALTER TABLE wiki_page_revisions ADD COLUMN IF NOT EXISTS css text NOT NULL DEFAULT '';
ALTER TABLE wiki_page_revisions ADD COLUMN IF NOT EXISTS design jsonb;

ALTER TABLE courses ADD COLUMN IF NOT EXISTS look jsonb;
