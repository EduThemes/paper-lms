-- 000115_course_storage_quota.up.sql
--
-- Per-course file storage quota (G072), in bytes. NULL means "use the
-- account default" (settings key quotas.default_course_storage_mb).
-- 000026 dropped an unused column of the same name; this re-adds it with
-- real enforcement behind it (POST /courses/:id/files).

ALTER TABLE courses ADD COLUMN IF NOT EXISTS storage_quota bigint;
