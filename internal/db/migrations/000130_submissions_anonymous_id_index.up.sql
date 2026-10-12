-- 000130_submissions_anonymous_id_index.up.sql
--
-- Index for submissions.anonymous_id (column added by 000082), built
-- concurrently: submissions is one of the largest tables.
--
-- CREATE INDEX CONCURRENTLY cannot run inside a transaction block, so this
-- file holds exactly one statement: golang-migrate sends the file as one
-- simple-protocol query, which Postgres runs outside a transaction when it
-- contains a single statement. Keep it that way (no BEGIN/COMMIT, no second
-- statement), or Postgres rejects CONCURRENTLY and the migration fails.
-- If the build is interrupted it can leave an INVALID index behind, which
-- IF NOT EXISTS would then skip: drop it (DROP INDEX CONCURRENTLY) and
-- re-run the migration.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_submissions_anonymous_id
    ON public.submissions USING btree (anonymous_id);
