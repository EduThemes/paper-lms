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
