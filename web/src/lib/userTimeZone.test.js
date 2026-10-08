import { describe, test, expect, afterEach } from 'vitest';
import {
  setUserTimeZone,
  getUserTimeZone,
  browserTimeZone,
  zonedWallClockToISO,
  isoToZonedWallClock,
  installTimeZoneDefaults,
  timeZoneMismatch,
  dayKeyInZone,
  isValidTimeZone,
} from './userTimeZone';
import { dateTimeLocalToISO, isoToDateTimeLocal } from '../utils/datetimeLocal';

afterEach(() => setUserTimeZone(null));

describe('user time zone (G157)', () => {
  test('falls back to the browser zone and ignores invalid zones', () => {
    expect(getUserTimeZone()).toBe(browserTimeZone());
    setUserTimeZone('Not/AZone');
    expect(getUserTimeZone()).toBe(browserTimeZone());
    expect(isValidTimeZone('America/Chicago')).toBe(true);
  });

  test('wall clock in a zone converts to the right instant', () => {
    // 23:59 in Chicago in January (CST, UTC-6) is 05:59Z the next day.
    expect(zonedWallClockToISO('2026-01-15T23:59', 'America/Chicago')).toBe('2026-01-16T05:59:00.000Z');
    // In July (CDT, UTC-5).
    expect(zonedWallClockToISO('2026-07-15T23:59', 'America/Chicago')).toBe('2026-07-16T04:59:00.000Z');
    expect(zonedWallClockToISO('2026-07-15T08:00', 'Asia/Kolkata')).toBe('2026-07-15T02:30:00.000Z');
    expect(zonedWallClockToISO('', 'UTC')).toBe('');
    expect(zonedWallClockToISO('soon', 'UTC')).toBe('');
  });

  test('instants format as the zone wall clock', () => {
    expect(isoToZonedWallClock('2026-01-16T05:59:00Z', 'America/Chicago')).toBe('2026-01-15T23:59');
    expect(isoToZonedWallClock('2026-07-15T02:30:00Z', 'Asia/Kolkata')).toBe('2026-07-15T08:00');
    expect(isoToZonedWallClock('nope', 'UTC')).toBe('');
  });

  test('round trips across a DST change without drift', () => {
    const tz = 'America/New_York';
    for (const local of ['2026-03-07T12:00', '2026-03-08T12:00', '2026-11-01T01:30', '2026-11-02T00:00']) {
      expect(isoToZonedWallClock(zonedWallClockToISO(local, tz), tz)).toBe(local);
    }
  });

  test('datetime-local helpers read the wall clock in the profile zone', () => {
    setUserTimeZone('Pacific/Auckland');
    expect(dateTimeLocalToISO('2026-06-01T09:00')).toBe('2026-05-31T21:00:00.000Z');
    expect(isoToDateTimeLocal('2026-05-31T21:00:00.000Z')).toBe('2026-06-01T09:00');
  });

  test('toLocale* default to the profile zone unless a zone is given', () => {
    installTimeZoneDefaults();
    const d = new Date('2026-01-16T05:59:00Z');
    setUserTimeZone('America/Chicago');
    expect(d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })).toBe('11:59 PM');
    expect(d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })).toBe('Jan 15');
    expect(d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', timeZone: 'UTC' })).toBe('5:59 AM');
    expect(dayKeyInZone(d)).toBe('2026-01-15');
  });

  test('mismatch only when the offsets differ', () => {
    setUserTimeZone(browserTimeZone());
    expect(timeZoneMismatch()).toBe(false);
    const far = browserTimeZone() === 'Pacific/Kiritimati' ? 'Pacific/Pago_Pago' : 'Pacific/Kiritimati';
    setUserTimeZone(far);
    expect(timeZoneMismatch()).toBe(true);
  });
});
