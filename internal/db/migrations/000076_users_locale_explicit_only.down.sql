-- 000076_users_locale_explicit_only.down.sql

UPDATE users SET locale = 'en' WHERE locale IS NULL OR locale = '';

ALTER TABLE users ALTER COLUMN locale SET DEFAULT 'en';
