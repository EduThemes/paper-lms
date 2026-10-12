import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import CalendarPage from '../CalendarPage';
import { api } from '../../services/api';
import { calendarApi } from '../../services/api_calendar';

vi.mock('../../services/api', () => ({
  api: {
    getCourseCalendarEvents: vi.fn(),
    getCalendarICalUrl: vi.fn(() => '/api/v1/calendar_events.ics'),
  },
}));

vi.mock('../../services/api_calendar', () => ({
  calendarApi: {
    listContexts: vi.fn(),
    listItems: vi.fn(),
    getEvent: vi.fn(),
    createEvent: vi.fn(),
    updateEvent: vi.fn(),
    deleteEvent: vi.fn(),
    cancelReservation: vi.fn(),
    getFeed: vi.fn(),
    rotateFeed: vi.fn(),
    revokeFeed: vi.fn(),
  },
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

const contexts = [
  { asset_string: 'user_1', type: 'user', id: 1, name: 'Jane Doe', can_create_calendar_events: true },
  {
    asset_string: 'course_5', type: 'course', id: 5, name: 'Astronomy', can_create_calendar_events: true, is_instructor: true,
    sections: [{ id: 51, name: 'Section A' }, { id: 52, name: 'Section B' }],
  },
  { asset_string: 'course_6', type: 'course', id: 6, name: 'Geology', can_create_calendar_events: false },
];

const midMonth = (h = 10) => {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), 15, h, 0, 0).toISOString();
};

function renderPage(path = '/calendar') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/calendar" element={<CalendarPage />} />
        <Route path="/courses/:courseId/calendar" element={<CalendarPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CalendarPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    try { window.localStorage.clear(); } catch { /* ignore */ }
    calendarApi.listContexts.mockResolvedValue({ data: contexts });
  });

  test('renders skeleton while items are loading', () => {
    calendarApi.listItems.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('aggregates events, assignment due dates and appointments from every calendar (G094/G146)', async () => {
    calendarApi.listItems.mockResolvedValue({
      data: [
        { id: 'event_1', type: 'event', title: 'Office Hours', start_at: midMonth(9), context_code: 'user_1', calendar_event_id: 1, can_edit: true },
        { id: 'assignment_2', type: 'assignment', title: 'Lab report', start_at: midMonth(23), context_code: 'course_5', context_name: 'Astronomy', html_url: '/courses/5/assignments/2' },
        { id: 'quiz_3', type: 'quiz', title: 'Star quiz', start_at: midMonth(12), context_code: 'course_6', context_name: 'Geology', html_url: '/courses/6/quizzes/3' },
      ],
    });
    renderPage();
    await waitFor(() => expect(screen.getAllByText('Lab report').length).toBeGreaterThan(0));
    expect(screen.getAllByText('Office Hours').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Star quiz').length).toBeGreaterThan(0);
    // One request for the whole visible range, all calendars.
    const args = calendarApi.listItems.mock.calls.at(-1)[0];
    expect(args.contextCodes).toBeUndefined();
    expect(args.start).toBeInstanceOf(Date);
    // The calendar list shows every calendar.
    expect(screen.getByText('Astronomy')).toBeInTheDocument();
    expect(screen.getByText('Geology')).toBeInTheDocument();
  });

  test('hiding a calendar narrows context_codes', async () => {
    calendarApi.listItems.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => expect(screen.getByText('Geology')).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText('Geology'));
    await waitFor(() => {
      const args = calendarApi.listItems.mock.calls.at(-1)[0];
      expect(args.contextCodes).toEqual(['user_1', 'course_5']);
    });
  });

  test('week and agenda views (G095)', async () => {
    calendarApi.listItems.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => expect(calendarApi.listItems).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: 'Week' }));
    await waitFor(() => {
      const { start, end } = calendarApi.listItems.mock.calls.at(-1)[0];
      expect(Math.round((end - start) / 86400000)).toBe(7);
    });
    fireEvent.click(screen.getByRole('button', { name: 'Agenda' }));
    await waitFor(() => expect(screen.getByText('Nothing scheduled in this period.')).toBeInTheDocument());
  });

  test('course calendar asks only for that course', async () => {
    calendarApi.listItems.mockResolvedValue({ data: [] });
    renderPage('/courses/5/calendar');
    await waitFor(() => expect(calendarApi.listItems).toHaveBeenCalled());
    expect(calendarApi.listItems.mock.calls.at(-1)[0].contextCodes).toEqual(['course_5']);
  });

  test('creates a weekly recurring event', async () => {
    calendarApi.listItems.mockResolvedValue({ data: [] });
    calendarApi.createEvent.mockResolvedValue({ data: {} });
    renderPage();
    await waitFor(() => expect(calendarApi.listItems).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /New Event/i }));
    fireEvent.change(screen.getByLabelText('Calendar'), { target: { value: 'user_1' } });
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Study group' } });
    fireEvent.change(screen.getByLabelText('Start'), { target: { value: '2030-01-07T10:00' } });
    fireEvent.change(screen.getByLabelText('Repeat'), { target: { value: 'WEEKLY' } });
    fireEvent.click(screen.getByRole('button', { name: 'Mon' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(calendarApi.createEvent).toHaveBeenCalled());
    const payload = calendarApi.createEvent.mock.calls[0][0];
    expect(payload.context_code).toBe('user_1');
    expect(payload.rrule).toBe('FREQ=WEEKLY;BYDAY=MO;COUNT=10');
  });

  test('creates section-specific times for a course event', async () => {
    calendarApi.listItems.mockResolvedValue({ data: [] });
    calendarApi.createEvent.mockResolvedValue({ data: {} });
    renderPage('/courses/5/calendar');
    await waitFor(() => expect(calendarApi.listItems).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /New Event/i }));
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Lab' } });
    fireEvent.click(screen.getByLabelText('Use different times for each section'));
    fireEvent.click(screen.getByLabelText('Section A'));
    fireEvent.change(screen.getByLabelText('Section A start'), { target: { value: '2030-01-07T10:00' } });
    fireEvent.change(screen.getByLabelText('Section A end'), { target: { value: '2030-01-07T11:00' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(calendarApi.createEvent).toHaveBeenCalled());
    const payload = calendarApi.createEvent.mock.calls[0][0];
    expect(payload.context_code).toBe('course_5');
    expect(payload.child_event_data).toHaveLength(1);
    expect(payload.child_event_data[0].context_code).toBe('course_section_51');
    expect(payload.rrule).toBeUndefined();
  });

  test('deleting one occurrence of a series sends which=one', async () => {
    calendarApi.listItems.mockResolvedValue({
      data: [{
        id: 'event_9', type: 'event', title: 'Seminar', start_at: midMonth(14), context_code: 'user_1',
        calendar_event_id: 9, can_edit: true, series_uuid: 'abc', rrule: 'FREQ=WEEKLY;COUNT=4',
      }],
    });
    calendarApi.deleteEvent.mockResolvedValue({ data: {} });
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Agenda' }));
    const chip = await screen.findAllByText('Seminar');
    fireEvent.click(chip[0]);
    expect(await screen.findByText('Weekly, 4 times')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    fireEvent.click(screen.getByRole('button', { name: 'This event' }));
    await waitFor(() => expect(calendarApi.deleteEvent).toHaveBeenCalledWith(9, 'one'));
  });

  test('an appointment on the calendar can be cancelled (G097)', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    calendarApi.listItems.mockResolvedValue({
      data: [{
        id: 'reservation_4', type: 'appointment', title: 'Advising', start_at: midMonth(15), end_at: midMonth(16),
        context_code: 'course_5', appointment_group_id: 2, appointment_slot_id: 3, reservation_id: 4, html_url: '/courses/5/appointment_groups',
      }],
    });
    calendarApi.cancelReservation.mockResolvedValue({ data: {} });
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Agenda' }));
    fireEvent.click((await screen.findAllByText('Advising'))[0]);
    fireEvent.click(screen.getByRole('button', { name: 'Cancel reservation' }));
    await waitFor(() => expect(calendarApi.cancelReservation).toHaveBeenCalledWith(2, 3, 4));
  });

  test('an admin outside the course falls back to the course event list', async () => {
    api.getCourseCalendarEvents.mockResolvedValue({ data: [{ id: 7, title: 'Faculty meeting', start_at: midMonth(8), context_code: 'course_99' }] });
    renderPage('/courses/99/calendar');
    await waitFor(() => expect(screen.getAllByText('Faculty meeting').length).toBeGreaterThan(0));
    expect(calendarApi.listItems).not.toHaveBeenCalled();
  });

  test('shows an error when the calendar fails to load', async () => {
    calendarApi.listItems.mockRejectedValue(new Error('calendar broken'));
    renderPage();
    await waitFor(() => expect(screen.getByText('calendar broken')).toBeInTheDocument());
  });
});
