import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppointmentGroupsPage from '../AppointmentGroupsPage';
import { api } from '../../services/api';
import { calendarApi } from '../../services/api_calendar';

vi.mock('../../services/api', () => ({
  api: {
    listAppointmentGroups: vi.fn(),
    listAppointmentSlots: vi.fn(),
    listMyAppointmentReservations: vi.fn(),
    reserveAppointmentSlot: vi.fn(),
    cancelAppointmentReservation: vi.fn(),
    deleteAppointmentGroup: vi.fn(),
  },
}));

vi.mock('../../services/api_calendar', () => ({
  calendarApi: {
    reserveSlot: vi.fn(),
    deleteSlot: vi.fn(),
    listParticipants: vi.fn(),
    messageParticipants: vi.fn(),
  },
}));

let mockIsTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => mockIsTeacher }));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/5/appointment_groups']}>
      <Routes>
        <Route path="/courses/:courseId/appointment_groups" element={<AppointmentGroupsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AppointmentGroupsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = false;
  });

  // B166: the student's own booking (which fills a 1-person slot) stays
  // visible and can be cancelled via GET /users/self/appointments.
  test('student sees and cancels their own full slot', async () => {
    const start = new Date(Date.now() + 86400000).toISOString();
    const end = new Date(Date.now() + 86400000 + 900000).toISOString();
    api.listAppointmentGroups.mockResolvedValue({ data: [{ id: 3, title: 'Office hours', participants_per_appointment: 1 }] });
    api.listAppointmentSlots.mockResolvedValue({
      data: [{ id: 30, start_at: start, end_at: end, available: false, reservation_count: 1, effective_limit: 1 }],
    });
    api.listMyAppointmentReservations.mockResolvedValue({ data: [{ id: 8, slot_id: 30, group_id: 3 }] });
    api.cancelAppointmentReservation.mockResolvedValue({ data: {} });

    renderPage();
    await waitFor(() => expect(screen.getByText('Office hours')).toBeInTheDocument());
    fireEvent.click(screen.getByText('Office hours'));

    await waitFor(() => expect(api.listAppointmentSlots).toHaveBeenCalledWith(3, true));
    const cancel = await screen.findByRole('button', { name: /Reserved — Cancel/i });
    fireEvent.click(cancel);

    await waitFor(() => expect(api.cancelAppointmentReservation).toHaveBeenCalledWith(3, 30, 8));
  });

  // G097: the student's note goes with the reservation, and a group
  // appointment group says the whole group is signed up.
  test('student reserves with a note on a group sign-up', async () => {
    const start = new Date(Date.now() + 86400000).toISOString();
    const end = new Date(Date.now() + 86400000 + 900000).toISOString();
    api.listAppointmentGroups.mockResolvedValue({
      data: [{ id: 3, title: 'Project check-in', participants_per_appointment: 1, participant_type: 'Group', context_codes: ['course_5', 'course_6'] }],
    });
    api.listAppointmentSlots.mockResolvedValue({
      data: [{ id: 30, start_at: start, end_at: end, available: true, reservation_count: 0, effective_limit: 1 }],
    });
    api.listMyAppointmentReservations.mockResolvedValue({ data: [] });
    calendarApi.reserveSlot.mockResolvedValue({ data: {} });

    renderPage();
    await waitFor(() => expect(screen.getByText('Project check-in')).toBeInTheDocument());
    expect(screen.getByText('2 courses')).toBeInTheDocument();
    fireEvent.click(screen.getByText('Project check-in'));
    expect(await screen.findByText(/Your whole group is signed up together/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Note for the instructor (optional)'), { target: { value: 'About the poster' } });
    fireEvent.click(await screen.findByRole('button', { name: 'Reserve' }));
    await waitFor(() => expect(calendarApi.reserveSlot).toHaveBeenCalledWith(3, 30, 'About the poster'));
  });

  test('instructor sees who booked, deletes a slot and messages participants', async () => {
    mockIsTeacher = true;
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const start = new Date(Date.now() + 86400000).toISOString();
    const end = new Date(Date.now() + 86400000 + 900000).toISOString();
    api.listAppointmentGroups.mockResolvedValue({ data: [{ id: 3, title: 'Office hours', participants_per_appointment: 1, workflow_state: 'active' }] });
    api.listAppointmentSlots.mockResolvedValue({
      data: [{ id: 30, start_at: start, end_at: end, available: false, reservation_count: 1, effective_limit: 1, participants: [{ reservation_id: 8, name: 'Jane Doe' }] }],
    });
    api.listMyAppointmentReservations.mockResolvedValue({ data: [] });
    calendarApi.listParticipants.mockResolvedValue({ data: [{ id: 11, name: 'Jane Doe', registered: true }, { id: 12, name: 'John Roe', registered: false }] });
    calendarApi.deleteSlot.mockResolvedValue({ data: null });
    calendarApi.messageParticipants.mockResolvedValue({ data: { sent: 1, recipients: 1 } });

    renderPage();
    fireEvent.click(await screen.findByText('Office hours'));
    expect(await screen.findByText('John Roe')).toBeInTheDocument();
    expect(screen.getAllByText('Jane Doe').length).toBe(2); // slot + roster

    fireEvent.click(screen.getByRole('button', { name: /Delete slot/ }));
    await waitFor(() => expect(calendarApi.deleteSlot).toHaveBeenCalledWith(3, 30));

    fireEvent.click(screen.getByRole('button', { name: 'Message' }));
    fireEvent.change(screen.getByLabelText('Message', { selector: 'textarea' }), { target: { value: 'Please book a time.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Send' }));
    await waitFor(() => expect(calendarApi.messageParticipants).toHaveBeenCalledWith(3, {
      subject: 'Office hours', body: 'Please book a time.', registrationStatus: 'unregistered',
    }));
    expect(await screen.findByText('Message sent to 1 person.')).toBeInTheDocument();
  });
});
