-- 000076_users_locale_explicit_only.up.sql
--
-- users.locale had DEFAULT 'en', so every user carried a "saved" profile
-- locale whether or not they ever chose one. Since B210 the client applies
-- the profile locale first, which made the account's default_locale and
-- any earlier per-device choice unreachable: a Spanish-default tenant got
-- English for everyone.
--
-- An empty locale now means "no explicit choice" (fall back to the device
-- choice, then the account default). Rows still holding the old default
-- are reset; 'en' can't be told apart from the default, so a user who had
-- explicitly picked English in a non-English tenant picks it again once.

ALTER TABLE users ALTER COLUMN locale SET DEFAULT '';

UPDATE users SET locale = '' WHERE locale IS NULL OR locale = 'en';
