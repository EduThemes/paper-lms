import {
  viewRange, shiftDate, buildRRule, parseRRule, describeRRule, itemsByDay, dayKey, emptyRepeat,
} from '../calendarView';

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
