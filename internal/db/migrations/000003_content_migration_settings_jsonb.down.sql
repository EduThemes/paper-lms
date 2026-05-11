-- Revert content_migrations.migration_settings back to text.
ALTER TABLE content_migrations ALTER COLUMN migration_settings DROP DEFAULT;
ALTER TABLE content_migrations ALTER COLUMN migration_settings TYPE text USING migration_settings::text;
ALTER TABLE content_migrations ALTER COLUMN migration_settings SET DEFAULT '{}';
