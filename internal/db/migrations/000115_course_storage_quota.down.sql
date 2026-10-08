-- 000115_course_storage_quota.down.sql

ALTER TABLE courses DROP COLUMN IF EXISTS storage_quota;
