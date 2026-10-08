import React, { useMemo } from 'react';
import { browserTimeZone } from '../../lib/userTimeZone';

// Common zones used when the browser cannot list every IANA zone.
const FALLBACK_ZONES = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver', 'America/Phoenix',
  'America/Los_Angeles', 'America/Anchorage', 'Pacific/Honolulu', 'America/Puerto_Rico',
  'America/Toronto', 'America/Mexico_City', 'America/Bogota', 'America/Sao_Paulo',
  'Europe/London', 'Europe/Madrid', 'Europe/Paris', 'Europe/Berlin', 'Africa/Johannesburg',
  'Asia/Dubai', 'Asia/Kolkata', 'Asia/Singapore', 'Asia/Shanghai', 'Asia/Tokyo',
  'Australia/Sydney', 'Pacific/Auckland',
];

export function listTimeZones() {
  try {
    if (typeof Intl.supportedValuesOf === 'function') {
      const all = Intl.supportedValuesOf('timeZone');
      if (Array.isArray(all) && all.length > 0) return all.includes('UTC') ? all : ['UTC', ...all];
    }
  } catch {
    /* fall through */
  }
  return FALLBACK_ZONES;
}

// TimeZoneSelect is a labelled IANA time zone picker (G157). The stored
// value is always offered even when the browser's list lacks it.
export default function TimeZoneSelect({ id, value, onChange, className = '' }) {
  const zones = useMemo(() => {
    const list = listTimeZones();
    return value && !list.includes(value) ? [value, ...list] : list;
  }, [value]);
  const device = browserTimeZone();
  return (
    <div>
      <select
        id={id}
        value={value || ''}
        onChange={(e) => onChange(e.target.value)}
        className={`w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm ${className}`}
      >
        <option value="" disabled>Select a time zone</option>
        {zones.map((z) => (
          <option key={z} value={z}>{z.replace(/_/g, ' ')}</option>
        ))}
      </select>
      {value !== device && (
        <button type="button" className="mt-1 text-xs text-brand-600 underline" onClick={() => onChange(device)}>
          Use this device&apos;s time zone ({device.replace(/_/g, ' ')})
        </button>
      )}
    </div>
  );
}
