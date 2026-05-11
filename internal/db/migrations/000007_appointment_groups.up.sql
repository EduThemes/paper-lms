-- Phase: Appointment Groups (Canvas-compatible Scheduler)
-- Instructors publish a set of bookable time slots; students reserve them.

CREATE TABLE IF NOT EXISTS appointment_groups (
    id bigserial PRIMARY KEY,
    course_id bigint NOT NULL,
    title text NOT NULL,
    description text,
    location_name text,
    location_address text,
    min_appointments_per_participant int NOT NULL DEFAULT 0,
    max_appointments_per_participant int NOT NULL DEFAULT 1,
    participants_per_appointment int NOT NULL DEFAULT 1,
    created_by_user_id bigint NOT NULL,
    workflow_state text NOT NULL DEFAULT 'pending',
    created_at timestamptz,
    updated_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_appt_group_course ON appointment_groups(course_id);

CREATE TABLE IF NOT EXISTS appointment_slots (
    id bigserial PRIMARY KEY,
    group_id bigint NOT NULL REFERENCES appointment_groups(id) ON DELETE CASCADE,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    participants_limit int,
    created_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_appt_slot_group ON appointment_slots(group_id);
CREATE INDEX IF NOT EXISTS idx_appt_slot_start ON appointment_slots(start_at);

CREATE TABLE IF NOT EXISTS appointment_reservations (
    id bigserial PRIMARY KEY,
    slot_id bigint NOT NULL REFERENCES appointment_slots(id) ON DELETE CASCADE,
    group_id bigint NOT NULL REFERENCES appointment_groups(id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    reserved_at timestamptz,
    canceled_at timestamptz,
    workflow_state text NOT NULL DEFAULT 'reserved'
);

CREATE INDEX IF NOT EXISTS idx_appt_res_slot ON appointment_reservations(slot_id);
CREATE INDEX IF NOT EXISTS idx_appt_res_user ON appointment_reservations(user_id);
CREATE INDEX IF NOT EXISTS idx_appt_res_group ON appointment_reservations(group_id);

-- Prevent the same user from holding two active reservations on the same slot.
CREATE UNIQUE INDEX IF NOT EXISTS idx_appt_res_unique_active
    ON appointment_reservations(slot_id, user_id)
    WHERE workflow_state = 'reserved';
