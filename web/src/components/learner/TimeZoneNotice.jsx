import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { Globe, X } from 'lucide-react';
import { browserTimeZone, getUserTimeZone, timeZoneMismatch } from '../../lib/userTimeZone';

const DISMISS_KEY = 'paperlms_tz_notice_dismissed';

// TimeZoneNotice tells the user, like Canvas does, when dates are shown in
// their profile time zone but the device is in a different one (G157).
// Dismissal is remembered per zone pair for this browser.
export default function TimeZoneNotice({ className = '' }) {
  const pair = `${getUserTimeZone()}|${browserTimeZone()}`;
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(DISMISS_KEY) === pair;
    } catch {
      return false;
    }
  });
  if (dismissed || !timeZoneMismatch()) return null;
  const dismiss = () => {
    try {
      localStorage.setItem(DISMISS_KEY, pair);
    } catch {
      /* ignore */
    }
    setDismissed(true);
  };
  return (
    <div
      role="note"
      className={`flex items-start gap-2 rounded-md border border-border-default bg-surface-1 px-3 py-2 text-sm text-text-secondary ${className}`}
    >
      <Globe className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <p className="flex-1">
        Dates are shown in your profile time zone, <strong>{getUserTimeZone().replace(/_/g, ' ')}</strong>.
        This device is set to {browserTimeZone().replace(/_/g, ' ')}.{' '}
        <Link to="/profile/settings" className="text-brand-600 underline">Change time zone</Link>
      </p>
      <button type="button" onClick={dismiss} className="rounded p-0.5 hover:bg-surface-2" aria-label="Dismiss time zone notice">
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
    </div>
  );
}
