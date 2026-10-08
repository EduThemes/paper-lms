import { describe, test, expect } from 'vitest';
import { datetimeLocalToISO, isoToDatetimeLocal } from './datetimeLocal';

describe('datetime-local round trip (B208)', () => {
  test('editing and saving without changes keeps the same instant', () => {
    const due = '2026-03-01T03:59:00.000Z';
    // Round trip many times: the instant must never drift by the UTC offset.
    let value = due;
    for (let i = 0; i < 3; i += 1) {
      value = datetimeLocalToISO(isoToDatetimeLocal(value));
    }
    expect(value).toBe(due);
  });

  test('the input shows local wall-clock time', () => {
    const d = new Date(2026, 2, 1, 23, 59); // local 23:59
    expect(isoToDatetimeLocal(d.toISOString())).toBe('2026-03-01T23:59');
  });

  test('empty and invalid values', () => {
    expect(isoToDatetimeLocal(null)).toBe('');
    expect(isoToDatetimeLocal('nope')).toBe('');
    expect(datetimeLocalToISO('')).toBeNull();
  });
});
