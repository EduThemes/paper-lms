import React from 'react';
import { Target } from 'lucide-react';

// NextToBeatBanner renders the "earn N more {currency} to pass X"
// callout above the leaderboard table. Rendered only when the server
// returns a `next_to_beat` block (relative-window responses for
// students outside top-N, per the W3-C plan).
//
// Copy softens when the gap is "large" relative to a tunable
// LARGE_GAP_THRESHOLD — we don't want a 4-XP-from-the-bottom learner
// to see a daunting "earn 6800 more XP to pass …" callout. Per the
// behavioral research (03-claude-behavioral.md:281–286), upward
// social comparison should feel proximate, not impossible.
//
// Visual style follows the W2-A CurrencyPills / LeaderboardTable
// vocabulary: rounded surface card, lucide Target icon, brand-tinted
// accent on the actionable text.
const LARGE_GAP_THRESHOLD = 500;

export default function NextToBeatBanner({ nextToBeat }) {
  if (!nextToBeat) return null;

  const gap = Number(nextToBeat.gap) || 0;
  const name = nextToBeat.name || 'the next learner';
  const currencyLabel = nextToBeat.currency_label || 'points';
  const isLarge = gap > LARGE_GAP_THRESHOLD;

  // Two copy variants. The "large gap" version frames effort, not
  // distance; the "close" version names the specific number to chase.
  let message;
  if (isLarge) {
    message = (
      <>
        Keep going — <strong className="text-brand-700">{name}</strong> is up
        ahead. Every {currencyLabel} earned closes the gap.
      </>
    );
  } else {
    message = (
      <>
        Earn{' '}
        <strong className="tabular-nums text-brand-700">{gap.toLocaleString()}</strong>{' '}
        more {currencyLabel} to pass{' '}
        <strong className="text-brand-700">{name}</strong>.
      </>
    );
  }

  return (
    <div
      className="flex items-start gap-3 rounded-lg border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-text-primary"
      role="status"
      aria-live="polite"
    >
      <Target className="w-5 h-5 mt-0.5 flex-shrink-0 text-brand-600" aria-hidden="true" />
      <p className="leading-snug">{message}</p>
    </div>
  );
}
