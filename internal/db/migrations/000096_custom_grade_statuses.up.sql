-- 000096_custom_grade_statuses.up.sql
--
-- Account-defined grade statuses (Canvas custom_grade_statuses). A
-- submission points at one through submissions.custom_grade_status_id
-- (added in 000093).
CREATE TABLE IF NOT EXISTS custom_grade_statuses (
    id bigserial,
    account_id bigint NOT NULL,
    name text NOT NULL,
    color text NOT NULL DEFAULT '#E0E0E0'::text,
    applies_to_submissions boolean NOT NULL DEFAULT false,
    applies_to_final_grade boolean NOT NULL DEFAULT false,
    workflow_state text NOT NULL DEFAULT 'active'::text,
    created_by_id bigint,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_custom_grade_statuses_account_id ON public.custom_grade_statuses USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_custom_grade_statuses_workflow_state ON public.custom_grade_statuses USING btree (workflow_state);
