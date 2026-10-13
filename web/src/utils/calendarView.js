// Date helpers for the calendar views (month, week, agenda) and the
// simple repeat editor that writes an RFC 5545 RRULE (G095).

import {
  dayKeyInZone, getUserTimeZone, isoToZonedWallClock, isValidTimeZone, zonedWallClockToISO,
} from '../lib/userTimeZone';

export const VIEWS = ['month', 'week', 'agenda'];
export const AGENDA_DAYS = 28;

const pad = (n) => String(n).padStart(2, '0');

// Every calendar day here is a day in the user's profile time zone
// (G157), not the browser's: the views format times in the profile zone,
// so they must also bucket, label and fetch by its days. Otherwise a
// profile in New York on a UTC laptop shows an 11 PM event in the next
// day's cell. Each helper takes an optional tz (default: the profile
// zone) so tests and callers can pin it.

// Calendar day key, YYYY-MM-DD, in tz.
export function dayKey(date, tz = getUserTimeZone()) {
  return dayKeyInZone(date instanceof Date ? date : new Date(date), tz);
}

const keyParts = (key) => key.split('-').map(Number);

// keyToUTCDate is the day as a UTC-midnight Date, for weekday/date math
// and for formatting a label with { timeZone: 'UTC' } (no zone can then
// move it to a neighbouring day).
export function keyToUTCDate(key) {
  const [y, m, d] = keyParts(key);
  return new Date(Date.UTC(y, m - 1, d));
}

// shiftKey moves a day key by n calendar days.
export function shiftKey(key, n) {
  const d = keyToUTCDate(key);
  d.setUTCDate(d.getUTCDate() + n);
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

// keyWeekday is the day's weekday, 0 = Sunday.
export const keyWeekday = (key) => keyToUTCDate(key).getUTCDay();

// keyDayOfMonth / keyMonth read the day key's own fields (month 0-based).
export const keyDayOfMonth = (key) => keyParts(key)[2];
export const keyMonth = (key) => keyParts(key)[1] - 1;

// keyStart is the instant the day begins in tz.
export function keyStart(key, tz = getUserTimeZone()) {
  return new Date(zonedWallClockToISO(key, tz));
}

// formatDayKey formats a day key without letting any zone shift it.
export function formatDayKey(key, options = {}, locale = undefined) {
  return keyToUTCDate(key).toLocaleDateString(locale, { ...options, timeZone: 'UTC' });
}

export function startOfDay(date, tz = getUserTimeZone()) {
  return keyStart(dayKey(date, tz), tz);
}

// addDays moves date by n calendar days in tz, keeping its wall-clock
// time (a DST change in between doesn't shift it by an hour).
export function addDays(date, n, tz = getUserTimeZone()) {
  const wall = isoToZonedWallClock(date instanceof Date ? date : new Date(date), tz);
  if (!wall) return new Date(NaN);
  const key = shiftKey(wall.slice(0, 10), n);
  return new Date(zonedWallClockToISO(`${key}${wall.slice(10)}`, tz));
}

export function startOfWeek(date, tz = getUserTimeZone()) {
  const key = dayKey(date, tz);
  return keyStart(shiftKey(key, -keyWeekday(key)), tz);
}

// The visible range of a view around date: [start, end), each the start
// of a day in tz.
export function viewRange(view, date, tz = getUserTimeZone()) {
  const key = dayKey(date, tz);
  if (view === 'week') {
    const startKey = shiftKey(key, -keyWeekday(key));
    return { start: keyStart(startKey, tz), end: keyStart(shiftKey(startKey, 7), tz) };
  }
  if (view === 'agenda') {
    return { start: keyStart(key, tz), end: keyStart(shiftKey(key, AGENDA_DAYS), tz) };
  }
  // Month: whole weeks covering the month, as the grid shows them.
  const [y, m] = keyParts(key);
  const firstKey = `${y}-${pad(m)}-01`;
  const lastKey = `${y}-${pad(m)}-${pad(new Date(Date.UTC(y, m, 0)).getUTCDate())}`;
  const startKey = shiftKey(firstKey, -keyWeekday(firstKey));
  const endKey = shiftKey(lastKey, 7 - keyWeekday(lastKey));
  return { start: keyStart(startKey, tz), end: keyStart(endKey, tz) };
}

// Moves the anchor date one view-length back (-1) or forward (+1).
export function shiftDate(view, date, dir, tz = getUserTimeZone()) {
  if (view === 'week') return addDays(date, 7 * dir, tz);
  if (view === 'agenda') return addDays(date, AGENDA_DAYS * dir, tz);
  const [y, m] = keyParts(dayKey(date, tz));
  const first = new Date(Date.UTC(y, m - 1 + dir, 1));
  return keyStart(`${first.getUTCFullYear()}-${pad(first.getUTCMonth() + 1)}-01`, tz);
}

// The day keys of a range [start, end), one per day in tz.
export function dayKeysIn(start, end, tz = getUserTimeZone()) {
  const out = [];
  const endMs = new Date(end).getTime();
  for (let key = dayKey(start, tz); keyStart(key, tz).getTime() < endMs; key = shiftKey(key, 1)) {
    out.push(key);
    if (out.length > 400) break; // a malformed range can't loop forever
  }
  return out;
}

// The days of a range, one Date (the day's start in tz) per day.
export function daysIn(start, end, tz = getUserTimeZone()) {
  return dayKeysIn(start, end, tz).map((key) => keyStart(key, tz));
}

// itemZone is the zone an item's calendar day is read in: an all-day
// event's own time_zone (it is stored as midnight there, so reading it in
// another zone can land on the neighbouring day), else tz.
export function itemZone(item, tz = getUserTimeZone()) {
  return item?.all_day && isValidTimeZone(item.time_zone) ? item.time_zone : tz;
}

// Groups items by the day (in tz) they start on, sorted by start time
// (all-day items first). All-day events are placed on their date in
// their own time_zone.
export function itemsByDay(items, tz = getUserTimeZone()) {
  const map = {};
  items.forEach((item) => {
    const key = dayKey(item.start_at, itemZone(item, tz));
    (map[key] = map[key] || []).push(item);
  });
  Object.values(map).forEach((list) => list.sort((a, b) => {
    if (a.all_day !== b.all_day) return a.all_day ? -1 : 1;
    return new Date(a.start_at) - new Date(b.start_at);
  }));
  return map;
}

// ---- repeat editor <-> RRULE ----

export const WEEKDAY_CODES = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA'];

export const emptyRepeat = () => ({
  freq: 'none', // none | DAILY | WEEKLY | MONTHLY
  interval: 1,
  byDay: [],
  ends: 'count', // count | until
  count: 10,
  until: '', // YYYY-MM-DD
});

// buildRRule turns the repeat editor's state into an RRULE value, or ''
// for a single event. A rule always ends (COUNT or UNTIL), as the server
// requires.
export function buildRRule(repeat) {
  if (!repeat || repeat.freq === 'none' || !repeat.freq) return '';
  const parts = [`FREQ=${repeat.freq}`];
  const interval = Number(repeat.interval) || 1;
  if (interval > 1) parts.push(`INTERVAL=${interval}`);
  if (repeat.freq === 'WEEKLY' && repeat.byDay?.length) {
    const days = WEEKDAY_CODES.filter((d) => repeat.byDay.includes(d));
    parts.push(`BYDAY=${days.join(',')}`);
  }
  if (repeat.ends === 'until' && repeat.until) {
    parts.push(`UNTIL=${repeat.until.replace(/-/g, '')}`);
  } else {
    parts.push(`COUNT=${Math.max(1, Number(repeat.count) || 1)}`);
  }
  return parts.join(';');
}

// parseRRule reads a server RRULE back into the repeat editor's state.
// Parts the editor can't show (BYMONTHDAY, ordinals) are kept out; the
// form then re-saves the series with the simpler rule only if edited.
export function parseRRule(rrule) {
  const out = emptyRepeat();
  if (!rrule) return out;
  rrule.replace(/^RRULE:/i, '').split(';').forEach((part) => {
    const [k, v = ''] = part.split('=');
    switch ((k || '').toUpperCase()) {
      case 'FREQ':
        out.freq = ['DAILY', 'WEEKLY', 'MONTHLY'].includes(v) ? v : 'none';
        break;
      case 'INTERVAL':
        out.interval = Number(v) || 1;
        break;
      case 'BYDAY':
        out.byDay = v.split(',').map((d) => d.slice(-2)).filter((d) => WEEKDAY_CODES.includes(d));
        break;
      case 'COUNT':
        out.ends = 'count';
        out.count = Number(v) || 1;
        break;
      case 'UNTIL':
        out.ends = 'until';
        out.until = `${v.slice(0, 4)}-${v.slice(4, 6)}-${v.slice(6, 8)}`;
        break;
      default:
    }
  });
  return out;
}

// describeRRule is a short English-or-translated summary of a rule.
// t is the i18next t function; the keys live under calendarLane.repeat.
export function describeRRule(rrule, t) {
  if (!rrule) return '';
  const r = parseRRule(rrule);
  if (r.freq === 'none') return t('calendarLane.repeat.custom');
  const unit = t(`calendarLane.repeat.every.${r.freq}`, { count: r.interval });
  let days = '';
  if (r.freq === 'WEEKLY' && r.byDay.length) {
    days = ` ${t('calendarLane.repeat.on')} ${r.byDay.map((d) => t(`calendarLane.weekdays.${d}`)).join(', ')}`;
  }
  const ends = r.ends === 'until' && r.until
    ? t('calendarLane.repeat.until', { date: r.until })
    : t('calendarLane.repeat.times', { count: r.count });
  return `${unit}${days}, ${ends}`;
}

// Local "YYYY-MM-DDTHH:MM" for a datetime-local input.
// datetime-local values are wall-clock times in the profile time zone
// (G157), like every other date form.
export function toLocalInput(value) {
  return isoToZonedWallClock(value);
}

export function fromLocalInput(value) {
  return zonedWallClockToISO(value) || null;
}

export function browserTimeZone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
  } catch {
    return '';
  }
}
