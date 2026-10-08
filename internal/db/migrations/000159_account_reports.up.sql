-- 000159_account_reports.up.sql
--
-- G138: Canvas account reports (/api/v1/accounts/:account_id/reports).
-- One row per report run; the generated CSV / ZIP lives in the storage
-- backend under file_key. workflow_state uses Canvas's report statuses.

BEGIN;

CREATE TABLE IF NOT EXISTS account_reports (
    id bigserial,
    account_id bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    report_type text NOT NULL,
    workflow_state text NOT NULL DEFAULT 'created',
    parameters text NOT NULL DEFAULT '{}',
    progress bigint NOT NULL DEFAULT 0,
    message text,
    file_key text,
    file_name text,
    content_type text,
    file_size bigint NOT NULL DEFAULT 0,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT account_reports_workflow_state_check
        CHECK (workflow_state IN ('created', 'running', 'complete', 'error', 'aborted', 'deleted'))
);

CREATE INDEX IF NOT EXISTS idx_account_reports_account_id ON public.account_reports USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_account_reports_report_type ON public.account_reports USING btree (report_type);
CREATE INDEX IF NOT EXISTS idx_account_reports_user_id ON public.account_reports USING btree (user_id);

COMMIT;
