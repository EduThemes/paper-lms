-- Convert content_migrations.migration_settings from text/varchar to jsonb.
-- Preserves any pre-existing content: empty/null becomes '{}', JSON object
-- payloads parse natively, and arbitrary legacy strings are wrapped in
-- {"legacy_string": "..."} so they remain accessible to the typed model.
ALTER TABLE content_migrations ALTER COLUMN migration_settings TYPE jsonb USING
  CASE
    WHEN migration_settings IS NULL OR migration_settings = '' THEN '{}'::jsonb
    WHEN migration_settings ~ '^\s*\{' THEN migration_settings::jsonb
    ELSE jsonb_build_object('legacy_string', migration_settings)
  END;

ALTER TABLE content_migrations ALTER COLUMN migration_settings SET DEFAULT '{}'::jsonb;
