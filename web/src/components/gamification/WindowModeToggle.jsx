import React from 'react';
import { CalendarDays, Clock } from 'lucide-react';

// WindowModeToggle is the segmented "This week" / "Last week" switch
// above the leaderboard table. Mirrors the chrome-toggle vocabulary
// used elsewhere in the app (border-surface-raised, bg-surface-1,
// active = brand fill).
//
// v1 ships two options:
//   * 'current' → offset_weeks=0, live compute
//   * 'last'    → offset_weeks=1, snapshot read
//
// HigherEd / Corp tenants will eventually get an 'all_time' option
// (a third segment). v1 holds the surface tight to weekly-only per
// the W3-C plan; the toggle's two-option shape leaves room.
export default function WindowModeToggle({ mode, onChange, disabled = false }) {
  const options = [
    { value: 'current', label: 'This week', icon: Clock },
    { value: 'last', label: 'Last week', icon: CalendarDays },
  ];

  return (
    <div
      role="tablist"
      aria-label="Leaderboard window"
      className="inline-flex items-center gap-1 rounded-full border border-surface-raised bg-surface-1 p-1"
    >
      {options.map((opt) => {
        const Icon = opt.icon;
        const active = mode === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={disabled}
            onClick={() => onChange(opt.value)}
            className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-brand-400/60 ${
              active
                ? 'bg-brand-600 text-white shadow-sm'
                : 'text-text-secondary hover:bg-surface-2 hover:text-text-primary'
            } ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
          >
            <Icon className="w-4 h-4" aria-hidden="true" />
            <span>{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
