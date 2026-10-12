import { calendarApi } from '../api_calendar';

describe('calendarApi', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: { get: () => null },
      text: () => Promise.resolve('[]'),
    });
  });

  test('listItems sends the range and repeated context_codes[]', async () => {
    await calendarApi.listItems({
      start: new Date('2030-01-01T00:00:00Z'),
      end: new Date('2030-02-01T00:00:00Z'),
      contextCodes: ['user_1', 'course_5'],
    });
    const url = global.fetch.mock.calls[0][0];
    expect(url).toContain('/calendar/items?');
    expect(url).toContain('start_date=2030-01-01T00%3A00%3A00.000Z');
    expect(url).toContain('context_codes%5B%5D=user_1&context_codes%5B%5D=course_5');
  });

  test('updateEvent and deleteEvent carry the series scope', async () => {
    await calendarApi.updateEvent(9, { title: 'Seminar' }, 'following');
    expect(JSON.parse(global.fetch.mock.calls[0][1].body)).toEqual({ calendar_event: { title: 'Seminar' }, which: 'following' });
    await calendarApi.deleteEvent(9, 'all');
    expect(global.fetch.mock.calls[1][0]).toContain('/calendar_events/9?which=all');
  });

  test('reserveSlot sends the note', async () => {
    await calendarApi.reserveSlot(3, 30, 'About the poster');
    expect(global.fetch.mock.calls[0][0]).toContain('/appointment_groups/3/appointments/30/reservations');
    expect(JSON.parse(global.fetch.mock.calls[0][1].body)).toEqual({ comments: 'About the poster' });
  });
});
