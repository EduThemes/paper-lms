-- Paper LMS: feature_flags table.
-- Stores per-context overrides of feature definitions hard-coded in
-- internal/domain/models/feature_flag.go. Mirrors Canvas's feature_flags
-- table layout.
CREATE TABLE IF NOT EXISTS feature_flags (
    id           bigserial PRIMARY KEY,
    feature      varchar(255) NOT NULL,
    state        varchar(32)  NOT NULL DEFAULT 'allowed',
    context_type varchar(32)  NOT NULL,
    context_id   bigint       NOT NULL,
    created_at   timestamptz,
    updated_at   timestamptz
);

-- Natural key: one flag per (context, feature). Lookup pattern matches
-- the FindByContext repo method exactly.
CREATE UNIQUE INDEX IF NOT EXISTS idx_feature_flags_context_feature
    ON feature_flags(context_type, context_id, feature);

CREATE INDEX IF NOT EXISTS idx_feature_flags_feature
    ON feature_flags(feature);
