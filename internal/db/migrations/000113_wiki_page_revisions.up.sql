-- 000113_wiki_page_revisions.up.sql
--
-- Page revision history (G065) and page to-do dates (G074).
--
-- wiki_page_revisions holds one row per saved version of a page. The
-- model, repository and service existed but the table was never created,
-- so nothing could record or read history. revision_number counts from 1
-- per page; (wiki_page_id, revision_number) is unique. edited_by is NULL
-- for the backfilled first revision, whose author is unknown.
--
-- Every live page gets a revision 1 holding its current title and body,
-- so the first edit after this migration already has something to diff
-- against and restore.

CREATE TABLE IF NOT EXISTS wiki_page_revisions (
    id bigserial,
    wiki_page_id bigint NOT NULL,
    revision_number bigint NOT NULL,
    title text NOT NULL,
    body text,
    edited_by bigint,
    created_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wiki_page_revisions_page_number ON public.wiki_page_revisions USING btree (wiki_page_id, revision_number);
CREATE INDEX IF NOT EXISTS idx_wiki_page_revisions_edited_by ON public.wiki_page_revisions USING btree (edited_by);

INSERT INTO wiki_page_revisions (wiki_page_id, revision_number, title, body, edited_by, created_at)
SELECT p.id, 1, COALESCE(p.title, ''), p.body, NULL, COALESCE(p.updated_at, p.created_at, now())
FROM wiki_pages p
WHERE p.workflow_state <> 'deleted'
  AND NOT EXISTS (SELECT 1 FROM wiki_page_revisions r WHERE r.wiki_page_id = p.id);

ALTER TABLE wiki_pages ADD COLUMN IF NOT EXISTS todo_date timestamptz;
