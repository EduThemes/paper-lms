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

  test('renders both locale options as a radio group', () => {
    render(<LanguagePreferencesPage />);
    const radios = screen.getAllByRole('radio');
    expect(radios).toHaveLength(2);
    // English is the default in tests — first radio is checked.
    expect(radios[0]).toHaveAttribute('aria-checked', 'true');
    expect(radios[1]).toHaveAttribute('aria-checked', 'false');
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
