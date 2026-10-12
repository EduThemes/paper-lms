import { useEffect } from 'react';
import { useOptionalAuth } from '../../contexts/AuthContext';
import { useMyBrandConfig } from '../../services/adminAccountsQueries';
import { adminUrls } from '../../services/api_admin_accounts';
import { buildBrandCss } from '../../utils/adminAccounts';
import { setBrandLogoUrl } from './brandStore';

const STYLE_ID = 'paper-brand-theme';

// BrandThemeLoader applies the signed-in user's account theme (G118):
// allow-listed colour variables become CSS custom properties for the light
// theme, and the uploaded logo replaces the Paper mark. Renders nothing.
export default function BrandThemeLoader() {
  const auth = useOptionalAuth();
  const signedIn = Boolean(auth?.user);
  const query = useMyBrandConfig({ enabled: signedIn });
  const config = signedIn ? query.data : null;

  useEffect(() => {
    if (typeof document === 'undefined') return;
    let el = document.getElementById(STYLE_ID);
    const css = config ? buildBrandCss(config.variables || {}) : '';
    if (!css) {
      if (el) el.remove();
    } else {
      if (!el) {
        el = document.createElement('style');
        el.id = STYLE_ID;
        document.head.appendChild(el);
      }
      el.textContent = css;
    }
    setBrandLogoUrl(config?.has_logo ? adminUrls.brandLogo(config.version) : null);
  }, [config]);

  return null;
}
