import { describe, test, expect, vi, afterEach } from 'vitest';
import { applyUserLocale, pickLocale } from './localePreference';

const fakeI18n = (language) => {
  const i18n = { language, changeLanguage: vi.fn((l) => { i18n.language = l; }) };
  return i18n;
};

describe('applyUserLocale (B210)', () => {
  afterEach(() => vi.unstubAllGlobals());

  test('applies the profile locale without recording it as the device choice', () => {
    const setItem = vi.fn();
    vi.stubGlobal('localStorage', { getItem: () => null, setItem, removeItem: vi.fn() });
    const i18n = fakeI18n('en');
    expect(applyUserLocale({ id: 1, locale: 'es' }, { i18n, deviceLocale: null })).toBe('es');
    expect(i18n.changeLanguage).toHaveBeenCalledWith('es');
    expect(setItem).not.toHaveBeenCalled();
  });

  test('a masquerade (or shared device) does not leave the next user in the previous locale', () => {
    const i18n = fakeI18n('en');
    // Admin masquerades as a Spanish-speaking student...
    applyUserLocale({ id: 2, locale: 'es' }, { i18n, deviceLocale: null });
    expect(i18n.language).toBe('es');
    // ...then stops: the admin has no profile locale and the device has
    // no explicit choice, so the account default applies again.
    applyUserLocale({ id: 1, locale: '', account: { default_locale: 'en' } }, { i18n, deviceLocale: null });
    expect(i18n.language).toBe('en');
  });

  test('does nothing without a user', () => {
    const i18n = fakeI18n('en');
    expect(applyUserLocale(null, { i18n })).toBeNull();
    expect(i18n.changeLanguage).not.toHaveBeenCalled();
  });
});

describe('pickLocale (B210)', () => {
  test("the user's saved locale wins on any device", () => {
    expect(pickLocale({ userLocale: 'es', deviceLocale: null, accountLocale: 'en' })).toBe('es');
    expect(pickLocale({ userLocale: 'es', deviceLocale: 'en', accountLocale: 'en' })).toBe('es');
  });

  test('falls back to the device choice, then the account default', () => {
    expect(pickLocale({ userLocale: '', deviceLocale: 'es', accountLocale: 'en' })).toBe('es');
    expect(pickLocale({ accountLocale: 'es' })).toBe('es');
  });

  test('regional variants map to a supported language; unsupported ones are ignored', () => {
    expect(pickLocale({ userLocale: 'es-MX' })).toBe('es');
    expect(pickLocale({ userLocale: 'fr', accountLocale: 'en' })).toBe('en');
    expect(pickLocale({})).toBeNull();
  });
});
