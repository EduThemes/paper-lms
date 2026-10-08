-- 000097_assignments_omit_from_final_grade.down.sql
ALTER TABLE assignments DROP COLUMN IF EXISTS omit_from_final_grade;
