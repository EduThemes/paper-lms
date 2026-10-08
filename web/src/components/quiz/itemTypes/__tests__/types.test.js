import { describe, it, expect } from 'vitest';
import {
  ALL_TYPES,
  NEW_TYPES,
  AUTO_GRADED_TYPES,
  defaultAnswersForType,
  parseAnswers,
  stringifyAnswers,
  TYPE_LABELS,
  computeQuizPointsPossible,
  normalizeQuestionType,
  matchingParts,
} from '../types';

describe('item-type registry', () => {
  it('registers all 9 new item types', () => {
    const newValues = NEW_TYPES.map(t => t.value);
    [
      'multiple_answer',
      'multiple_dropdown',
      'fill_in_the_blank',
      'formula',
      'file_upload',
      'ordering',
      'categorization',
      'hot_spot',
      'text_only',
    ].forEach(t => expect(newValues).toContain(t));
  });

  it('marks the new auto-graded types correctly', () => {
    [
      'multiple_answer',
      'multiple_dropdown',
      'fill_in_the_blank',
      'formula',
      'ordering',
      'categorization',
      'hot_spot',
    ].forEach(t => expect(AUTO_GRADED_TYPES.has(t)).toBe(true));
    // file_upload and text_only are not auto-graded.
    expect(AUTO_GRADED_TYPES.has('file_upload')).toBe(false);
    expect(AUTO_GRADED_TYPES.has('text_only')).toBe(false);
  });

  it('exposes TYPE_LABELS for every registered type', () => {
    ALL_TYPES.forEach(t => expect(TYPE_LABELS[t.value]).toBeTruthy());
  });

  it('defaultAnswersForType returns sensible shapes', () => {
    expect(defaultAnswersForType('multiple_answer').length).toBeGreaterThanOrEqual(2);
    expect(defaultAnswersForType('multiple_answer').filter(a => a.weight > 0).length)
      .toBeGreaterThanOrEqual(2);

    const dropdownDefault = defaultAnswersForType('multiple_dropdown');
    expect(dropdownDefault[0].blank_id).toBe('blank1');

    expect(defaultAnswersForType('file_upload')).toEqual([]);
    expect(defaultAnswersForType('text_only')).toEqual([]);

    const ordering = defaultAnswersForType('ordering');
    expect(ordering.length).toBeGreaterThanOrEqual(3);
    expect(ordering[0].position).toBe(0);

    const cat = defaultAnswersForType('categorization');
    expect(cat[0]).toHaveProperty('items');
    expect(cat[0]).toHaveProperty('buckets');

    const hotSpot = defaultAnswersForType('hot_spot');
    expect(hotSpot[0]).toHaveProperty('image_url');
    expect(Array.isArray(hotSpot[0].regions)).toBe(true);

    const formula = defaultAnswersForType('formula');
    expect(formula[0]).toHaveProperty('formula');
    expect(formula[0]).toHaveProperty('tolerance');
  });

  it('parseAnswers / stringifyAnswers round-trip', () => {
    const input = [{ id: 'a', text: 'one', weight: 100 }];
    const s = stringifyAnswers(input);
    expect(typeof s).toBe('string');
    expect(parseAnswers(s)).toEqual(input);
    expect(parseAnswers('[invalid')).toEqual([]);
    expect(parseAnswers(input)).toEqual(input);
  });
});

describe('quiz parity helpers', () => {
  it('computeQuizPointsPossible honors group pick counts (B131)', () => {
    const questions = [
      { id: 1, points_possible: 1 },
      ...Array.from({ length: 10 }, (_, i) => ({ id: 10 + i, points_possible: 1, quiz_question_group_id: 7 })),
    ];
    const groups = [{ id: 7, pick_count: 2 }];
    expect(computeQuizPointsPossible(questions, groups)).toBe(3);
  });

  it('computeQuizPointsPossible uses points_per_item and ignores bank-draw copies', () => {
    const questions = [
      { id: 1, points_possible: 2 },
      { id: 2, points_possible: 5, quiz_question_group_id: 8, question_bank_entry_id: 44 },
    ];
    const groups = [{ id: 8, pick_count: 3, points_per_item: 2, question_bank_id: 5 }];
    expect(computeQuizPointsPossible(questions, groups)).toBe(8);
  });

  it('computeQuizPointsPossible treats questions of a deleted group as ungrouped', () => {
    expect(computeQuizPointsPossible([{ id: 1, points_possible: 4, quiz_question_group_id: 99 }], [])).toBe(4);
  });

  it('normalizeQuestionType maps legacy Canvas spellings', () => {
    expect(normalizeQuestionType('multiple_answers')).toBe('multiple_answer');
    expect(normalizeQuestionType('multiple_dropdowns')).toBe('multiple_dropdown');
    expect(normalizeQuestionType('essay')).toBe('essay');
  });

  it('matchingParts splits pairs from the right-hand pool', () => {
    const { pairs, pool } = matchingParts([
      { id: '31', left: '31', text: 'France' },
      { id: '901', text: 'Paris', blank_id: '_match_options_' },
      { id: '902', text: 'London', blank_id: '_match_options_' },
    ]);
    expect(pairs).toEqual([{ key: '31', label: 'France' }]);
    expect(pool.map(p => p.id)).toEqual(['901', '902']);
  });
});
