import { isoToDateTimeLocal, dateTimeLocalToISO } from '../datetimeLocal';

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
