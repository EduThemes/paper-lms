-- G117: account reports (provisioning, SIS export, grade export, last
-- user access, unpublished / recently deleted courses, ...). The generated
-- CSV is stored on the row.
CREATE TABLE IF NOT EXISTS account_reports (
    id bigserial,
    account_id bigint NOT NULL,
    user_id bigint NOT NULL,
    report_type text NOT NULL,
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
    workflow_state text NOT NULL DEFAULT 'created'::text,
    progress bigint NOT NULL DEFAULT 0,
    message text NOT NULL DEFAULT ''::text,
    row_count bigint NOT NULL DEFAULT 0,
    file_name text NOT NULL DEFAULT ''::text,
    file_data bytea,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT account_reports_workflow_state_check CHECK (workflow_state IN ('created', 'running', 'complete', 'error', 'aborted'))
);

CREATE INDEX IF NOT EXISTS idx_account_reports_account_id ON public.account_reports USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_account_reports_report_type ON public.account_reports USING btree (report_type);
CREATE INDEX IF NOT EXISTS idx_account_reports_user_id ON public.account_reports USING btree (user_id);
