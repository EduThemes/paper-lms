-- 000097_assignments_omit_from_final_grade.up.sql
--
-- Grading-UI lane (gradebook parity): an assignment can be left out of
-- the course total while still being graded (Canvas
-- omit_from_final_grade). 000027 dropped the column as unused; the grade
-- calculator reads it again.
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS omit_from_final_grade boolean DEFAULT false;
