import { describe, test, expect } from 'vitest';
import { pickLocale } from './localePreference';

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
