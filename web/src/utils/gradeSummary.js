// Client-side grade math for the student Grades page (G147).
//
// The server's /courses/:id/grade_summary is the source of truth for the
// real grade: it comes from the gradebook's engine
// (internal/service/grade_calculator.go). The What-If calculator needs the
// same math in the browser so a student can try hypothetical scores, so
// this is a port of that engine, and What-If with unchanged scores
// reproduces the server total:
//
//   - only assignments that count (published, assigned, not "not graded",
//     not omitted, not excused) take part; 0-point assignments are extra
//     credit; a pending_review quiz is not graded yet;
//   - per assignment group, drop the lowest N then the highest N choosing
//     the subset with the best (worst) earned/possible ratio; never_drop
//     items are kept but still count in that ratio; drop_lowest keeps at
//     least one droppable item and drop_highest is ignored when both
//     together would drop everything;
//   - "current" uses graded work only, "final" counts ungraded as 0;
//   - weighted courses sum group percentage x weight over the groups with
//     points, scaling up only when those weights total less than 100
//     (weights over 100 are used as is, like Canvas); unweighted courses
//     pool the points;
//   - weighted grading periods combine the period grades by period weight
//     for the all-periods total.

const round2 = (v) => Math.round(v * 100) / 100;

// bestSubset picks keepCount of candidates so that, together with the
// fixed (never-drop) items, earned/possible is maximised (maximise, drop
// lowest) or minimised (drop highest). Dinkelbach iteration, as on the
// server.
export function bestSubset(candidates, fixed, keepCount, maximise) {
  const keep = Math.max(1, keepCount);
  if (candidates.length <= keep) return candidates;
  let fixedScore = 0;
  let fixedPossible = 0;
  fixed.forEach((it) => {
    fixedScore += it.score;
    fixedPossible += it.possible;
  });
  const byId = (a, b) => a.id - b.id;
  const pointed = candidates.some((it) => it.possible > 0);
  if (!pointed && fixedPossible === 0) {
    return [...candidates]
      .sort((a, b) => (a.score !== b.score ? (maximise ? b.score - a.score : a.score - b.score) : byId(a, b)))
      .slice(0, keep);
  }
  const pick = (q) =>
    [...candidates]
      .sort((a, b) => {
        const va = a.score - q * a.possible;
        const vb = b.score - q * b.possible;
        if (va !== vb) return maximise ? vb - va : va - vb;
        return byId(a, b);
      })
      .slice(0, keep);
  const ratio = (set) => {
    let s = fixedScore;
    let p = fixedPossible;
    set.forEach((it) => {
      s += it.score;
      p += it.possible;
    });
    if (p === 0) return maximise ? Infinity : -Infinity;
    return s / p;
  };
  let q = maximise ? -Infinity : Infinity;
  let current = pick(0);
  for (let i = 0; i < 100; i += 1) {
    const r = ratio(current);
    if (!Number.isFinite(r)) return current;
    if ((maximise && r <= q + 1e-12) || (!maximise && r >= q - 1e-12)) break;
    q = r;
    current = pick(q);
  }
  return current;
}

// applyDropRules returns { kept, dropped:Set } for one group's items
// ({id, score, possible}).
export function applyDropRules(items, rules = {}) {
  const dropLowestRule = Math.max(0, Number(rules.drop_lowest) || 0);
  const dropHighestRule = Math.max(0, Number(rules.drop_highest) || 0);
  const dropped = new Set();
  if ((!dropLowestRule && !dropHighestRule) || items.length === 0) return { kept: items, dropped };
  const never = new Set((rules.never_drop || []).map(Number));
  const fixed = [];
  const droppable = [];
  items.forEach((it) => {
    if (never.has(Number(it.id))) fixed.push(it);
    else droppable.push(it);
  });
  const n = droppable.length;
  if (n === 0) return { kept: items, dropped };
  const dropLowest = Math.min(dropLowestRule, n - 1);
  const dropHighest = dropLowest + dropHighestRule >= n ? 0 : dropHighestRule;
  let keep = droppable;
  if (dropLowest > 0) keep = bestSubset(keep, fixed, keep.length - dropLowest, true);
  if (dropHighest > 0) keep = bestSubset(keep, fixed, keep.length - dropHighest, false);
  const keptIds = new Set(keep.map((it) => it.id));
  droppable.forEach((it) => {
    if (!keptIds.has(it.id)) dropped.add(it.id);
  });
  return { kept: [...keep, ...fixed], dropped };
}

// combineGroups is the weighted assignment group total.
function combineGroups(totals, final) {
  let sum = 0;
  let weight = 0;
  let used = 0;
  totals.forEach((t) => {
    if (t.gid === 0) return;
    const score = final ? t.fe : t.ce;
    const possible = final ? t.fp : t.cp;
    if (possible <= 0) return;
    sum += (score / possible) * t.weight;
    weight += t.weight;
    used += 1;
  });
  if (used === 0 || weight === 0) return null;
  if (weight < 100) sum = (sum * 100) / weight;
  return round2(sum);
}

// scoreSet grades one set of summary assignments.
function scoreSet(summary, assignments, scores) {
  const groups = summary?.assignment_groups || [];
  const groupIds = new Set(groups.map((g) => g.id));
  const perGroup = new Map();
  assignments.forEach((a) => {
    const override = Object.prototype.hasOwnProperty.call(scores, a.id) ? scores[a.id] : undefined;
    const excused = a.excused && override === undefined;
    if (!a.counts_toward_total && override === undefined) return;
    if (excused || a.grading_type === 'not_graded' || a.omit_from_final_grade) return;
    const possible = Number(a.points_possible) > 0 ? Number(a.points_possible) : 0;
    let score = override !== undefined ? override : a.score;
    score = score === '' || score === null || score === undefined ? null : Number(score);
    if (score !== null && Number.isNaN(score)) score = null;
    // A pending_review quiz's partial score is not a grade yet (unless
    // the student types a What-If score for it).
    if (a.pending_review && override === undefined) score = null;
    if (possible === 0 && score === null) return;
    const gid = groupIds.has(a.assignment_group_id) ? a.assignment_group_id : 0;
    if (!perGroup.has(gid)) perGroup.set(gid, []);
    perGroup.get(gid).push({ id: a.id, score: score ?? 0, possible, graded: score !== null });
  });

  const byId = new Map(groups.map((g) => [g.id, g]));
  const dropped = new Set();
  const totals = [];
  const sum = (list) => list.reduce((acc, it) => [acc[0] + it.score, acc[1] + it.possible], [0, 0]);
  perGroup.forEach((items, gid) => {
    const rules = gid === 0 ? {} : byId.get(gid)?.rules || {};
    const cur = applyDropRules(items.filter((it) => it.graded), rules);
    cur.dropped.forEach((id) => dropped.add(id));
    const fin = applyDropRules(items, rules);
    const [ce, cp] = sum(cur.kept);
    const [fe, fp] = sum(fin.kept);
    totals.push({ gid, weight: Number(byId.get(gid)?.group_weight) || 0, ce, cp, fe, fp });
  });

  const weighted = !!summary?.apply_assignment_group_weights && groups.length > 0;
  let earned = 0;
  let possible = 0;
  let fe = 0;
  let fp = 0;
  totals.forEach((t) => {
    if (weighted && t.gid === 0) return; // ungrouped work has no weight
    earned += t.ce;
    possible += t.cp;
    fe += t.fe;
    fp += t.fp;
  });
  let current = null;
  let final = null;
  if (weighted) {
    current = combineGroups(totals, false);
    final = combineGroups(totals, true);
  } else {
    if (possible > 0) current = round2((earned / possible) * 100);
    if (fp > 0) final = round2((fe / fp) * 100);
  }
  return { current, final, earned, possible, dropped };
}

// combinePeriods is the weighted grading period total: the weight-scaled
// sum of period percentages, scaled up when the weights in play total less
// than 100; the current grade only uses periods with graded work.
function combinePeriods(periods, final) {
  let sum = 0;
  let weight = 0;
  let used = 0;
  periods.forEach((p) => {
    const pct = final ? p.final : p.current;
    if (pct === null) {
      if (final) weight += p.weight;
      return;
    }
    sum += (pct * p.weight) / 100;
    weight += p.weight;
    used += 1;
  });
  if (used === 0) return null;
  if (weight === 0) return 0;
  return round2((sum * 100) / Math.min(weight, 100));
}

// computeTotals derives the grade for the summary's scope (the selected
// grading period, or the course) from its assignments, with `scores`
// overriding scores by assignment id (What-If). Returns
// { current, final, earned, possible, dropped:Set }; current/final are
// percentages or null.
export function computeTotals(summary, scores = {}) {
  const assignments = summary?.assignments || [];
  const periods = summary?.grading_periods || [];
  const allPeriods = summary?.grading_period_id === null || summary?.grading_period_id === undefined;
  if (summary?.grading_periods_weighted && allPeriods && periods.length > 0) {
    const perPeriod = periods.map((p) => ({
      weight: Number(p.weight) || 0,
      ...scoreSet(summary, assignments.filter((a) => a.grading_period_id === p.id), scores),
    }));
    const dropped = new Set();
    let earned = 0;
    let possible = 0;
    perPeriod.forEach((p) => {
      p.dropped.forEach((id) => dropped.add(id));
      earned += p.earned;
      possible += p.possible;
    });
    return {
      current: combinePeriods(perPeriod, false),
      final: combinePeriods(perPeriod, true),
      earned: round2(earned),
      possible: round2(possible),
      dropped,
    };
  }
  const set = scoreSet(summary, assignments, scores);
  return { ...set, earned: round2(set.earned), possible: round2(set.possible) };
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

// GRADE_DISPLAY_TYPES show the stored grade (a letter, GPA value,
// complete / incomplete or percentage) rather than points.
export const GRADE_DISPLAY_TYPES = new Set(['letter_grade', 'gpa_scale', 'pass_fail', 'percent']);

export const STATUS_LABELS = {
  graded: 'Graded',
  pending_review: 'Pending review',
  submitted: 'Submitted',
  missing: 'Missing',
  late: 'Late',
  excused: 'Excused',
  not_posted: 'Not posted yet',
  unsubmitted: 'Not submitted',
};
