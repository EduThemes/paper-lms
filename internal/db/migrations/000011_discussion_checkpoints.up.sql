-- Phase 5 / Item 6: Discussion Checkpoints (Canvas-compatible multi-deadline
-- thread participation requirements). One topic may have a reply_to_topic
-- checkpoint (initial post) plus a reply_to_entry checkpoint (N replies).

CREATE TABLE IF NOT EXISTS discussion_checkpoints (
    id                  BIGSERIAL    PRIMARY KEY,
    discussion_topic_id BIGINT       NOT NULL REFERENCES discussion_topics(id) ON DELETE CASCADE,
    checkpoint_type     VARCHAR(32)  NOT NULL CHECK (checkpoint_type IN ('reply_to_topic', 'reply_to_entry')),
    due_at              TIMESTAMPTZ,
    points_possible     NUMERIC      NOT NULL DEFAULT 0,
    required_replies    INTEGER      NOT NULL DEFAULT 0,
    workflow_state      VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discussion_checkpoints_topic_id
    ON discussion_checkpoints (discussion_topic_id);

CREATE TABLE IF NOT EXISTS discussion_checkpoint_submissions (
    id                       BIGSERIAL    PRIMARY KEY,
    discussion_checkpoint_id BIGINT       NOT NULL REFERENCES discussion_checkpoints(id) ON DELETE CASCADE,
    user_id                  BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    completed_at             TIMESTAMPTZ,
    reply_count              INTEGER      NOT NULL DEFAULT 0,
    status                   VARCHAR(32)  NOT NULL DEFAULT 'not_started'
                              CHECK (status IN ('not_started', 'in_progress', 'complete', 'completed')),
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT idx_dcs_checkpoint_user UNIQUE (discussion_checkpoint_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_discussion_checkpoint_submissions_user_id
    ON discussion_checkpoint_submissions (user_id);
