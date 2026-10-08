-- 000166_course_hide_final_grades.up.sql
--
-- Learner dashboard lane (G147): Canvas's "hide totals in student grades
-- summary" course setting.

ALTER TABLE courses ADD COLUMN IF NOT EXISTS hide_final_grades boolean NOT NULL DEFAULT false;
