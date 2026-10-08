// The signed-in user's time zone (G157).
//
// Users pick a time zone in Account Settings (users.time_zone). Canvas
// shows and accepts every date in that zone, not the browser's: a student
// travelling, or a lab machine left on UTC, must still see "due 11:59 PM"
// the way the course meant it.
//
// Two pieces:
//
//  1. Display. installTimeZoneDefaults() makes Date#toLocaleString /
//     toLocaleDateString / toLocaleTimeString default their `timeZone`
//     option to the user's zone. Every page formats dates through these, so
//     one install covers them all; a caller that passes its own timeZone is
//     left alone.
//  2. Entry. zonedWallClockToISO / isoToZonedWallClock convert between a
//     <input type="datetime-local"> value (a wall-clock time with no zone)
//     and a UTC instant, reading the wall clock in the user's zone.

let userTimeZone = null;

export function browserTimeZone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

export function isValidTimeZone(tz) {
  if (!tz || typeof tz !== 'string') return false;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

// setUserTimeZone records the profile zone (null/invalid clears it, which
// falls back to the browser zone).
export function setUserTimeZone(tz) {
  userTimeZone = isValidTimeZone(tz) ? tz : null;
}

// getUserTimeZone is the zone dates are shown and entered in.
export function getUserTimeZone() {
  return userTimeZone || browserTimeZone();
}

// timeZoneMismatch reports whether the profile zone differs from the
// browser's in effect right now (same offset counts as a match, e.g.
// America/Detroit vs America/New_York).
export function timeZoneMismatch(now = new Date()) {
  if (!userTimeZone) return false;
  const b = browserTimeZone();
  if (b === userTimeZone) return false;
  return zoneOffsetMinutes(now, userTimeZone) !== zoneOffsetMinutes(now, b);
}

const dtfCache = new Map();
function partsFormatter(tz) {
  let f = dtfCache.get(tz);
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      hourCycle: 'h23',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
    dtfCache.set(tz, f);
  }
  return f;
}

function wallClockParts(date, tz) {
  const out = {};
  for (const p of partsFormatter(tz).formatToParts(date)) {
    if (p.type !== 'literal') out[p.type] = parseInt(p.value, 10);
  }
  if (out.hour === 24) out.hour = 0;
  return out;
}

// zoneOffsetMinutes is tz's UTC offset at `date` (east positive).
export function zoneOffsetMinutes(date, tz) {
  const p = wallClockParts(date, tz);
  const asUTC = Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second);
  return Math.round((asUTC - Math.floor(date.getTime() / 1000) * 1000) / 60000);
}

const pad = (n) => String(n).padStart(2, '0');

// isoToZonedWallClock formats an instant as the "YYYY-MM-DDTHH:mm" wall
// clock in tz. Empty / invalid input yields ''.
export function isoToZonedWallClock(iso, tz = getUserTimeZone()) {
  if (!iso) return '';
  const d = iso instanceof Date ? iso : new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const p = wallClockParts(d, tz);
  return `${p.year}-${pad(p.month)}-${pad(p.day)}T${pad(p.hour)}:${pad(p.minute)}`;
}

// zonedWallClockToISO reads a "YYYY-MM-DDTHH:mm[:ss]" (or "YYYY-MM-DD")
// wall clock in tz and returns the UTC ISO string. A wall clock that does
// not exist (spring-forward gap) resolves forward, like Canvas. Empty /
// invalid input yields ''.
export function zonedWallClockToISO(local, tz = getUserTimeZone()) {
  if (!local) return '';
  const m = /^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2})(?::(\d{2}))?)?$/.exec(String(local).trim());
  if (!m) return '';
  const [, y, mo, d, h = '0', mi = '0', s = '0'] = m;
  const guess = Date.UTC(+y, +mo - 1, +d, +h, +mi, +s);
  if (Number.isNaN(guess)) return '';
  // Two passes settle DST: the offset at the first estimate may differ
  // from the offset at the answer.
  let ts = guess - zoneOffsetMinutes(new Date(guess), tz) * 60000;
  ts = guess - zoneOffsetMinutes(new Date(ts), tz) * 60000;
  return new Date(ts).toISOString();
}

// formatInUserZone is an explicit formatter for new code.
export function formatInUserZone(value, options = {}, locale = undefined) {
  if (!value) return '';
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleString(locale, { timeZone: getUserTimeZone(), ...options });
}

// startOfDayInZone / endOfDayInZone bound the calendar day containing
// `date` in the user's zone (for "due today" and planner day groups).
export function dayKeyInZone(date, tz = getUserTimeZone()) {
  const p = wallClockParts(date instanceof Date ? date : new Date(date), tz);
  return `${p.year}-${pad(p.month)}-${pad(p.day)}`;
}

export function startOfDayInZone(date, tz = getUserTimeZone()) {
  return new Date(zonedWallClockToISO(dayKeyInZone(date, tz), tz));
}

let installed = false;

// installTimeZoneDefaults patches the three Date#toLocale* methods to
// default `timeZone` to the user's zone. Idempotent.
export function installTimeZoneDefaults() {
  if (installed || typeof Date === 'undefined') return;
  installed = true;
  const wrap = (name) => {
    const original = Date.prototype[name];
    // eslint-disable-next-line no-extend-native
    Date.prototype[name] = function patched(locales, options) {
      if (!userTimeZone || (options && options.timeZone)) {
        return original.call(this, locales, options);
      }
      return original.call(this, locales, { ...(options || {}), timeZone: userTimeZone });
    };
  };
  wrap('toLocaleString');
  wrap('toLocaleDateString');
  wrap('toLocaleTimeString');
}
