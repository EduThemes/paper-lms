import React from 'react';
import { render, act } from '@testing-library/react';
import BrandThemeLoader from '../brand/BrandThemeLoader';
import BrandLogo from '../brand/BrandLogo';
import { setBrandLogoUrl } from '../brand/brandStore';

// G118 — the account theme is applied as CSS custom properties and the
// account logo replaces the Paper mark. All data is made up.

let user = { id: 1 };
let config = null;

vi.mock('../../contexts/AuthContext', () => ({ useOptionalAuth: () => ({ user }) }));
vi.mock('../../services/adminAccountsQueries', () => ({
  useMyBrandConfig: (opts) => ({ data: opts?.enabled ? config : undefined }),
}));

describe('BrandThemeLoader', () => {
  afterEach(() => {
    document.getElementById('paper-brand-theme')?.remove();
    act(() => setBrandLogoUrl(null));
  });

  it('injects the theme CSS and swaps the logo', () => {
    user = { id: 1 };
    config = { variables: { brand_primary: '#0A7B3E' }, has_logo: true, version: 42 };
    const { getByRole } = render(<><BrandThemeLoader /><BrandLogo alt="Logo" /></>);
    const style = document.getElementById('paper-brand-theme');
    expect(style.textContent).toContain('--color-brand-600: 10 123 62;');
    expect(getByRole('img', { name: 'Logo' }).getAttribute('src')).toContain('/brand_config/logo?v=42');
  });

  it('does nothing when signed out', () => {
    user = null;
    config = { variables: { brand_primary: '#0A7B3E' }, has_logo: true, version: 1 };
    const { getByRole } = render(<><BrandThemeLoader /><BrandLogo alt="Logo" /></>);
    expect(document.getElementById('paper-brand-theme')).toBeNull();
    expect(getByRole('img', { name: 'Logo' }).getAttribute('src')).toBe('/brand/paper-logo.svg');
  });
});
