import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import LanguagePreferencesPage from '../LanguagePreferencesPage';
import i18n from '../../i18n';

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

// jsdom in this vitest config doesn't ship a working localStorage. Stub
// a Map-backed implementation onto window so the page's persistence
// call is observable.
function installLocalStorageStub() {
  const store = new Map();
  const stub = {
    getItem: (k) => (store.has(k) ? store.get(k) : null),
    setItem: (k, v) => { store.set(k, String(v)); },
    removeItem: (k) => { store.delete(k); },
    clear: () => { store.clear(); },
  };
  Object.defineProperty(window, 'localStorage', {
    value: stub,
    configurable: true,
    writable: true,
  });
  return stub;
}

describe('LanguagePreferencesPage', () => {
  let storage;

  beforeEach(() => {
    storage = installLocalStorageStub();
    i18n.changeLanguage('en');
  });

  test('renders the school default and both locales as a radio group', () => {
    render(<LanguagePreferencesPage />);
    const radios = screen.getAllByRole('radio');
    expect(radios).toHaveLength(3);
    // No saved or device choice: the school default is selected (G158).
    expect(radios[0]).toHaveAttribute('aria-checked', 'true');
    expect(radios[1]).toHaveAttribute('aria-checked', 'false');
    expect(radios[2]).toHaveAttribute('aria-checked', 'false');
  });

  test('choosing the school default forgets the device choice', async () => {
    storage.setItem('paperlms_locale', 'es');
    i18n.changeLanguage('es');
    render(<LanguagePreferencesPage />);
    expect(screen.getByRole('radio', { name: /Español/i })).toHaveAttribute('aria-checked', 'true');

    fireEvent.click(screen.getByRole('radio', { name: /School default/i }));
    await waitFor(() => expect(i18n.language).toBe('en'));
    expect(storage.getItem('paperlms_locale')).toBeNull();
    expect(screen.getByRole('radio', { name: /School default/i })).toHaveAttribute('aria-checked', 'true');
  });

  test('clicking a locale switches i18next and shows the saved indicator', async () => {
    render(<LanguagePreferencesPage />);
    const spanishRadio = screen.getByRole('radio', { name: /Español/i });

    fireEvent.click(spanishRadio);

    await waitFor(() => {
      expect(i18n.language).toBe('es');
    });
    expect(storage.getItem('paperlms_locale')).toBe('es');
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  test('persists choice to localStorage so it outlives a reload', () => {
    render(<LanguagePreferencesPage />);
    const spanishRadio = screen.getByRole('radio', { name: /Español/i });
    fireEvent.click(spanishRadio);
    expect(storage.getItem('paperlms_locale')).toBe('es');
  });
});
