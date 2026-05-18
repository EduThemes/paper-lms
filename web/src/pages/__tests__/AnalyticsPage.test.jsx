import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AnalyticsPage from '../AnalyticsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    request: vi.fn(),
    getEnrollments: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

// Stub useIsTeacher → true so the Navigate-to-course redirect doesn't fire and
// loading-state hinges only on the getCourse call.
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => true,
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/analytics']}>
      <Routes>
        <Route path="/courses/:courseId/analytics" element={<AnalyticsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AnalyticsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.request.mockResolvedValue({ data: [] });
  });

  test('shows loading spinner while course is being fetched', () => {
    api.getCourse.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
    expect(screen.getByText(/Loading analytics/i)).toBeInTheDocument();
  });

  test('renders the analytics tabs and course name after fetch', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Algebra I' });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Algebra I')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Activity/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Assignments/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Students/i })).toBeInTheDocument();
  });

  test('shows empty-state message when there is no activity data', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Algebra I' });
    api.request.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No activity data recorded yet/i)).toBeInTheDocument();
    });
  });

  test('renders error state when getCourse rejects', async () => {
    api.getCourse.mockRejectedValue(new Error('analytics down'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('analytics down')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });
});
