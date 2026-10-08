// Date helpers for the calendar views (month, week, agenda) and the
// simple repeat editor that writes an RFC 5545 RRULE (G095).

export const VIEWS = ['month', 'week', 'agenda'];
export const AGENDA_DAYS = 28;

const pad = (n) => String(n).padStart(2, '0');

// Local calendar day key, YYYY-MM-DD.
export function dayKey(date) {
  const d = date instanceof Date ? date : new Date(date);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function startOfDay(date) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  return d;
}

export function addDays(date, n) {
  const d = new Date(date);
  d.setDate(d.getDate() + n);
  return d;
}

export function startOfWeek(date) {
  const d = startOfDay(date);
  return addDays(d, -d.getDay());
}

// The visible range of a view around date: [start, end) local.
export function viewRange(view, date) {
  if (view === 'week') {
    const start = startOfWeek(date);
    return { start, end: addDays(start, 7) };
  }
  if (view === 'agenda') {
    const start = startOfDay(date);
    return { start, end: addDays(start, AGENDA_DAYS) };
  }
  // Month: whole weeks covering the month, as the grid shows them.
  const first = new Date(date.getFullYear(), date.getMonth(), 1);
  const start = startOfWeek(first);
  const last = new Date(date.getFullYear(), date.getMonth() + 1, 0);
  const end = addDays(startOfWeek(last), 7);
  return { start, end };
}

// Moves the anchor date one view-length back (-1) or forward (+1).
export function shiftDate(view, date, dir) {
  if (view === 'week') return addDays(date, 7 * dir);
  if (view === 'agenda') return addDays(date, AGENDA_DAYS * dir);
  return new Date(date.getFullYear(), date.getMonth() + dir, 1);
}

// The days of a range, one Date per local day.
export function daysIn(start, end) {
  const out = [];
  for (let d = startOfDay(start); d < end; d = addDays(d, 1)) out.push(d);
  return out;
}

// Groups items by the local day they start on, sorted by start time
// (all-day items first).
export function itemsByDay(items) {
  const map = {};
  items.forEach((item) => {
    const key = dayKey(item.start_at);
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
export function toLocalInput(value) {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  return `${dayKey(d)}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fromLocalInput(value) {
  if (!value) return null;
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

export function browserTimeZone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
  } catch {
    return '';
  }
}
