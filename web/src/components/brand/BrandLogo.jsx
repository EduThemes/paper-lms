import React from 'react';

// Single source of truth for the Paper LMS brand mark.
// Asset lives at /brand/paper-logo.svg (web/public/brand/), referenced by
// URL so a future rebrand is a one-file replacement with no rebuild.
export default function BrandLogo({ size = 32, className = '', alt = 'Paper LMS' }) {
  return (
    <img
      src="/brand/paper-logo.svg"
      width={size}
      height={size}
      alt={alt}
      className={className}
      draggable={false}
    />
  );
}
