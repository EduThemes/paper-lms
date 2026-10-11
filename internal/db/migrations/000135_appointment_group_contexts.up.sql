-- 000135_appointment_group_contexts.up.sql
--
-- Scheduler parity (G097):
--   * appointment_group_contexts — extra courses (context_type 'Course')
--     and section limits (context_type 'CourseSection') for a group.
--     appointment_groups.course_id stays the primary (tenant) course.
--   * appointment_groups.participant_type / group_category_id — 'Group'
--     appointment groups are reserved by a student group of that
--     category rather than by individual students.
--   * appointment_reservations.participant_group_id — the student group
--     a group reservation holds the slot for; comments — the note the
--     participant left when signing up.

CREATE TABLE IF NOT EXISTS appointment_group_contexts (
    id bigserial PRIMARY KEY,
    appointment_group_id bigint NOT NULL REFERENCES appointment_groups(id) ON DELETE CASCADE,
    context_type text NOT NULL,
    context_id bigint NOT NULL,
    created_at timestamptz,
    CONSTRAINT chk_appt_group_ctx_type CHECK (context_type IN ('Course', 'CourseSection'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_appt_group_ctx_unique
    ON appointment_group_contexts (appointment_group_id, context_type, context_id);

ALTER TABLE appointment_groups ADD COLUMN IF NOT EXISTS participant_type text NOT NULL DEFAULT 'User';
ALTER TABLE appointment_groups ADD COLUMN IF NOT EXISTS group_category_id bigint;
ALTER TABLE appointment_groups DROP CONSTRAINT IF EXISTS chk_appt_group_participant_type;
ALTER TABLE appointment_groups ADD CONSTRAINT chk_appt_group_participant_type
    CHECK (participant_type IN ('User', 'Group'));
CREATE INDEX IF NOT EXISTS idx_appt_group_category ON appointment_groups (group_category_id);

ALTER TABLE appointment_reservations ADD COLUMN IF NOT EXISTS participant_group_id bigint;
ALTER TABLE appointment_reservations ADD COLUMN IF NOT EXISTS comments text;
CREATE INDEX IF NOT EXISTS idx_appt_res_participant_group ON appointment_reservations (participant_group_id);

-- One live reservation per student group per slot (the per-user unique
-- index from 000007 does not cover a groupmate booking the same slot).
CREATE UNIQUE INDEX IF NOT EXISTS idx_appt_res_unique_active_group
    ON appointment_reservations (slot_id, participant_group_id)
    WHERE workflow_state = 'reserved' AND participant_group_id IS NOT NULL;
