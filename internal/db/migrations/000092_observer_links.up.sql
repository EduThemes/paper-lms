-- 000092_observer_links.up.sql
--
-- B177: a durable observer -> student pairing (Canvas's
-- user_observation_links). Previously the pairing existed only as the
-- ObserverEnrollment rows created at link time, so it was a snapshot:
-- pairing failed while the child had no courses, later courses never
-- got an observer enrollment, and dropped courses stayed visible.
-- Enrollment.AfterSave now mirrors StudentEnrollments onto the linked
-- observers' ObserverEnrollments for every active link.

BEGIN;

CREATE TABLE IF NOT EXISTS observer_links (
    id             bigserial PRIMARY KEY,
    observer_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id     bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id     bigint NOT NULL,
    workflow_state text   NOT NULL DEFAULT 'active',
    created_at     timestamptz,
    updated_at     timestamptz,
    CONSTRAINT observer_links_workflow_state_check
        CHECK (workflow_state IN ('active', 'deleted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_observer_links_observer_student
    ON observer_links (observer_id, student_id);
CREATE INDEX IF NOT EXISTS idx_observer_links_student_id
    ON observer_links (student_id);
CREATE INDEX IF NOT EXISTS idx_observer_links_account_id
    ON observer_links (account_id);

-- Backfill: every existing pairing is an active ObserverEnrollment with
-- an associated student. The link inherits the student's tenant.
INSERT INTO observer_links (observer_id, student_id, account_id, workflow_state, created_at, updated_at)
SELECT DISTINCT e.user_id, e.associated_user_id, u.account_id, 'active', now(), now()
  FROM enrollments e
  JOIN users u ON u.id = e.associated_user_id
 WHERE e.type = 'ObserverEnrollment'
   AND e.associated_user_id IS NOT NULL
   AND e.user_id IS NOT NULL
   AND e.workflow_state = 'active'
ON CONFLICT (observer_id, student_id) DO NOTHING;

COMMIT;
