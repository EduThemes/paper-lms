DROP INDEX IF EXISTS idx_appt_res_unique_active_group;
DROP INDEX IF EXISTS idx_appt_res_participant_group;
ALTER TABLE appointment_reservations DROP COLUMN IF EXISTS comments;
ALTER TABLE appointment_reservations DROP COLUMN IF EXISTS participant_group_id;

DROP INDEX IF EXISTS idx_appt_group_category;
ALTER TABLE appointment_groups DROP CONSTRAINT IF EXISTS chk_appt_group_participant_type;
ALTER TABLE appointment_groups DROP COLUMN IF EXISTS group_category_id;
ALTER TABLE appointment_groups DROP COLUMN IF EXISTS participant_type;

DROP TABLE IF EXISTS appointment_group_contexts;
