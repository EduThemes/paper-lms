-- 000043_enrollment_pseudonym.up.sql
--
-- Phase 6 Wave 3 Sprint W3-B — per-enrollment whimsical pseudonyms.
--
-- Adds two columns to enrollments that drive the leaderboard's
-- identity layer (per the SYNTHESIS / behavioral research and the
-- user's prior conversation requirements: a K-5 student sees
-- "Wandering Otter"-style names for peers, never raw legal names).
--
-- Why on enrollments and not on users:
--   * Anonymity is per-course, not global. A learner can be "Wandering
--     Otter" in one teacher's class and "Steady Beacon" in another's.
--   * The teacher controls the per-course defaults (W3-B render policy),
--     so the identity is naturally scoped to the enrollment record.
--   * Adding to users would also force pseudonym uniqueness across the
--     whole tenant, which is much harder than uniqueness within a course.
--
-- Why a UNIQUE constraint per (course_id, pseudonym_pool_code,
-- pseudonym_name):
--   * Two learners can share a pseudonym in different courses (fine).
--   * Within one course one pool can't produce duplicates (the
--     generator re-rolls deterministically on conflict).
--   * Across pools, the same name is unlikely (different word
--     vocabularies); the constraint pairs (pool, name) so a hypothetical
--     adjective-collision between pools doesn't trip.
--
-- Why pseudonym_name is nullable:
--   * Lazy assignment: populated on the first leaderboard read of an
--     enrollment, not at INSERT time. Avoids backfilling tens of
--     thousands of existing enrollments at migration time, and keeps
--     the cost on the read path where it's already cheap.
--
-- Why no `default:` GORM tag on pseudonym_pool_code in the Go model
-- (separate change in this PR): the W2-A/W2-B bool-default class of bug
-- is about GORM eliding columns at db.Save / Create. Even though this
-- is TEXT not bool, the same elision behavior applies if the default
-- ever changes — keep the SQL DEFAULT load-bearing and the Go side
-- explicit on every INSERT.

BEGIN;

ALTER TABLE enrollments
    ADD COLUMN IF NOT EXISTS pseudonym_pool_code TEXT NOT NULL DEFAULT 'animals_v1',
    ADD COLUMN IF NOT EXISTS pseudonym_name      TEXT;

-- One pseudonym per (course, pool, name). Partial: only enforce when
-- a name is actually set (null rows allowed, lazy-fill semantic).
CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollments_pseudonym_unique_per_course
    ON enrollments (course_id, pseudonym_pool_code, pseudonym_name)
    WHERE pseudonym_name IS NOT NULL;

COMMIT;
