import { describe, test, expect } from 'vitest';
import { fetchAllOutcomeRollups, rollupResponseToView } from './outcomeRollups';

describe('fetchAllOutcomeRollups', () => {
  const rollup = (user) => ({ links: { user }, scores: [] });

  test('reads every page so classes larger than one page are complete', async () => {
    // 25 students served 10 per page (the server caps below the 100 asked).
    const all = Array.from({ length: 25 }, (_, i) => rollup(i + 1));
    const calls = [];
    const getPage = async (page) => {
      calls.push(page);
      return { rollups: all.slice((page - 1) * 10, page * 10), linked: { outcomes: [{ id: 1 }] } };
    };
    const merged = await fetchAllOutcomeRollups(getPage);
    expect(merged.rollups.map((r) => r.links.user)).toEqual(all.map((r) => r.links.user));
    expect(merged.linked.outcomes).toEqual([{ id: 1 }]);
    expect(calls).toEqual([1, 2, 3, 4]);
  });

  test('keeps the outcomes when nobody has results yet', async () => {
    const merged = await fetchAllOutcomeRollups(async () => ({ rollups: [], linked: { outcomes: [{ id: 2 }] } }));
    expect(merged).toEqual({ rollups: [], linked: { outcomes: [{ id: 2 }] } });
  });
});

describe('rollupResponseToView', () => {
  test('maps the outcome_rollups API shape onto the page view model', () => {
    const response = {
      rollups: [
        {
          links: { user: 11 },
          scores: [
            { outcome_id: 1, score: 2.5, mastery: false },
            { outcome_id: 2, score: null, mastery: null },
          ],
        },
      ],
      linked: { outcomes: [{ id: 1, title: 'Reading' }, { id: 2, title: 'Writing' }] },
    };
    const view = rollupResponseToView(response, [
      { user_id: 11, user: { id: 11, name: 'Jane Doe' } },
    ]);
    expect(view.outcomes).toHaveLength(2);
    expect(view.students).toEqual([
      { user_id: 11, user_name: 'Jane Doe', scores: { 1: 2.5 }, mastery: { 1: false } },
    ]);
  });

  test('tolerates an empty response', () => {
    expect(rollupResponseToView(undefined)).toEqual({ outcomes: [], students: [] });
  });
});
