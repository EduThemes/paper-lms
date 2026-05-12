-- Reverse of 000035. Drops the FERPA tag lookup table and the two accounts
-- columns. tenant_mode data is LOST on rollback (operators with K-12
-- deployments must restore from backup or re-migrate after a forward
-- re-apply).

BEGIN;

DROP TABLE IF EXISTS gamification_ferpa_field_tags;

ALTER TABLE accounts DROP COLUMN IF EXISTS coppa_strict;
ALTER TABLE accounts DROP COLUMN IF EXISTS tenant_mode;

COMMIT;
