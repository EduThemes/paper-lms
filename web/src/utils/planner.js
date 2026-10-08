// Planner / to-do helpers (G144): group planner items by calendar day in
// the user's time zone and describe each item.

import { dayKeyInZone, getUserTimeZone, zonedWallClockToISO } from '../lib/userTimeZone';

export const PLANNABLE_LABELS = {
  assignment: 'Assignment',
  quiz: 'Quiz',
  discussion_topic: 'Discussion',
  announcement: 'Announcement',
  calendar_event: 'Event',
  planner_note: 'To do',
  wiki_page: 'Page',
};

export function itemTitle(item) {
  return item?.plannable?.title || item?.plannable?.name || 'Untitled';
}

export function itemKey(item) {
  return `${item.plannable_type}:${item.plannable_id}`;
}

// isDone: the user's explicit override wins, otherwise the server's
// completed flag (submitted work counts as done).
export function isDone(item) {
  if (item?.planner_override) return !!item.planner_override.marked_complete;
  return !!item?.completed;
}

// groupByDay buckets items by their plannable_date's day in tz, in date
// order; undated items go last.
export function groupByDay(items, tz = getUserTimeZone()) {
  const groups = new Map();
  (items || []).forEach((it) => {
    const key = it.plannable_date ? dayKeyInZone(new Date(it.plannable_date), tz) : 'undated';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(it);
  });
  return [...groups.entries()]
    .sort(([a], [b]) => (a === 'undated' ? 1 : b === 'undated' ? -1 : a.localeCompare(b)))
    .map(([day, list]) => ({ day, items: list }));
}

// dayLabel renders "Today", "Tomorrow", "Yesterday" or a weekday + date
// for a YYYY-MM-DD key in tz.
export function dayLabel(dayKey, now = new Date(), tz = getUserTimeZone()) {
  if (dayKey === 'undated') return 'No date';
  const today = dayKeyInZone(now, tz);
  const shift = (days) => dayKeyInZone(new Date(now.getTime() + days * 86400000), tz);
  if (dayKey === today) return 'Today';
  if (dayKey === shift(1)) return 'Tomorrow';
  if (dayKey === shift(-1)) return 'Yesterday';
  const noonISO = zonedWallClockToISO(`${dayKey}T12:00`, tz);
  return new Date(noonISO).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', timeZone: tz });
}

// weekRange returns the planner window [start, end) as ISO strings: the
// day containing `anchor` (in tz) plus `days` days.
export function windowRange(anchor, days, tz = getUserTimeZone()) {
  const startKey = dayKeyInZone(anchor, tz);
  const start = zonedWallClockToISO(startKey, tz);
  const endKey = dayKeyInZone(new Date(new Date(start).getTime() + (days + 0.5) * 86400000), tz);
  const end = new Date(new Date(zonedWallClockToISO(endKey, tz)).getTime() - 1000).toISOString();
  return { start, end };
}
