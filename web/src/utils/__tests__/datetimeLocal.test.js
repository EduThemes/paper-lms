import {
  isoToDateTimeLocal, dateTimeLocalToISO, dateInputToStartISO, dateInputToEndISO, isoToDateInput, parseFormDate,
} from '../datetimeLocal';
import { browserTimeZone, setUserTimeZone, zonedWallClockToISO } from '../../lib/userTimeZone';

// B176: editing an enrollment term must not shift its dates by the
// browser's UTC offset. The old code fed `iso.slice(0, 16)` (a UTC wall
// clock) to a datetime-local input, which reads it as local time.
describe('datetime-local helpers', () => {
  it('round-trips an instant without drift', () => {
    const stored = '2026-08-15T00:00:00.000Z';
    const shown = isoToDateTimeLocal(stored);
    expect(dateTimeLocalToISO(shown)).toBe(stored);
    // Saving again (e.g. just a rename) keeps the same instant.
    expect(dateTimeLocalToISO(isoToDateTimeLocal(dateTimeLocalToISO(shown)))).toBe(stored);
  });

  it('shows the local wall clock of the stored instant', () => {
    const stored = '2026-01-10T15:30:00Z';
    const d = new Date(stored);
    const pad = (n) => String(n).padStart(2, '0');
    const expected = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    expect(isoToDateTimeLocal(stored)).toBe(expected);
  });

  it('maps empty and invalid values to empty strings', () => {
    expect(isoToDateTimeLocal(null)).toBe('');
    expect(isoToDateTimeLocal('not a date')).toBe('');
    expect(dateTimeLocalToISO('')).toBe('');
    expect(dateTimeLocalToISO('garbage')).toBe('');
  });
});

// G157: date inputs (grading periods, course start/end) are calendar
// days in the profile zone, whatever the device's zone.
describe('date input helpers in the profile time zone', () => {
  const profile = browserTimeZone() === 'America/New_York' ? 'America/Los_Angeles' : 'America/New_York';
  beforeEach(() => setUserTimeZone(profile));
  afterEach(() => setUserTimeZone(null));

  it('saves a day as its start / last second in the profile zone', () => {
    expect(dateInputToStartISO('2030-09-01')).toBe(zonedWallClockToISO('2030-09-01T00:00', profile));
    expect(dateInputToEndISO('2030-12-20')).toBe(zonedWallClockToISO('2030-12-20T23:59:59', profile));
    expect(dateInputToStartISO('')).toBe('');
    expect(dateInputToEndISO(null)).toBe('');
  });

  it('round-trips a day without moving it', () => {
    for (const day of ['2030-01-01', '2030-03-10', '2030-11-03', '2030-12-31']) {
      expect(isoToDateInput(dateInputToStartISO(day))).toBe(day);
      expect(isoToDateInput(dateInputToEndISO(day))).toBe(day);
    }
    expect(isoToDateInput('')).toBe('');
    expect(isoToDateInput('nope')).toBe('');
  });

  it('parseFormDate reads zone-less wall clocks in the profile zone', () => {
    expect(parseFormDate('2030-05-01T09:30').toISOString()).toBe(zonedWallClockToISO('2030-05-01T09:30', profile));
    expect(parseFormDate('2030-05-01T09:30:00Z').toISOString()).toBe('2030-05-01T09:30:00.000Z');
    expect(Number.isNaN(parseFormDate('').getTime())).toBe(true);
  });
});
