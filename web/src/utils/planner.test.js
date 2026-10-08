import { describe, test, expect } from 'vitest';
import { groupByDay, dayLabel, isDone, windowRange, itemTitle } from './planner';

const tz = 'America/Chicago';

describe('planner helpers (G144)', () => {
  test('groups by day in the user zone, not UTC', () => {
    const items = [
      { plannable_type: 'assignment', plannable_id: 1, plannable_date: '2026-03-10T04:30:00Z' }, // Mar 9, 11:30 PM CDT
      { plannable_type: 'quiz', plannable_id: 2, plannable_date: '2026-03-10T15:00:00Z' },
      { plannable_type: 'planner_note', plannable_id: 3, plannable_date: null },
    ];
    const groups = groupByDay(items, tz);
    expect(groups.map((g) => g.day)).toEqual(['2026-03-09', '2026-03-10', 'undated']);
  });

  test('day labels', () => {
    const now = new Date('2026-03-10T18:00:00Z');
    expect(dayLabel('2026-03-10', now, tz)).toBe('Today');
    expect(dayLabel('2026-03-11', now, tz)).toBe('Tomorrow');
    expect(dayLabel('2026-03-09', now, tz)).toBe('Yesterday');
    expect(dayLabel('2026-03-14', now, tz)).toMatch(/March 14/);
  });

  test('override decides done-ness, else the server flag', () => {
    expect(isDone({ completed: true })).toBe(true);
    expect(isDone({ completed: true, planner_override: { marked_complete: false } })).toBe(false);
    expect(isDone({ completed: false, planner_override: { marked_complete: true } })).toBe(true);
  });

  test('window range starts at local midnight', () => {
    const { start, end } = windowRange(new Date('2026-01-15T18:00:00Z'), 7, tz);
    expect(start).toBe('2026-01-15T06:00:00.000Z');
    expect(end).toBe('2026-01-22T05:59:59.000Z');
  });

  test('titles fall back gracefully', () => {
    expect(itemTitle({ plannable: { title: 'Essay' } })).toBe('Essay');
    expect(itemTitle({})).toBe('Untitled');
  });
});
