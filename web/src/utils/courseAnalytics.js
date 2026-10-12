// Pure helpers for course analytics (G154): page-view tracking, weekly
// activity, and the "Message students who…" filters.

// Mirrors service.AnalyticsCategory on the server so a client-reported page
// view and a server-recorded participation group the same way.
const CATEGORY_BY_SEGMENT = {
  '': 'home',
  assignments: 'assignments',
  discussions: 'discussions',
  discussion_topics: 'discussions',
  quizzes: 'quizzes',
  'item-banks': 'quizzes',
  question_banks: 'quizzes',
  pages: 'pages',
  p: 'pages',
  files: 'files',
  modules: 'modules',
  announcements: 'announcements',
  grades: 'grades',
  gradebook: 'grades',
  mastery_gradebook: 'grades',
  syllabus: 'syllabus',
  calendar: 'calendar',
  appointment_groups: 'calendar',
  conferences: 'conferences',
  collaborations: 'collaborations',
  groups: 'groups',
  outcomes: 'outcomes',
  people: 'people',
  external_tools: 'external_tools',
};

/**
 * For a course page path (/courses/12/assignments/3) returns
 * { courseId: '12', category: 'assignments' }; null for any other path.
 * Instructor-only areas (settings, analytics) are not tracked.
 */
export function courseRouteInfo(pathname) {
  if (typeof pathname !== 'string') return null;
  const m = pathname.match(/^\/courses\/(\d+)(?:\/([^/?#]*))?/);
  if (!m) return null;
  const segment = (m[2] || '').toLowerCase();
  if (['settings', 'analytics', 'audit_log', 'blueprint', 'content_import'].includes(segment)) return null;
  return { courseId: m[1], category: CATEGORY_BY_SEGMENT[segment] || 'other' };
}

const DAY_MS = 24 * 60 * 60 * 1000;

// Monday (UTC) of the week containing the YYYY-MM-DD date.
function weekStart(dateStr) {
  const d = new Date(`${dateStr}T00:00:00Z`);
  const dow = (d.getUTCDay() + 6) % 7; // Monday = 0
  return new Date(d.getTime() - dow * DAY_MS);
}

const iso = (d) => d.toISOString().slice(0, 10);

/**
 * Groups per-day activity ({date, views, participations}) into consecutive
 * Monday-start weeks, filling empty weeks with zeros, oldest first.
 */
export function groupByWeek(days) {
  const rows = Array.isArray(days) ? days.filter((d) => d && d.date) : [];
  if (rows.length === 0) return [];
  const byWeek = new Map();
  let first = null;
  let last = null;
  for (const d of rows) {
    const ws = weekStart(d.date);
    const key = iso(ws);
    const cur = byWeek.get(key) || { week_start: key, views: 0, participations: 0 };
    cur.views += Number(d.views) || 0;
    cur.participations += Number(d.participations) || 0;
    byWeek.set(key, cur);
    if (!first || ws < first) first = ws;
    if (!last || ws > last) last = ws;
  }
  const out = [];
  for (let t = first.getTime(); t <= last.getTime(); t += 7 * DAY_MS) {
    const key = iso(new Date(t));
    out.push(byWeek.get(key) || { week_start: key, views: 0, participations: 0 });
  }
  return out;
}

/** Fills gaps in per-day activity so a chart shows idle days as zero. */
export function fillDays(days) {
  const rows = Array.isArray(days) ? days.filter((d) => d && d.date) : [];
  if (rows.length === 0) return [];
  const map = new Map(rows.map((d) => [d.date, d]));
  const sorted = [...map.keys()].sort();
  const out = [];
  const end = new Date(`${sorted[sorted.length - 1]}T00:00:00Z`).getTime();
  for (let t = new Date(`${sorted[0]}T00:00:00Z`).getTime(); t <= end; t += DAY_MS) {
    const key = iso(new Date(t));
    const d = map.get(key);
    out.push({ date: key, views: Number(d?.views) || 0, participations: Number(d?.participations) || 0 });
  }
  return out;
}

export const MESSAGE_CRITERIA = [
  { value: 'not_submitted', label: "Haven't submitted yet", needsAssignment: true },
  { value: 'scored_less_than', label: 'Scored less than', needsAssignment: true, needsValue: true },
  { value: 'scored_more_than', label: 'Scored more than', needsAssignment: true, needsValue: true },
  { value: 'missing_any', label: 'Have missing assignments' },
  { value: 'inactive_days', label: 'Have had no activity for (days)', needsValue: true },
];

/**
 * Students matching a "Message students who…" criterion. summaries are
 * the student_summaries rows (statuses / scores keyed by assignment id).
 */
export function filterStudentsForMessage(summaries, { criterion, assignmentId, value, now = Date.now() } = {}) {
  const list = Array.isArray(summaries) ? summaries : [];
  const aid = assignmentId != null ? String(assignmentId) : null;
  const num = value === '' || value == null ? null : Number(value);
  return list.filter((s) => {
    const status = aid ? s.statuses?.[aid] : null;
    const score = aid ? s.scores?.[aid] : null;
    switch (criterion) {
      case 'not_submitted':
        return !!aid && status !== 'on_time' && status !== 'late' && status !== 'excused';
      case 'scored_less_than':
        return !!aid && num != null && score != null && Number(score) < num;
      case 'scored_more_than':
        return !!aid && num != null && score != null && Number(score) > num;
      case 'missing_any':
        return (s.tardiness_breakdown?.missing || 0) > 0;
      case 'inactive_days': {
        if (num == null) return false;
        if (!s.last_activity_at) return true;
        return now - new Date(s.last_activity_at).getTime() > num * DAY_MS;
      }
      default:
        return false;
    }
  });
}

export function formatDuration(seconds) {
  const s = Math.max(0, Math.round(Number(seconds) || 0));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export const TARDINESS_LABELS = {
  on_time: 'On time',
  late: 'Late',
  missing: 'Missing',
  floating: 'Not due yet',
  excused: 'Excused',
};
