import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import MyAppointmentsPage from '../MyAppointmentsPage';
import { calendarApi } from '../../services/api_calendar';

vi.mock('../../services/api_calendar', () => ({
  calendarApi: {
    listMyAppointments: vi.fn(),
    listMyAppointmentGroups: vi.fn(),
    cancelReservation: vi.fn(),
  },
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <MyAppointmentsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const inHours = (h) => new Date(Date.now() + h * 3600000).toISOString();

describe('MyAppointmentsPage (G097)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  test('lists booked appointments across courses and cancels one', async () => {
    calendarApi.listMyAppointments.mockResolvedValue({
      data: [
        {
          id: 8, slot_id: 30, group_id: 3, title: 'Advising', start_at: inHours(24), end_at: inHours(25),
          course_id: 5, course_name: 'Astronomy', location_name: 'Room 101', participant_name: 'Team Orion', comments: 'About the poster',
        },
      ],
    });
    calendarApi.listMyAppointmentGroups.mockImplementation((scope) => Promise.resolve({
      data: scope === 'reservable'
        ? [{ id: 3, title: 'Advising', course_id: 5 }, { id: 4, title: 'Lab check-off', course_id: 6, participant_type: 'Group' }]
        : [],
    }));
    calendarApi.cancelReservation.mockResolvedValue({ data: {} });

    renderPage();
    expect(await screen.findByText('Advising')).toBeInTheDocument();
    expect(screen.getByText('Astronomy')).toBeInTheDocument();
    expect(screen.getByText('For Team Orion')).toBeInTheDocument();
    expect(screen.getByText('About the poster')).toBeInTheDocument();
    // A group already booked isn't offered again; another one is.
    expect(await screen.findByText('Lab check-off')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel reservation' }));
    await waitFor(() => expect(calendarApi.cancelReservation).toHaveBeenCalledWith(3, 30, 8));
  });

  test('shows an empty state and the groups an instructor manages', async () => {
    calendarApi.listMyAppointments.mockResolvedValue({ data: [] });
    calendarApi.listMyAppointmentGroups.mockImplementation((scope) => Promise.resolve({
      data: scope === 'manageable' ? [{ id: 9, title: 'Thesis meetings', course_id: 5, workflow_state: 'pending' }] : [],
    }));
    renderPage();
    expect(await screen.findByText('You have no upcoming appointments.')).toBeInTheDocument();
    expect(await screen.findByText('Thesis meetings')).toBeInTheDocument();
  });
});
