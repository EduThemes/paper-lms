-- Canvas-compatible custom gradebook columns + per-student data.

CREATE TABLE IF NOT EXISTS custom_gradebook_columns (
    id              BIGSERIAL PRIMARY KEY,
    course_id       BIGINT       NOT NULL,
    title           VARCHAR(255) NOT NULL,
    position        INTEGER      NOT NULL DEFAULT 0,
    hidden          BOOLEAN      NOT NULL DEFAULT FALSE,
    read_only       BOOLEAN      NOT NULL DEFAULT FALSE,
    teacher_notes   BOOLEAN      NOT NULL DEFAULT FALSE,
    workflow_state  VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_custom_gradebook_columns_course_pos
    ON custom_gradebook_columns(course_id, position)
    WHERE workflow_state <> 'deleted';

CREATE INDEX IF NOT EXISTS idx_custom_gradebook_columns_workflow_state
    ON custom_gradebook_columns(workflow_state);

CREATE TABLE IF NOT EXISTS custom_gradebook_column_data (
    id                          BIGSERIAL PRIMARY KEY,
    custom_gradebook_column_id  BIGINT      NOT NULL,
    user_id                     BIGINT      NOT NULL,
    content                     TEXT,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_column_data_col_user
    ON custom_gradebook_column_data(custom_gradebook_column_id, user_id);

CREATE INDEX IF NOT EXISTS idx_custom_column_data_user
    ON custom_gradebook_column_data(user_id);
