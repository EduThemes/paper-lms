import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AnalyticsPage from '../AnalyticsPage';
import { api } from '../../services/api';
import { analyticsApi } from '../../services/api_learner_outcomes';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    request: vi.fn(),
  },
}));

vi.mock('../../services/api_learner_outcomes', () => ({
  analyticsApi: {
    courseActivity: vi.fn(),
    courseCategories: vi.fn(),
    courseAssignments: vi.fn(),
    studentSummaries: vi.fn(),
  },
  messageStudentsIndividually: vi.fn(),
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

// Stub useIsTeacher → true so the Navigate-to-course redirect doesn't fire.
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/1/analytics']}>
        <Routes>
          <Route path="/courses/:courseId/analytics" element={<AnalyticsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('AnalyticsPage (G154)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getCourse.mockResolvedValue({ id: 1, name: 'Algebra I' });
    analyticsApi.courseActivity.mockResolvedValue([]);
    analyticsApi.courseCategories.mockResolvedValue([]);
    analyticsApi.courseAssignments.mockResolvedValue([]);
    analyticsApi.studentSummaries.mockResolvedValue([]);
  });

  test('shows a loading state while activity loads', () => {
    analyticsApi.courseActivity.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
  });

  test('renders the analytics tabs and course name', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('Algebra I')).toBeInTheDocument());
    expect(screen.getByRole('tab', { name: /Activity/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Assignments/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Students/i })).toBeInTheDocument();
  });

  test('shows the empty state when no activity was recorded', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText(/No student activity recorded yet/i)).toBeInTheDocument());
  });

  test('summarizes page views and participations by week', async () => {
    analyticsApi.courseActivity.mockResolvedValue([
      { date: '2026-03-02', views: 3, participations: 1 },
      { date: '2026-03-10', views: 2, participations: 2 },
    ]);
    renderPage();
    await waitFor(() => expect(screen.getByText('Weekly online activity')).toBeInTheDocument());
    const stat = (label) => screen.getAllByText(label).find((el) => el.tagName === 'P').parentElement.textContent;
    expect(stat('Page views')).toBe('Page views5');
    expect(stat('Participations')).toBe('Participations3');
    expect(stat('Active days')).toBe('Active days2');
    expect(screen.getByText('Course activity')).toBeInTheDocument(); // accessible data table caption
  });

  test('lists students with a drill-down link and opens "Message students who"', async () => {
    analyticsApi.studentSummaries.mockResolvedValue([
      { id: 7, name: 'Jane Doe', page_views: 4, participations: 1, interaction_seconds: 120, last_activity_at: null, tardiness_breakdown: { on_time: 1, late: 0, missing: 1 }, statuses: {}, scores: {} },
    ]);
    renderPage();
    fireEvent.click(screen.getByRole('tab', { name: /Students/i }));
    const link = await screen.findByRole('link', { name: 'Jane Doe' });
    expect(link).toHaveAttribute('href', '/courses/1/analytics/users/7');

    fireEvent.click(screen.getByRole('button', { name: /Message students who/i }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  test('shows an error with retry when activity fails to load', async () => {
    analyticsApi.courseActivity.mockRejectedValue(new Error('analytics down'));
    renderPage();
    await waitFor(() => expect(screen.getByText('analytics down')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /Try again/i })).toBeInTheDocument();
  });
});
