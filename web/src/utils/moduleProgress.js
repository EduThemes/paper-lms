// Helpers for module progression (G061/G062) and course pacing rows
// (G079). Pure functions so the UI logic is unit-testable.

// requirementLabel is the short text Canvas shows next to a module item
// with a completion requirement.
export function requirementLabel(type, minScore) {
  switch (type) {
    case 'must_view':
      return 'View';
    case 'must_submit':
      return 'Submit';
    case 'must_contribute':
      return 'Contribute';
    case 'must_mark_done':
      return 'Mark done';
    case 'min_score':
      return minScore === null || minScore === undefined || minScore === ''
        ? 'Score at least'
        : `Score at least ${Number(minScore)}`;
    default:
      return '';
  }
}

// indexProgress turns GET /module_progress into lookups by module and by
// item id. A missing/empty payload yields empty maps.
export function indexProgress(progress) {
  const modules = new Map();
  const items = new Map();
  (progress?.modules || []).forEach((m) => {
    modules.set(m.module_id, m);
    (m.items || []).forEach((it) => items.set(it.item_id, { ...it, module_id: m.module_id }));
  });
  return { modules, items };
}

// moduleStatus maps a module's progression state to a badge.
export function moduleStatus(moduleProgress) {
  switch (moduleProgress?.state) {
    case 'completed':
      return { label: 'Completed', tone: 'success' };
    case 'locked':
      return { label: 'Locked', tone: 'muted' };
    case 'started':
      return { label: 'In progress', tone: 'info' };
    default:
      return null;
  }
}

// progressStateLabel is the teacher grid cell text.
export function progressStateLabel(state) {
  switch (state) {
    case 'completed':
      return 'Completed';
    case 'started':
      return 'In progress';
    case 'unlocked':
      return 'Not started';
    case 'locked':
      return 'Locked';
    default:
      return '—';
  }
}

// buildPaceRows lists every pace-able module item of the course (sub
// headers excluded) in module / item order, with the pace's saved
// duration or 1 day. paceItems is GET .../course_pacing/:id/module_items.
export function buildPaceRows(modules, paceItems) {
  const durations = new Map((paceItems || []).map((p) => [p.module_item_id, p.duration]));
  const rows = [];
  [...(modules || [])]
    .sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || a.id - b.id)
    .forEach((m) => {
      [...(m.items || [])]
        .sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || a.id - b.id)
        .forEach((it) => {
          if (it.type === 'SubHeader') return;
          rows.push({
            module_item_id: it.id,
            title: it.title,
            type: it.type,
            module_name: m.name,
            duration: durations.has(it.id) ? durations.get(it.id) : 1,
          });
        });
    });
  return rows;
}
