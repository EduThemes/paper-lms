/**
 * Helpers for <input type="datetime-local">, whose value is a wall-clock
 * time in the browser's local zone ("YYYY-MM-DDTHH:mm", no offset).
 *
 * B208: putting a server UTC ISO string in the input via .slice(0, 16)
 * showed the UTC wall time as if it were local, and saving it with
 * new Date(value).toISOString() then shifted the instant by the UTC
 * offset on every edit.
 */

const pad = (n) => String(n).padStart(2, '0');

/** ISO instant (e.g. "2026-03-01T03:59:00Z") -> local "YYYY-MM-DDTHH:mm". */
export function isoToDatetimeLocal(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** Local "YYYY-MM-DDTHH:mm" -> ISO instant (UTC), or null when empty. */
export function datetimeLocalToISO(value) {
  if (!value) return null;
  const d = new Date(value); // parsed as local time
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}
