-- Reverse of 000044. Restores the original W2-D shape: NOT NULL DEFAULT ''.
-- Drops the CHECK constraint. Any rows that became NULL after the relax
-- are coerced back to '' on the way down (idempotent).

BEGIN;

UPDATE gamification_badges SET description    = '' WHERE description    IS NULL;
UPDATE gamification_badges SET icon           = '' WHERE icon           IS NULL;
UPDATE gamification_badges SET image_url      = '' WHERE image_url      IS NULL;
UPDATE gamification_badges SET color          = '' WHERE color          IS NULL;
UPDATE gamification_badges SET audience_level = '' WHERE audience_level IS NULL;

ALTER TABLE gamification_badges
    ALTER COLUMN description    SET NOT NULL,
    ALTER COLUMN description    SET DEFAULT '';
ALTER TABLE gamification_badges
    ALTER COLUMN icon           SET NOT NULL,
    ALTER COLUMN icon           SET DEFAULT '';
ALTER TABLE gamification_badges
    ALTER COLUMN image_url      SET NOT NULL,
    ALTER COLUMN image_url      SET DEFAULT '';
ALTER TABLE gamification_badges
    ALTER COLUMN color          SET NOT NULL,
    ALTER COLUMN color          SET DEFAULT '';
ALTER TABLE gamification_badges
    ALTER COLUMN audience_level SET NOT NULL,
    ALTER COLUMN audience_level SET DEFAULT '';

ALTER TABLE gamification_badges
    DROP CONSTRAINT IF EXISTS chk_gam_badges_scope_type;

COMMIT;
