// Helpers for <input type="datetime-local">, whose value is a wall-clock
// time in the browser's local zone ("YYYY-MM-DDTHH:mm", no offset).
//
// Slicing an ISO/UTC string ("2026-08-15T00:00:00Z".slice(0, 16)) and
// feeding it to the input reinterprets the UTC wall clock as local time,
// so every save shifted the stored value by the browser's UTC offset
// (B176). Convert through Date in both directions instead.

const pad = (n) => String(n).padStart(2, '0');

// isoToDateTimeLocal formats an ISO timestamp as the local wall-clock
// value a datetime-local input expects. Empty/invalid input → ''.
export function isoToDateTimeLocal(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// dateTimeLocalToISO converts a datetime-local value (local wall clock)
// back to a UTC ISO string. Empty/invalid input → ''.
export function dateTimeLocalToISO(local) {
  if (!local) return '';
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return '';
  return d.toISOString();
}

// Aliases used by the learner-side forms (B208). Same conversions, but
// empty/invalid input yields null so callers can send "clear this date".
export function isoToDatetimeLocal(iso) {
  return isoToDateTimeLocal(iso);
}

export function datetimeLocalToISO(value) {
  return dateTimeLocalToISO(value) || null;
}
