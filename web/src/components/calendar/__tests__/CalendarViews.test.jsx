import React from 'react';
import { render, fireEvent } from '@testing-library/react';
import { MonthView, WeekView, AgendaView } from '../CalendarViews';
import { viewRange } from '../../../utils/calendarView';
import { browserTimeZone, setUserTimeZone, zonedWallClockToISO } from '../../../lib/userTimeZone';

// G157: the views bucket, label and click by profile-zone days. The
// profile zone is chosen to differ from the test runner's zone so a
// browser-zone fallback would put things on the wrong day.
const profile = browserTimeZone() === 'America/New_York' ? 'Asia/Tokyo' : 'America/New_York';
const at = (wall) => zonedWallClockToISO(wall, profile);

const lateEvent = {
  id: 'late',
  type: 'event',
  title: 'Late lab',
  start_at: at('2030-01-15T23:00'),
  end_at: at('2030-01-15T23:45'),
  all_day: false,
  context_code: 'course_1',
};

describe('calendar views in the profile time zone', () => {
  beforeEach(() => setUserTimeZone(profile));
  afterEach(() => setUserTimeZone(null));

  test('month view puts an 11 PM event in its own day cell', () => {
    const anchor = new Date(at('2030-01-15T12:00'));
    const onDayClick = vi.fn();
    const { container } = render(
      <MonthView
        items={[lateEvent]}
        range={viewRange('month', anchor)}
        anchor={anchor}
        colorIndex={{}}
        onSelect={() => {}}
        onDayClick={onDayClick}
      />,
    );
    const cell = container.querySelector('[data-day="2030-01-15"]');
    expect(cell).toHaveTextContent('Late lab');
    expect(container.querySelector('[data-day="2030-01-16"]')).not.toHaveTextContent('Late lab');
    // Jan 1 2030 is a Tuesday: the grid starts on Sunday Dec 30.
    expect(container.querySelector('[data-day]')).toHaveAttribute('data-day', '2029-12-30');
    expect(container.querySelector('[data-day]')).toHaveTextContent(/^30/);
    fireEvent.click(cell);
    expect(onDayClick.mock.calls[0][0].toISOString()).toBe(at('2030-01-15'));
  });

  test('week view headers are the profile days', () => {
    const anchor = new Date(at('2030-01-15T12:00'));
    const { container } = render(
      <WeekView items={[lateEvent]} range={viewRange('week', anchor)} colorIndex={{}} onSelect={() => {}} />,
    );
    const days = [...container.querySelectorAll('[data-day]')].map((el) => el.getAttribute('data-day'));
    expect(days).toEqual(['2030-01-13', '2030-01-14', '2030-01-15', '2030-01-16', '2030-01-17', '2030-01-18', '2030-01-19']);
    const tue = container.querySelector('[data-day="2030-01-15"]');
    expect(tue).toHaveTextContent('Late lab');
    expect(tue).toHaveTextContent('15');
  });

  test('agenda labels the day the event is on', () => {
    const { container } = render(<AgendaView items={[lateEvent]} colorIndex={{}} onSelect={() => {}} />);
    const day = container.querySelector('[data-day]');
    expect(day).toHaveAttribute('data-day', '2030-01-15');
    expect(day).toHaveTextContent(/15/);
    expect(day).toHaveTextContent('11:00');
  });
});
