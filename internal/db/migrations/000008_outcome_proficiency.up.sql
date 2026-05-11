-- Outcome Proficiency scales (Canvas-compatible)
CREATE TABLE IF NOT EXISTS outcome_proficiencies (
    id              BIGSERIAL PRIMARY KEY,
    context_type    VARCHAR(32) NOT NULL,
    context_id      BIGINT      NOT NULL,
    workflow_state  VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outcome_proficiency_context
    ON outcome_proficiencies (context_type, context_id);

CREATE TABLE IF NOT EXISTS outcome_proficiency_ratings (
    id                       BIGSERIAL PRIMARY KEY,
    outcome_proficiency_id   BIGINT      NOT NULL REFERENCES outcome_proficiencies(id) ON DELETE CASCADE,
    description              VARCHAR(255) NOT NULL,
    points                   DOUBLE PRECISION NOT NULL,
    mastery                  BOOLEAN     NOT NULL DEFAULT FALSE,
    color                    VARCHAR(16) NOT NULL DEFAULT '#999999',
    position                 INTEGER     NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outcome_proficiency_ratings_proficiency
    ON outcome_proficiency_ratings (outcome_proficiency_id);
