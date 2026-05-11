import { describe, it, expect } from 'vitest';
import {
  ALL_TYPES,
  NEW_TYPES,
  AUTO_GRADED_TYPES,
  defaultAnswersForType,
  parseAnswers,
  stringifyAnswers,
  TYPE_LABELS,
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
