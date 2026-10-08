import React from 'react';
import { RESTRICTION_KEYS, parseRestrictions } from '../../lib/contentMigration';

// RestrictionCheckboxes edits a blueprint restrictions object
// ({content, points, due_dates, availability_dates}).
export default function RestrictionCheckboxes({ value, onChange, idPrefix = 'restr', disabled = false, compact = false }) {
  const r = parseRestrictions(value);
  return (
    <div className={compact ? 'flex flex-wrap gap-3' : 'grid grid-cols-2 gap-2'}>
      {RESTRICTION_KEYS.map(({ key, label }) => {
        const id = `${idPrefix}-${key}`;
        return (
          <label key={key} htmlFor={id} className="inline-flex items-center gap-2 text-sm text-text-secondary">
            <input
              id={id}
              type="checkbox"
              disabled={disabled}
              checked={r[key]}
              onChange={(e) => onChange({ ...r, [key]: e.target.checked })}
              className="h-4 w-4 rounded border-border-strong text-brand-600 focus:ring-brand-500"
            />
            {label}
          </label>
        );
      })}
    </div>
  );
}
