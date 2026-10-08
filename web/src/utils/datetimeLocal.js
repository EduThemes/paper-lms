// Helpers for <input type="datetime-local">, whose value is a wall-clock
// time with no offset ("YYYY-MM-DDTHH:mm").
//
// Slicing an ISO/UTC string ("2026-08-15T00:00:00Z".slice(0, 16)) and
// feeding it to the input reinterprets the UTC wall clock as local time,
// so every save shifted the stored value by the UTC offset (B176). Convert
// through the instant in both directions instead.
//
// The wall clock is read in the user's profile time zone (G157), falling
// back to the browser's zone when none is set — see lib/userTimeZone.js.

import { isoToZonedWallClock, zonedWallClockToISO } from '../lib/userTimeZone';

// isoToDateTimeLocal formats an ISO timestamp as the wall-clock value a
// datetime-local input expects. Empty/invalid input → ''.
export function isoToDateTimeLocal(iso) {
  return isoToZonedWallClock(iso);
}

// dateTimeLocalToISO converts a datetime-local value (user-zone wall clock)
// back to a UTC ISO string. Empty/invalid input → ''.
export function dateTimeLocalToISO(local) {
  return zonedWallClockToISO(local);
}

// Aliases used by the learner-side forms (B208). Same conversions, but
// empty/invalid input yields null so callers can send "clear this date".
export function isoToDatetimeLocal(iso) {
  return isoToDateTimeLocal(iso);
}

export function datetimeLocalToISO(value) {
  return dateTimeLocalToISO(value) || null;
}
