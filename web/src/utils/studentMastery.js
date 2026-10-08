// Helpers for the student Learning Mastery view (G152).

/** Groups outcomes by their outcome group, keeping server order. */
export function groupMasteryOutcomes(outcomes) {
  const groups = [];
  const byKey = new Map();
  for (const o of Array.isArray(outcomes) ? outcomes : []) {
    const key = String(o.outcome_group_id || 0);
    let g = byKey.get(key);
    if (!g) {
      g = { key, title: o.group_title || 'Outcomes', outcomes: [] };
      byKey.set(key, g);
      groups.push(g);
    }
    g.outcomes.push(o);
  }
  return groups;
}

const trim = (n) => (Math.round(Number(n) * 100) / 100).toString();

/** "2.5 / 4" — or a dash when not assessed. */
export function formatMasteryScore(score, possible) {
  if (score == null) return '—';
  return possible != null ? `${trim(score)} / ${trim(possible)}` : trim(score);
}
