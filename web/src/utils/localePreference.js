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

/**
 * Applies the signed-in user's locale (see pickLocale) to i18n.
 *
 * It only reads the device choice; it never writes the profile locale into
 * it. The device key records an explicit choice made on this device, and
 * copying a profile locale there would outlive the session: after an admin
 * masquerades as a Spanish-speaking student and stops, or a Spanish-speaking
 * user signs out of a shared computer, the next user without a profile
 * locale would be left in Spanish.
 */
export function applyUserLocale(user, { i18n, deviceLocale } = {}) {
  if (!user || !i18n) return null;
  const locale = pickLocale({
    userLocale: user.locale,
    deviceLocale,
    accountLocale: user?.account?.default_locale || user?.account_default_locale,
  });
  if (locale && locale !== i18n.language) {
    i18n.changeLanguage(locale);
  }
  return locale;
}
