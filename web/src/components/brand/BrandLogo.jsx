import React, { useSyncExternalStore } from 'react';
import { getBrandLogoUrl, subscribeBrandLogo } from './brandStore';

// Single source of truth for the Paper LMS brand mark.
// Asset lives at /brand/paper-logo.svg (web/public/brand/), referenced by
// URL so a future rebrand is a one-file replacement with no rebuild.
// When the signed-in account has a theme logo (G118, BrandThemeLoader),
// that logo is shown instead.
export default function BrandLogo({ size = 32, className = '', alt = 'Paper LMS' }) {
  const accountLogo = useSyncExternalStore(subscribeBrandLogo, getBrandLogoUrl, () => null);
  return (
    <img
      src={accountLogo || '/brand/paper-logo.svg'}
      width={size}
      height={size}
      alt={alt}
      className={`${accountLogo ? 'object-contain' : ''} ${className}`.trim()}
      draggable={false}
    />
  );
}
