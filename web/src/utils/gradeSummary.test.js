import { describe, test, expect } from 'vitest';
import { applyDropRules, computeTotals, letterFor } from './gradeSummary';

const a = (id, score, possible, extra = {}) => ({
  id,
  name: `A${id}`,
  score,
  points_possible: possible,
  counts_toward_total: true,
  assignment_group_id: 1,
  grading_type: 'points',
  ...extra,
});

describe('drop rules (G147)', () => {
  test('drops the item that helps the ratio most, not the lowest percentage', () => {
    const items = [
      { id: 1, score: 50, possible: 100 },
      { id: 2, score: 0, possible: 5 },
      { id: 3, score: 95, possible: 100 },
    ];
    const { kept, dropped } = applyDropRules(items, { drop_lowest: 1 });
    expect(kept.map((i) => i.id).sort()).toEqual([2, 3]);
    expect([...dropped]).toEqual([1]);
  });

  test('never_drop and the keep-one floor', () => {
    const items = [
      { id: 1, score: 1, possible: 10 },
      { id: 2, score: 9, possible: 10 },
    ];
    // Only one droppable item left once never_drop protects id 1: keep it.
    expect([...applyDropRules(items, { drop_lowest: 1, never_drop: [1] }).dropped]).toEqual([]);
    const three = [...items, { id: 3, score: 10, possible: 10 }];
    expect([...applyDropRules(three, { drop_lowest: 1, never_drop: [1] }).dropped]).toEqual([2]);
    expect(applyDropRules(items, { drop_lowest: 5 }).kept).toHaveLength(1);
  });
});

describe('computeTotals', () => {
  const summary = {
    apply_assignment_group_weights: false,
    assignment_groups: [{ id: 1, group_weight: 0, rules: { drop_lowest: 1 } }],
    assignments: [a(1, 10, 10), a(2, 8, 10), a(3, 2, 10), a(4, null, 10)],
  };

  test('matches the server: lowest dropped, ungraded left out of current', () => {
    const t = computeTotals(summary);
    expect(t.current).toBe(90);
    expect(t.dropped.has(3)).toBe(true);
    // final: ungraded as 0, then drop the lowest (the 0) → (10+8+2)/30
    expect(t.final).toBeCloseTo(66.67, 2);
  });

  test('what-if scores replace real ones', () => {
    const t = computeTotals(summary, { 4: 10 });
    expect(t.current).toBe(93.33);
  });

  test('weighted groups', () => {
    const weighted = {
      apply_assignment_group_weights: true,
      assignment_groups: [
        { id: 1, group_weight: 75, rules: {} },
        { id: 2, group_weight: 25, rules: {} },
      ],
      assignments: [a(1, 8, 10), a(2, 10, 10, { assignment_group_id: 2 })],
    };
    expect(computeTotals(weighted).current).toBe(85);
  });

  test('excused and not-graded never count; extra credit adds points only', () => {
    const s = {
      assignment_groups: [],
      assignments: [
        a(1, 5, 10),
        a(2, null, 10, { excused: true, counts_toward_total: false }),
        a(3, 2, 0),
      ],
    };
    expect(computeTotals(s).current).toBe(70);
  });
});

test('letterFor uses the server scale shape', () => {
  const scale = [{ Name: 'A', Value: 0.9 }, { Name: 'B', Value: 0.8 }, { Name: 'F', Value: 0 }];
  expect(letterFor(91, scale)).toBe('A');
  expect(letterFor(85, scale)).toBe('B');
  expect(letterFor(10, scale)).toBe('F');
  expect(letterFor(null, scale)).toBe('');
});

// The same fixtures as TestCalculate_WhatIfTwinFixtures in
// internal/service/grade_calculator_test.go: What-If with unchanged scores
// must reproduce the server (gradebook engine) total.
describe('computeTotals reproduces the gradebook engine', () => {
  const g = (id, groupId, score, possible = 10, extra = {}) =>
    a(id, score, possible, { assignment_group_id: groupId, ...extra });
  // What-If starts from the real scores (pending-review ones excluded,
  // as the page does).
  const unchanged = (summary) => {
    const scores = {};
    summary.assignments.forEach((x) => {
      if (x.score != null && !x.pending_review) scores[x.id] = Number(String(x.score));
    });
    return scores;
  };

  test('never_drop, weights over 100 and a pending_review quiz', () => {
    const summary = {
      apply_assignment_group_weights: true,
      assignment_groups: [
        { id: 1, group_weight: 70, rules: { drop_lowest: 1, never_drop: [1] } },
        { id: 2, group_weight: 50, rules: {} },
      ],
      assignments: [
        g(1, 1, 2),
        g(2, 1, 3),
        g(3, 1, 9),
        g(4, 2, 8),
        g(5, 2, 4, 10, { pending_review: true, status: 'pending_review' }),
      ],
    };
    const t = computeTotals(summary);
    expect(t.current).toBe(78.5);
    expect(t.final).toBe(58.5);
    expect([...t.dropped]).toEqual([2]);
    const w = computeTotals(summary, unchanged(summary));
    expect(w.current).toBe(78.5);
    expect(w.final).toBe(58.5);
  });

  test('group weights over 100 are not normalised (Canvas)', () => {
    const summary = {
      apply_assignment_group_weights: true,
      assignment_groups: [
        { id: 1, group_weight: 80, rules: {} },
        { id: 2, group_weight: 40, rules: {} },
      ],
      assignments: [g(1, 1, 9), g(2, 2, 6)],
    };
    expect(computeTotals(summary).current).toBe(96);
    expect(computeTotals(summary, unchanged(summary)).current).toBe(96);
  });

  test('never_drop items count in the drop decision', () => {
    const summary = {
      assignment_groups: [{ id: 1, group_weight: 0, rules: { drop_lowest: 1, never_drop: [1] } }],
      assignments: [g(1, 1, 0, 100), g(2, 1, 10, 10), g(3, 1, 50, 100)],
    };
    const t = computeTotals(summary);
    expect(t.current).toBe(25);
    expect([...t.dropped]).toEqual([2]);
  });

  test('drop_highest is ignored when the drops cover every item', () => {
    const summary = {
      assignment_groups: [{ id: 1, group_weight: 0, rules: { drop_lowest: 1, drop_highest: 2 } }],
      assignments: [g(1, 1, 10), g(2, 1, 8), g(3, 1, 2)],
    };
    expect(computeTotals(summary).current).toBe(90);
  });

  test('weighted grading periods combine period grades for the all-periods total', () => {
    const summary = {
      grading_periods_weighted: true,
      grading_period_id: null,
      grading_periods: [
        { id: 10, weight: 25 },
        { id: 11, weight: 75 },
      ],
      assignment_groups: [],
      assignments: [
        a(1, 10, 10, { grading_period_id: 10 }),
        a(2, 5, 10, { grading_period_id: 11 }),
      ],
    };
    // 100% x 25 + 50% x 75 = 62.5 (pooled points would give 75).
    expect(computeTotals(summary).current).toBe(62.5);
  });
});
