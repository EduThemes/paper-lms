import { describe, test, expect } from 'vitest';
import {
  CALCULATION_METHODS,
  methodLabel,
  methodTakesInt,
  intForMethod,
  describeCalculation,
  validateOutcomeForm,
} from './outcomeCalculation';
import { groupMasteryOutcomes, formatMasteryScore } from './studentMastery';

describe('outcome calculation methods (G150/G151)', () => {
  test('offers every Canvas method, including the true decaying average', () => {
    expect(CALCULATION_METHODS.map((m) => m.value)).toEqual([
      'decaying_average', 'standard_decaying_average', 'n_mastery', 'latest', 'highest', 'average',
    ]);
    expect(methodLabel('weighted_average')).toBe('Weighted Average');
    expect(methodLabel('standard_decaying_average')).toBe('Decaying Average');
  });

  test('only the averages and n_mastery take a parameter, with Canvas defaults', () => {
    expect(methodTakesInt('latest')).toBe(false);
    expect(intForMethod('n_mastery', 65)).toBe(5);
    expect(intForMethod('n_mastery', 3)).toBe(3);
    expect(intForMethod('decaying_average', 5)).toBe(5);
    expect(intForMethod('decaying_average', '')).toBe(65);
    expect(intForMethod('highest', 65)).toBeNull();
  });

  test('describes a calculation', () => {
    expect(describeCalculation('decaying_average', 70)).toBe('Most recent score 70%, earlier scores 30%');
    expect(describeCalculation('n_mastery', 1)).toBe('Mastery required 1 time');
  });

  test('validates the outcome form', () => {
    const ratings = [{ description: 'Exceeds', points: 5 }, { description: 'Below', points: 0 }];
    const ok = { title: 'Reads closely', masteryPoints: 3, method: 'n_mastery', calcInt: 2, ratings };
    expect(validateOutcomeForm(ok)).toBeNull();
    expect(validateOutcomeForm({ ...ok, title: ' ' })).toMatch(/Title/);
    expect(validateOutcomeForm({ ...ok, calcInt: 65 })).toMatch(/between 1 and 10/);
    expect(validateOutcomeForm({ ...ok, masteryPoints: 6 })).toMatch(/above the highest rating/);
    expect(validateOutcomeForm({ ...ok, ratings: [ratings[0]] })).toMatch(/two ratings/);
    expect(validateOutcomeForm({ ...ok, method: 'median' })).toMatch(/calculation method/);
  });
});

describe('student mastery helpers (G152)', () => {
  test('groups outcomes by group in server order', () => {
    const groups = groupMasteryOutcomes([
      { id: 1, outcome_group_id: 9, group_title: 'Lab' },
      { id: 2, outcome_group_id: 4, group_title: 'Writing' },
      { id: 3, outcome_group_id: 9, group_title: 'Lab' },
    ]);
    expect(groups.map((g) => [g.title, g.outcomes.map((o) => o.id)])).toEqual([['Lab', [1, 3]], ['Writing', [2]]]);
  });

  test('formats scores', () => {
    expect(formatMasteryScore(null, 4)).toBe('—');
    expect(formatMasteryScore(2.456, 4)).toBe('2.46 / 4');
    expect(formatMasteryScore(3)).toBe('3');
  });
});
