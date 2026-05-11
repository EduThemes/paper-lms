-- Roll back Phase 5 Item 7 smart search tables.
DROP INDEX IF EXISTS idx_content_embeddings_embedding;
DROP INDEX IF EXISTS idx_content_embeddings_type_id;
DROP INDEX IF EXISTS idx_content_embeddings_course_id;
DROP TABLE IF EXISTS content_embeddings;
-- We intentionally do NOT drop the pgvector extension; other tables
-- elsewhere may depend on it.
