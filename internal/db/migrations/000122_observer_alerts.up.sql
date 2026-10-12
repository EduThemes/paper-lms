-- 000122_observer_alerts.up.sql
--
-- Learner dashboard lane (G149): observer alerts. An observer opts in to
-- alert types per linked student (observer_alert_thresholds); the alert
-- scan raises observer_alerts, one per (threshold, context object).

CREATE TABLE IF NOT EXISTS observer_alert_thresholds (
    id bigserial,
    account_id bigint NOT NULL,
    observer_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_type varchar(40) NOT NULL,
    threshold numeric,
    last_evaluated_at timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_observer_alert_thresholds_account_id ON public.observer_alert_thresholds USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_observer_alert_thresholds_student_id ON public.observer_alert_thresholds USING btree (student_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_observer_alert_thresholds_unique ON public.observer_alert_thresholds USING btree (observer_id, student_id, alert_type);

CREATE TABLE IF NOT EXISTS observer_alerts (
    id bigserial,
    account_id bigint NOT NULL,
    threshold_id bigint NOT NULL REFERENCES observer_alert_thresholds(id) ON DELETE CASCADE,
    observer_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_type varchar(40) NOT NULL,
    context_type varchar(40) NOT NULL,
    context_id bigint NOT NULL,
    course_id bigint,
    title text NOT NULL,
    link text,
    workflow_state text NOT NULL DEFAULT 'unread'::text
        CHECK (workflow_state IN ('unread', 'read', 'dismissed')),
    action_date timestamptz,
    created_at timestamptz,
    updated_at timestamptz,
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_observer_alerts_account_id ON public.observer_alerts USING btree (account_id);
CREATE INDEX IF NOT EXISTS idx_observer_alerts_observer_id ON public.observer_alerts USING btree (observer_id);
CREATE INDEX IF NOT EXISTS idx_observer_alerts_student_id ON public.observer_alerts USING btree (student_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_observer_alerts_unique ON public.observer_alerts USING btree (threshold_id, context_type, context_id);
