-- Phase 5 / Item 8: Commons content library (Canvas Commons equivalent).
-- District-scoped content sharing — teachers publish course resources to a
-- catalog scoped to their account; other teachers in the same account can
-- import them back into their own courses. The actual payload is stored as
-- jsonb in content_snapshot for fast retrieval and cheap clone-on-import.

CREATE TABLE IF NOT EXISTS shared_content (
    id                BIGSERIAL    PRIMARY KEY,
    account_id        BIGINT       NOT NULL DEFAULT 1,
    author_user_id    BIGINT       NOT NULL,
    title             VARCHAR(512) NOT NULL,
    description       TEXT         NOT NULL DEFAULT '',
    resource_type     VARCHAR(64)  NOT NULL,
    source_course_id  BIGINT       NOT NULL DEFAULT 0,
    source_content_id BIGINT,
    subject           VARCHAR(128) NOT NULL DEFAULT '',
    grade_level       VARCHAR(32)  NOT NULL DEFAULT '',
    tags              JSONB        NOT NULL DEFAULT '[]'::jsonb,
    thumbnail_url     VARCHAR(1024) NOT NULL DEFAULT '',
    download_count    INTEGER      NOT NULL DEFAULT 0,
    favorite_count    INTEGER      NOT NULL DEFAULT 0,
    visibility        VARCHAR(32)  NOT NULL DEFAULT 'account',
    content_snapshot  JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shared_content_account_resource
    ON shared_content (account_id, resource_type);

CREATE INDEX IF NOT EXISTS idx_shared_content_account_subject
    ON shared_content (account_id, subject);

CREATE INDEX IF NOT EXISTS idx_shared_content_account_grade
    ON shared_content (account_id, grade_level);

CREATE INDEX IF NOT EXISTS idx_shared_content_author
    ON shared_content (author_user_id);

CREATE INDEX IF NOT EXISTS idx_shared_content_source_course
    ON shared_content (source_course_id);

CREATE TABLE IF NOT EXISTS shared_content_favorites (
    id                BIGSERIAL    PRIMARY KEY,
    shared_content_id BIGINT       NOT NULL REFERENCES shared_content(id) ON DELETE CASCADE,
    user_id           BIGINT       NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT idx_shared_fav_unique UNIQUE (shared_content_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_shared_content_favorites_user
    ON shared_content_favorites (user_id);
