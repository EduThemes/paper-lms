import { describe, test, expect } from 'vitest';
import { rollupResponseToView } from './outcomeRollups';

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
