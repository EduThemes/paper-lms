import React from 'react';
import { CurrencyIcon } from './currencyIcon';

// BadgeIcon renders a stamped-paper-style badge medallion.
//
// Two display modes:
//   - image_url provided: render as an <img> in a circular frame
//     (admin-uploaded artwork takes precedence)
//   - otherwise: render the lucide-named glyph via CurrencyIcon (which
//     handles unknown names with the Sparkles fallback) on a circular
//     background tinted by the badge's color
//
// Per the design memory: paper / eink / Canvas-LMS-restrained.  We do
// NOT use bright gradient orbs (Duolingo-style) — instead a flat
// circular chip with a thin solid border keeps the badge legible on a
// grayscale e-ink display and consistent with the rest of the chrome.
export function BadgeIcon({ badge, size = 'md' }) {
  const dim = size === 'lg' ? 'w-16 h-16' : size === 'sm' ? 'w-8 h-8' : 'w-12 h-12';
  const iconClass = size === 'lg' ? 'w-8 h-8' : size === 'sm' ? 'w-4 h-4' : 'w-6 h-6';
  const tint = badge?.color || '#A855F7';

  if (badge?.image_url) {
    return (
      <span
        className={`inline-flex items-center justify-center ${dim} rounded-full border-2 border-surface-raised overflow-hidden bg-surface-1`}
        style={{ borderColor: tint }}
      >
        <img src={badge.image_url} alt="" className="w-full h-full object-cover" />
      </span>
    );
  }
  return (
    <span
      className={`inline-flex items-center justify-center ${dim} rounded-full border-2 bg-surface-1`}
      style={{ borderColor: tint }}
    >
      <CurrencyIcon icon={badge?.icon} color={tint} className={iconClass} />
    </span>
  );
}
