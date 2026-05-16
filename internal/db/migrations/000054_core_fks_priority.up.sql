-- 000054_core_fks_priority.up.sql
--
-- Phase 13 Item 13.2 (priority subset).
--
-- The audit's "no FK enforcement on the Canvas-inherited core" finding
-- listed ~98 tables wired by integer-by-convention. Closing all of
-- them is a separate plan-estimated 1-sprint piece; this migration
-- lands the priority subset — the rows the audit cited as the
-- highest-leverage targets (grade-bearing + identity-bearing).
--
-- Strategy:
--   * `ON DELETE RESTRICT` on grade-bearing rows (submissions, etc.) —
--     audit memo says graded work must never silently vanish.
--   * `ON DELETE CASCADE` on enrollment-style rows where the parent
--     row leaving means the relationship leaves.
--   * `ON DELETE SET NULL` on audit/grade-change logs where the row
--     outlives the user it references.
--
-- `NOT VALID` + immediate `VALIDATE CONSTRAINT` is used so the lock
-- pattern matches CONCURRENTLY-style deploys. On a clean dev DB this
-- is a single transaction; on prod it'd be split (NOT VALID first,
-- VALIDATE in a follow-up).
--
-- Pre-clean: NULL out any references that point at non-existent rows
-- so the constraint creation doesn't trip.

BEGIN;

-- 1. enrollments
UPDATE enrollments SET user_id = NULL
 WHERE user_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM users WHERE id = enrollments.user_id);
UPDATE enrollments SET course_id = NULL
 WHERE course_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM courses WHERE id = enrollments.course_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'enrollments_user_id_fkey') THEN
        ALTER TABLE enrollments
            ADD CONSTRAINT enrollments_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE NOT VALID;
        ALTER TABLE enrollments VALIDATE CONSTRAINT enrollments_user_id_fkey;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'enrollments_course_id_fkey') THEN
        ALTER TABLE enrollments
            ADD CONSTRAINT enrollments_course_id_fkey
            FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE NOT VALID;
        ALTER TABLE enrollments VALIDATE CONSTRAINT enrollments_course_id_fkey;
    END IF;
END $$;

-- 2. submissions (grade-bearing → RESTRICT on user / assignment delete)
UPDATE submissions SET user_id = NULL
 WHERE user_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM users WHERE id = submissions.user_id);
UPDATE submissions SET assignment_id = NULL
 WHERE assignment_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM assignments WHERE id = submissions.assignment_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'submissions_user_id_fkey') THEN
        ALTER TABLE submissions
            ADD CONSTRAINT submissions_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT NOT VALID;
        ALTER TABLE submissions VALIDATE CONSTRAINT submissions_user_id_fkey;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'submissions_assignment_id_fkey') THEN
        ALTER TABLE submissions
            ADD CONSTRAINT submissions_assignment_id_fkey
            FOREIGN KEY (assignment_id) REFERENCES assignments(id) ON DELETE RESTRICT NOT VALID;
        ALTER TABLE submissions VALIDATE CONSTRAINT submissions_assignment_id_fkey;
    END IF;
END $$;

-- 3. assignments
UPDATE assignments SET course_id = NULL
 WHERE course_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM courses WHERE id = assignments.course_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'assignments_course_id_fkey') THEN
        ALTER TABLE assignments
            ADD CONSTRAINT assignments_course_id_fkey
            FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE NOT VALID;
        ALTER TABLE assignments VALIDATE CONSTRAINT assignments_course_id_fkey;
    END IF;
END $$;

-- 4. submission_comments
UPDATE submission_comments SET submission_id = NULL
 WHERE submission_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM submissions WHERE id = submission_comments.submission_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'submission_comments_submission_id_fkey') THEN
        ALTER TABLE submission_comments
            ADD CONSTRAINT submission_comments_submission_id_fkey
            FOREIGN KEY (submission_id) REFERENCES submissions(id) ON DELETE CASCADE NOT VALID;
        ALTER TABLE submission_comments VALIDATE CONSTRAINT submission_comments_submission_id_fkey;
    END IF;
END $$;

-- 5. audit_logs (rows OUTLIVE the user; SET NULL on the optional FK columns)
DO $$ BEGIN
    -- Only attempt the constraint if the column exists and is nullable.
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_logs' AND column_name = 'user_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'audit_logs_user_id_fkey'
    ) THEN
        EXECUTE 'UPDATE audit_logs SET user_id = NULL
                  WHERE user_id IS NOT NULL
                    AND NOT EXISTS (SELECT 1 FROM users WHERE id = audit_logs.user_id)';
        ALTER TABLE audit_logs
            ADD CONSTRAINT audit_logs_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL NOT VALID;
        ALTER TABLE audit_logs VALIDATE CONSTRAINT audit_logs_user_id_fkey;
    END IF;
END $$;

-- 6. grade_change_logs (same posture as audit_logs — durable trail)
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'grade_change_logs' AND column_name = 'student_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'grade_change_logs_student_id_fkey'
    ) THEN
        EXECUTE 'UPDATE grade_change_logs SET student_id = NULL
                  WHERE student_id IS NOT NULL
                    AND NOT EXISTS (SELECT 1 FROM users WHERE id = grade_change_logs.student_id)';
        ALTER TABLE grade_change_logs
            ADD CONSTRAINT grade_change_logs_student_id_fkey
            FOREIGN KEY (student_id) REFERENCES users(id) ON DELETE SET NULL NOT VALID;
        ALTER TABLE grade_change_logs VALIDATE CONSTRAINT grade_change_logs_student_id_fkey;
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'grade_change_logs' AND column_name = 'grader_id'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'grade_change_logs_grader_id_fkey'
    ) THEN
        EXECUTE 'UPDATE grade_change_logs SET grader_id = NULL
                  WHERE grader_id IS NOT NULL
                    AND NOT EXISTS (SELECT 1 FROM users WHERE id = grade_change_logs.grader_id)';
        ALTER TABLE grade_change_logs
            ADD CONSTRAINT grade_change_logs_grader_id_fkey
            FOREIGN KEY (grader_id) REFERENCES users(id) ON DELETE SET NULL NOT VALID;
        ALTER TABLE grade_change_logs VALIDATE CONSTRAINT grade_change_logs_grader_id_fkey;
    END IF;
END $$;

COMMIT;
