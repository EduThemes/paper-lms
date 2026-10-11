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

import { dayKeyInZone, isoToZonedWallClock, zonedWallClockToISO } from '../lib/userTimeZone';

// parseFormDate turns a form value into a Date: a zone-less wall clock
// ("YYYY-MM-DD" or "YYYY-MM-DDTHH:mm[:ss]", as date / datetime-local
// inputs produce) is read in the profile zone; anything else (an ISO
// string with Z or an offset, a Date) is parsed as-is. Invalid → an
// Invalid Date.
export function parseFormDate(value) {
  if (value instanceof Date) return value;
  if (!value) return new Date(NaN);
  const s = String(value).trim();
  if (/^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2})?)?$/.test(s)) {
    return new Date(zonedWallClockToISO(s) || NaN);
  }
  return new Date(s);
}

// ---- <input type="date"> (a calendar day with no zone, "YYYY-MM-DD") ----
//
// new Date('YYYY-MM-DD') is UTC midnight, which is the evening before in
// every US zone: saving it and showing it back with toLocaleDateString
// moves the date a day earlier on each round trip. Read the day in the
// profile zone instead.

// dateInputToStartISO is the instant the day starts (00:00:00) in the
// profile zone. Empty/invalid input → ''.
export function dateInputToStartISO(ymd) {
  return ymd ? zonedWallClockToISO(String(ymd).slice(0, 10)) : '';
}

// dateInputToEndISO is the last second of the day (23:59:59) in the
// profile zone, for inclusive end / close dates. Empty/invalid → ''.
export function dateInputToEndISO(ymd) {
  return ymd ? zonedWallClockToISO(`${String(ymd).slice(0, 10)}T23:59:59`) : '';
}

// isoToDateInput is the profile-zone calendar day of an instant, as a
// date input value. Empty/invalid input → ''.
export function isoToDateInput(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return dayKeyInZone(d);
}

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
