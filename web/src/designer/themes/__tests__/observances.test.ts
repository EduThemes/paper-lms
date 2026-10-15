import { describe, expect, it } from 'vitest';
import { THEMES, getTheme, observanceMonths, upcomingObservances } from '../index';

const ids = (month: number, region?: Parameters<typeof upcomingObservances>[1]) =>
  upcomingObservances(month, region).map((t) => t.id);

describe('observances', () => {
  it('use real months, including per-country months', () => {
    for (const t of THEMES.filter((x) => x.observance)) {
      const o = t.observance!;
      expect(o.months.length, t.id).toBeGreaterThan(0);
      for (const m of [...o.months, ...Object.values(o.regionalMonths ?? {}).flat()]) {
        expect(Number.isInteger(m) && m >= 1 && m <= 12, `${t.id}: ${m}`).toBe(true);
      }
    }
  });

  it('fall back to the general months for countries not listed', () => {
    const o = getTheme('black-history-month')!.observance!;
    expect(observanceMonths(o, 'US')).toEqual([2]);
    expect(observanceMonths(o, 'GB')).toEqual([10]);
    expect(observanceMonths(o, 'AU')).toEqual(o.months);
    expect(observanceMonths(o)).toEqual(o.months);
  });

  it('show each country its own calendar', () => {
    expect(ids(10, 'US')).not.toContain('black-history-month');
    expect(ids(10, 'GB')).toContain('black-history-month');
    expect(ids(1, 'US')).toContain('black-history-month'); // February is next month
    expect(ids(12, 'AU')).toContain('summer-break');
    expect(ids(12, 'US')).not.toContain('summer-break');
    expect(ids(10, 'CA')).toContain('harvest-thanksgiving');
  });
});
