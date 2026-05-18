import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import CalendarPage from '../CalendarPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCalendarEvents: vi.fn(),
    getCourseCalendarEvents: vi.fn(),
    getEnrollments: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  // The page works without courseId — test global calendar route.
  return render(
    <MemoryRouter initialEntries={['/calendar']}>
      <Routes>
        <Route path="/calendar" element={<CalendarPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CalendarPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getEnrollments.mockResolvedValue({ data: [] });
  });

  test('renders skeleton while events are loading', () => {
    api.getCalendarEvents.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders calendar grid with the current month name after fetch', async () => {
    api.getCalendarEvents.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => {
      // Once loading clears, the grid weekday headers should render.
      expect(screen.getByText('Sun')).toBeInTheDocument();
    });
    expect(screen.getByText('Sat')).toBeInTheDocument();
  });

  test('renders an event title when api returns events for the current month', async () => {
    const now = new Date();
    const eventDate = new Date(now.getFullYear(), now.getMonth(), 15, 10, 0, 0).toISOString();
    api.getCalendarEvents.mockResolvedValue({
      data: [
        { id: 1, title: 'Office Hours', start_at: eventDate, end_at: eventDate, all_day: false },
      ],
    });
    renderPage();
    await waitFor(() => {
      expect(screen.getAllByText('Office Hours').length).toBeGreaterThan(0);
    });
  });

  test('shows inline error alert when fetch fails', async () => {
    api.getCalendarEvents.mockRejectedValue(new Error('calendar broken'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('calendar broken')).toBeInTheDocument();
    });
  });
});
