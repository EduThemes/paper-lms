-- Phase 5 / Item 7: Smart Search via pgvector embeddings.
--
-- We try to enable the pgvector extension for native cosine search via
-- the `<=>` operator. If the extension isn't available (managed Postgres
-- without the contrib package, or older versions), the DO block swallows
-- the error and the column falls back to TEXT — the application's Vector
-- Scan/Value uses the same bracketed text encoding either way, and the
-- repository falls back to in-process cosine ranking automatically.

DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pgvector extension not available; falling back to TEXT column. Smart search will rank in-process.';
END
$$;

-- Detect whether the vector type is registered, and pick the right column type.
DO $$
DECLARE
    has_vector BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'vector'
    ) INTO has_vector;

    IF has_vector THEN
        EXECUTE $sql$
            CREATE TABLE IF NOT EXISTS content_embeddings (
                id            BIGSERIAL PRIMARY KEY,
                course_id     BIGINT      NOT NULL,
                content_type  VARCHAR(32) NOT NULL,
                content_id    BIGINT      NOT NULL,
                title         TEXT        NOT NULL DEFAULT '',
                excerpt       TEXT        NOT NULL DEFAULT '',
                embedding     vector(384),
                created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        $sql$;
    ELSE
        EXECUTE $sql$
            CREATE TABLE IF NOT EXISTS content_embeddings (
                id            BIGSERIAL PRIMARY KEY,
                course_id     BIGINT      NOT NULL,
                content_type  VARCHAR(32) NOT NULL,
                content_id    BIGINT      NOT NULL,
                title         TEXT        NOT NULL DEFAULT '',
                excerpt       TEXT        NOT NULL DEFAULT '',
                embedding     TEXT,
                created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        $sql$;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_content_embeddings_course_id
    ON content_embeddings (course_id);

CREATE INDEX IF NOT EXISTS idx_content_embeddings_type_id
    ON content_embeddings (content_type, content_id);

-- IVFFlat ANN index over cosine distance — only when pgvector is installed.
-- Lists=100 is a sensible default for small/medium tables; tune in prod.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
        EXECUTE $sql$
            CREATE INDEX IF NOT EXISTS idx_content_embeddings_embedding
                ON content_embeddings
                USING ivfflat (embedding vector_cosine_ops)
                WITH (lists = 100)
        $sql$;
    END IF;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'Could not create ivfflat index: %', SQLERRM;
END
$$;
