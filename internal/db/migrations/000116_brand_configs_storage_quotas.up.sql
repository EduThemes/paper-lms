-- G118: per-account theme (allow-listed CSS variables + logo).
CREATE TABLE IF NOT EXISTS brand_configs (
    id bigserial,
    account_id bigint NOT NULL,
    variables jsonb NOT NULL DEFAULT '{}'::jsonb,
    logo_data bytea,
    logo_content_type text NOT NULL DEFAULT ''::text,
    updated_by_user_id bigint NOT NULL DEFAULT 0,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_brand_configs_account_id ON public.brand_configs USING btree (account_id);

-- G120: storage quotas. Per-context overrides (Course / User / Group) and
-- account defaults (AccountCourseDefault / AccountUserDefault /
-- AccountGroupDefault, context_id = account id), in MB.
CREATE TABLE IF NOT EXISTS storage_quotas (
    id bigserial,
    account_id bigint NOT NULL,
    context_type text NOT NULL,
    context_id bigint NOT NULL,
    quota_mb bigint NOT NULL,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT storage_quotas_context_type_check CHECK (context_type IN (
        'Course', 'User', 'Group',
        'AccountCourseDefault', 'AccountUserDefault', 'AccountGroupDefault')),
    CONSTRAINT storage_quotas_quota_mb_check CHECK (quota_mb >= 0)
);

CREATE INDEX IF NOT EXISTS idx_storage_quotas_account_id ON public.storage_quotas USING btree (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_quotas_context ON public.storage_quotas USING btree (context_type, context_id);
