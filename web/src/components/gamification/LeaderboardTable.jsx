import React from 'react';
import { Trophy, Medal } from 'lucide-react';

function formatScore(n) {
  if (typeof n !== 'number') return '0';
  if (n >= 100_000) return `${Math.floor(n / 1000)}k`;
  if (n >= 10_000) return `${(n / 1000).toFixed(1)}k`;
  return n.toLocaleString();
}

// rankIcon picks a medal for the top three rows, falls back to a plain
// numeric rank for the rest. Trophy chrome is reserved for the header.
function rankIcon(rank) {
  if (rank === 1) return <Medal className="w-4 h-4 text-yellow-500" aria-label="1st" />;
  if (rank === 2) return <Medal className="w-4 h-4 text-gray-400" aria-label="2nd" />;
  if (rank === 3) return <Medal className="w-4 h-4 text-amber-700" aria-label="3rd" />;
  return null;
}

// LeaderboardTable renders ranked rows for the gamification leaderboard.
// W3-A wires this for the admin / teacher full view; W3-B reuses it for
// the student-facing pseudonymized view (no client-side decisions about
// what to render — the server decides); W3-C extends it with mode='relative'
// and a "next to beat" callout above the rows.
//
// Props
//   rows           — [{ rank, user_id, name, lifetime_earned }] from the API.
//   currencyLabel  — display label of the ranking currency (e.g. "XP").
//   viewerId       — current user id (highlighted in-row).
//   loading        — render skeleton when true.
//   emptyHint      — copy shown when rows is empty (e.g. "No learners yet").
export default function LeaderboardTable({
  rows = [],
  currencyLabel = 'XP',
  viewerId = null,
  loading = false,
  emptyHint = 'No ranked learners yet.',
}) {
  if (loading) {
    return (
      <div className="rounded-lg border border-surface-raised bg-surface-1 p-6 text-sm text-text-secondary">
        Loading leaderboard…
      </div>
    );
  }
  if (!rows.length) {
    return (
      <div className="rounded-lg border border-surface-raised bg-surface-1 p-6 text-sm text-text-secondary">
        {emptyHint}
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-surface-raised bg-surface-0 overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-surface-raised bg-surface-1">
        <Trophy className="w-4 h-4 text-text-secondary" aria-hidden="true" />
        <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wider">
          Leaderboard · {currencyLabel}
        </h2>
      </div>
      <ul role="list" className="divide-y divide-surface-raised">
        {rows.map((row) => {
          const isViewer = viewerId && row.user_id === viewerId;
          return (
            <li
              key={`${row.user_id}-${row.rank}`}
              className={`flex items-center gap-4 px-4 py-3 text-sm ${
                isViewer ? 'bg-brand-50 font-semibold text-brand-700' : 'text-text-primary'
              }`}
            >
              <span className="w-10 flex items-center justify-end gap-1 text-text-secondary tabular-nums">
                {rankIcon(row.rank)}
                <span>{row.rank}</span>
              </span>
              <span className="flex-1 truncate">{row.name || `User #${row.user_id}`}</span>
              <span className="tabular-nums text-text-primary">
                {formatScore(row.lifetime_earned)}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
