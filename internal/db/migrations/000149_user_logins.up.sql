-- G115: multiple logins (Canvas pseudonyms) per user. users.login_id stays
-- the primary login; user_logins carries the extra ones (a second local
-- username, a second SIS id, or an SSO provider's subject).
CREATE TABLE IF NOT EXISTS user_logins (
    id bigserial,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id bigint NOT NULL,
    unique_id text NOT NULL,
    unique_id_normalized text NOT NULL,
    sis_user_id text,
    authentication_provider_id bigint,
    password_hash text NOT NULL DEFAULT ''::text,
    workflow_state text NOT NULL DEFAULT 'active'::text,
    last_login_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT user_logins_workflow_state_check CHECK (workflow_state IN ('active', 'suspended', 'deleted'))
);

CREATE INDEX IF NOT EXISTS idx_user_logins_user_id ON public.user_logins USING btree (user_id);
CREATE INDEX IF NOT EXISTS idx_user_logins_account_id ON public.user_logins USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_user_logins_authentication_provider_id ON public.user_logins USING btree (authentication_provider_id);
CREATE INDEX IF NOT EXISTS idx_user_logins_sis_user_id ON public.user_logins USING btree (sis_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_logins_unique_id_active
    ON public.user_logins USING btree (unique_id_normalized) WHERE (workflow_state = 'active'::text);
