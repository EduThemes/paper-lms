import {
  viewRange, shiftDate, buildRRule, parseRRule, describeRRule, itemsByDay, dayKey, emptyRepeat,
  dayKeysIn, daysIn, addDays, startOfDay, formatDayKey, itemZone,
} from '../calendarView';
import { browserTimeZone, setUserTimeZone, zonedWallClockToISO } from '../../lib/userTimeZone';

const t = (key, opts = {}) => {
  const table = {
    'calendarLane.repeat.every.WEEKLY': opts.count === 1 ? 'Weekly' : `Every ${opts.count} weeks`,
    'calendarLane.repeat.every.DAILY': opts.count === 1 ? 'Daily' : `Every ${opts.count} days`,
    'calendarLane.repeat.on': 'on',
    'calendarLane.weekdays.MO': 'Mon',
    'calendarLane.weekdays.WE': 'Wed',
    'calendarLane.repeat.times': `${opts.count} times`,
    'calendarLane.repeat.until': `until ${opts.date}`,
  };
  return table[key] ?? key;
};

describe('calendarView', () => {
  test('month range covers whole weeks around the month', () => {
    const { start, end } = viewRange('month', new Date(2030, 0, 15)); // Jan 2030
    expect(start.getDay()).toBe(0);
    expect(start <= new Date(2030, 0, 1)).toBe(true);
    expect(end > new Date(2030, 0, 31)).toBe(true);
    expect(end.getDay()).toBe(0);
  });

  test('week range is Sunday to Sunday', () => {
    const { start, end } = viewRange('week', new Date(2030, 0, 16)); // Wednesday
    expect(dayKey(start)).toBe('2030-01-13');
    expect(dayKey(end)).toBe('2030-01-20');
  });

  test('agenda covers four weeks from the day', () => {
    const { start, end } = viewRange('agenda', new Date(2030, 0, 16, 15));
    expect(dayKey(start)).toBe('2030-01-16');
    expect(dayKey(end)).toBe('2030-02-13');
  });

  test('shiftDate moves by the view length', () => {
    expect(dayKey(shiftDate('week', new Date(2030, 0, 16), 1))).toBe('2030-01-23');
    expect(dayKey(shiftDate('month', new Date(2030, 0, 31), 1))).toBe('2030-02-01');
  });

  test('buildRRule writes bounded rules', () => {
    expect(buildRRule(emptyRepeat())).toBe('');
    expect(buildRRule({ ...emptyRepeat(), freq: 'WEEKLY', byDay: ['WE', 'MO'], count: 6 }))
      .toBe('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=6');
    expect(buildRRule({ ...emptyRepeat(), freq: 'DAILY', interval: 2, ends: 'until', until: '2030-03-01' }))
      .toBe('FREQ=DAILY;INTERVAL=2;UNTIL=20300301');
  });

  test('parseRRule round-trips the editor fields', () => {
    const r = parseRRule('FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;UNTIL=20300301');
    expect(r).toMatchObject({ freq: 'WEEKLY', interval: 2, byDay: ['MO', 'WE'], ends: 'until', until: '2030-03-01' });
    expect(buildRRule(r)).toBe('FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;UNTIL=20300301');
  });

  test('describeRRule summarizes a rule', () => {
    expect(describeRRule('FREQ=WEEKLY;BYDAY=MO,WE;COUNT=6', t)).toBe('Weekly on Mon, Wed, 6 times');
    expect(describeRRule('FREQ=DAILY;INTERVAL=3;UNTIL=20300301', t)).toBe('Every 3 days, until 2030-03-01');
  });

  test('itemsByDay groups by local day with all-day items first', () => {
    const items = [
      { id: 'b', start_at: new Date(2030, 0, 2, 15).toISOString(), all_day: false },
      { id: 'a', start_at: new Date(2030, 0, 2, 9).toISOString(), all_day: false },
      { id: 'c', start_at: new Date(2030, 0, 2, 0).toISOString(), all_day: true },
      { id: 'd', start_at: new Date(2030, 0, 3, 9).toISOString(), all_day: false },
    ];
    const byDay = itemsByDay(items);
    expect(byDay['2030-01-02'].map((i) => i.id)).toEqual(['c', 'a', 'b']);
    expect(byDay['2030-01-03']).toHaveLength(1);
  });
});

// G157: the calendar's days are the profile zone's days, whatever zone
// the device (here: the test runner) is in. Pick a profile zone that
// differs from the runner's so a browser-zone fallback would fail.
describe('calendarView in the profile time zone', () => {
  const profile = browserTimeZone() === 'America/New_York' ? 'America/Los_Angeles' : 'America/New_York';
  // Profile-zone wall clock -> instant, independent of the runner's zone.
  const at = (wall) => zonedWallClockToISO(wall, profile);

  beforeEach(() => setUserTimeZone(profile));
  afterEach(() => setUserTimeZone(null));

  test('an 11 PM event lands on its own day, not the next', () => {
    const late = { id: 'late', start_at: at('2030-01-01T23:00'), all_day: false };
    const early = { id: 'early', start_at: at('2030-01-02T00:30'), all_day: false };
    const byDay = itemsByDay([late, early]);
    expect(byDay['2030-01-01'].map((i) => i.id)).toEqual(['late']);
    expect(byDay['2030-01-02'].map((i) => i.id)).toEqual(['early']);
  });

  test('ranges start at profile-zone midnight', () => {
    const anchor = new Date(at('2030-01-16T22:00')); // Wednesday evening
    const week = viewRange('week', anchor);
    expect(week.start.toISOString()).toBe(at('2030-01-13'));
    expect(week.end.toISOString()).toBe(at('2030-01-20'));
    expect(dayKey(week.start)).toBe('2030-01-13');

    const agenda = viewRange('agenda', anchor);
    expect(agenda.start.toISOString()).toBe(at('2030-01-16'));
    expect(dayKey(agenda.end)).toBe('2030-02-13');

    const month = viewRange('month', new Date(at('2030-03-31T23:30')));
    expect(month.start.toISOString()).toBe(at('2030-02-24'));
    expect(month.end.toISOString()).toBe(at('2030-04-07'));
  });

  test('grid days follow profile days across a DST change', () => {
    // US DST starts 2030-03-10.
    const keys = dayKeysIn(new Date(at('2030-03-08')), new Date(at('2030-03-12')));
    expect(keys).toEqual(['2030-03-08', '2030-03-09', '2030-03-10', '2030-03-11']);
    expect(daysIn(new Date(at('2030-03-08')), new Date(at('2030-03-12'))).map((d) => d.toISOString()))
      .toEqual(keys.map((k) => at(k)));
    expect(addDays(new Date(at('2030-03-09T09:00')), 2).toISOString()).toBe(at('2030-03-11T09:00'));
  });

  test('shiftDate moves months and weeks in the profile zone', () => {
    const anchor = new Date(at('2030-01-31T23:30'));
    expect(dayKey(shiftDate('month', anchor, 1))).toBe('2030-02-01');
    expect(dayKey(shiftDate('month', anchor, -1))).toBe('2029-12-01');
    expect(dayKey(shiftDate('week', anchor, 1))).toBe('2030-02-07');
    expect(startOfDay(anchor).toISOString()).toBe(at('2030-01-31'));
  });

  test('an all-day event sits on its date in its own time_zone', () => {
    // Midnight of Jan 2 in Tokyo is still Jan 1 in either US profile zone.
    const tokyoDay = zonedWallClockToISO('2030-01-02', 'Asia/Tokyo');
    const pinned = { id: 'pinned', start_at: tokyoDay, all_day: true, time_zone: 'Asia/Tokyo' };
    const legacy = { id: 'legacy', start_at: tokyoDay, all_day: true, time_zone: null };
    const timed = { id: 'timed', start_at: tokyoDay, all_day: false, time_zone: 'Asia/Tokyo' };
    const byDay = itemsByDay([pinned, legacy, timed]);
    expect(byDay['2030-01-02'].map((i) => i.id)).toEqual(['pinned']);
    expect(byDay['2030-01-01'].map((i) => i.id).sort()).toEqual(['legacy', 'timed']);
    expect(itemZone({ all_day: true, time_zone: 'Not/AZone' })).toBe(profile);
  });

  test('day labels come from the key, not a zone', () => {
    expect(formatDayKey('2030-01-01', { weekday: 'long', month: 'long', day: 'numeric' }, 'en-US'))
      .toBe('Tuesday, January 1');
  });
});
