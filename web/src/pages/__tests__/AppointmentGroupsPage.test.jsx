import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AppointmentGroupsPage from '../AppointmentGroupsPage';
import { api } from '../../services/api';

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

vi.mock('../../hooks/useIsTeacher', () => ({ default: () => false }));

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
  beforeEach(() => vi.clearAllMocks());

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
});
