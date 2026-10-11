// Client-side grade math for the student Grades page (G147).
//
// The server's /courses/:id/grade_summary is the source of truth for the
// real grade (posted scores, drop rules, grading periods, hidden totals).
// The What-If calculator needs the same math in the browser so a student
// can try hypothetical scores; this mirrors service.GradeSummaryService:
//
//   - only assignments that count (published, not "not graded", not
//     excused) take part; 0-point assignments are extra credit;
//   - per assignment group, drop the lowest N then the highest N, never
//     dropping never_drop or extra-credit items and always keeping one;
//   - "current" uses graded work only, "final" counts ungraded as 0;
//   - weighted courses average group percentages by group weight over the
//     groups that have points; unweighted courses pool the points.

// keepByRatio picks `keep` items maximizing (best) or minimizing the
// group's earned/possible ratio (Dinkelbach iteration; a plain percentage
// sort is wrong when points possible differ).
export function keepByRatio(items, keep, best) {
  if (keep >= items.length) return items;
  if (keep <= 0) return [];
  const ratio = (set) => {
    let s = 0;
    let p = 0;
    set.forEach((it) => {
      s += it.score;
      p += it.possible;
    });
    return p === 0 ? 0 : s / p;
  };
  let q = ratio(items);
  let chosen = items;
  for (let i = 0; i < 100; i += 1) {
    const sorted = [...items].sort((a, b) => {
      const va = a.score - q * a.possible;
      const vb = b.score - q * b.possible;
      if (va !== vb) return best ? vb - va : va - vb;
      return a.id - b.id;
    });
    chosen = sorted.slice(0, keep);
    const nq = ratio(chosen);
    if (Math.abs(nq - q) < 1e-12) break;
    q = nq;
  }
  return chosen;
}

// applyDropRules returns { kept, dropped:Set } for one group's items
// ({id, score, possible}).
export function applyDropRules(items, rules = {}) {
  const dropLowest = Math.max(0, Number(rules.drop_lowest) || 0);
  const dropHighest = Math.max(0, Number(rules.drop_highest) || 0);
  const dropped = new Set();
  if (!dropLowest && !dropHighest) return { kept: items, dropped };
  const never = new Set((rules.never_drop || []).map(Number));
  const fixed = [];
  const cands = [];
  items.forEach((it) => {
    if (never.has(Number(it.id)) || it.possible <= 0) fixed.push(it);
    else cands.push(it);
  });
  let remaining = cands;
  if (dropLowest > 0 && remaining.length > 1) {
    remaining = keepByRatio(remaining, Math.max(1, remaining.length - dropLowest), true);
  }
  if (dropHighest > 0 && remaining.length > 1) {
    remaining = keepByRatio(remaining, Math.max(1, remaining.length - dropHighest), false);
  }
  const keptIds = new Set(remaining.map((it) => it.id));
  cands.forEach((it) => {
    if (!keptIds.has(it.id)) dropped.add(it.id);
  });
  return { kept: [...fixed, ...remaining], dropped };
}

const round2 = (v) => Math.round(v * 100) / 100;

// computeTotals derives the course grade from the summary's assignments,
// with `scores` overriding scores by assignment id (What-If). Returns
// { current, final, earned, possible, dropped:Set }; current/final are
// percentages or null.
export function computeTotals(summary, scores = {}) {
  const groups = summary?.assignment_groups || [];
  const groupIds = new Set(groups.map((g) => g.id));
  const perGroup = new Map();
  (summary?.assignments || []).forEach((a) => {
    const override = Object.prototype.hasOwnProperty.call(scores, a.id) ? scores[a.id] : undefined;
    const excused = a.excused && override === undefined;
    if (!a.counts_toward_total && override === undefined) return;
    if (excused || a.grading_type === 'not_graded' || a.omit_from_final_grade) return;
    const possible = Number(a.points_possible) || 0;
    let score = override !== undefined ? override : a.score;
    score = score === '' || score === null || score === undefined ? null : Number(score);
    if (score !== null && Number.isNaN(score)) score = null;
    if (possible <= 0 && score === null) return;
    const gid = groupIds.has(a.assignment_group_id) ? a.assignment_group_id : 0;
    if (!perGroup.has(gid)) perGroup.set(gid, { current: [], final: [] });
    const g = perGroup.get(gid);
    if (score !== null) g.current.push({ id: a.id, score, possible });
    g.final.push({ id: a.id, score: score ?? 0, possible });
  });

  const byId = new Map(groups.map((g) => [g.id, g]));
  const dropped = new Set();
  const totals = [];
  perGroup.forEach((g, gid) => {
    const rules = byId.get(gid)?.rules || {};
    const cur = applyDropRules(g.current, rules);
    cur.dropped.forEach((id) => dropped.add(id));
    const fin = applyDropRules(g.final, rules);
    const sum = (list) => list.reduce((acc, it) => [acc[0] + it.score, acc[1] + it.possible], [0, 0]);
    const [ce, cp] = sum(cur.kept);
    const [fe, fp] = sum(fin.kept);
    totals.push({ gid, weight: Number(byId.get(gid)?.group_weight) || 0, ce, cp, fe, fp });
  });

  let earned = 0;
  let possible = 0;
  let current = null;
  let final = null;
  if (summary?.apply_assignment_group_weights && groups.length > 0) {
    let cs = 0;
    let cw = 0;
    let fs = 0;
    let fw = 0;
    totals.forEach((t) => {
      if (t.gid === 0 || t.weight <= 0) return;
      if (t.cp > 0) {
        cs += (t.ce / t.cp) * 100 * t.weight;
        cw += t.weight;
      }
      if (t.fp > 0) {
        fs += (t.fe / t.fp) * 100 * t.weight;
        fw += t.weight;
      }
      earned += t.ce;
      possible += t.cp;
    });
    if (cw > 0) current = round2(cs / cw);
    if (fw > 0) final = round2(fs / fw);
  } else {
    let fe = 0;
    let fp = 0;
    totals.forEach((t) => {
      earned += t.ce;
      possible += t.cp;
      fe += t.fe;
      fp += t.fp;
    });
    if (possible > 0) current = round2((earned / possible) * 100);
    if (fp > 0) final = round2((fe / fp) * 100);
  }
  return { current, final, earned: round2(earned), possible: round2(possible), dropped };
}

// letterFor maps a percentage to the course grading scale (server shape:
// [{Name, Value}] with Value as a 0-1 fraction).
export function letterFor(pct, scale) {
  if (pct === null || pct === undefined) return '';
  const entries = Array.isArray(scale) && scale.length > 0 ? scale : null;
  if (!entries) return '';
  const frac = Number(pct) / 100;
  for (const e of entries) {
    const name = e.Name ?? e.name ?? (Array.isArray(e) ? e[0] : '');
    let v = e.Value ?? e.value ?? (Array.isArray(e) ? e[1] : 0);
    if (v > 1) v /= 100;
    if (frac >= v) return name;
  }
  const last = entries[entries.length - 1];
  return last.Name ?? last.name ?? '';
}

export const STATUS_LABELS = {
  graded: 'Graded',
  submitted: 'Submitted',
  missing: 'Missing',
  late: 'Late',
  excused: 'Excused',
  not_posted: 'Not posted yet',
  unsubmitted: 'Not submitted',
};
