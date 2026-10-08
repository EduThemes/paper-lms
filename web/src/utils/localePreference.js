export const SUPPORTED_LOCALES = ['en', 'es'];

const normalize = (locale) => {
  if (!locale || typeof locale !== 'string') return null;
  const lower = locale.trim();
  if (SUPPORTED_LOCALES.includes(lower)) return lower;
  const base = lower.split(/[-_]/)[0].toLowerCase();
  return SUPPORTED_LOCALES.includes(base) ? base : null;
};

/**
 * Picks the UI locale, highest priority first (Canvas order):
 *   1. the user's saved profile locale (users.locale) — follows them to
 *      every device (B210: it used to be saved but never applied),
 *   2. an explicit choice made on this device (localStorage),
 *   3. the account's default_locale.
 * Returns null when nothing applies (keep i18next's detected language).
 */
export function pickLocale({ userLocale, deviceLocale, accountLocale } = {}) {
  return normalize(userLocale) || normalize(deviceLocale) || normalize(accountLocale);
}
