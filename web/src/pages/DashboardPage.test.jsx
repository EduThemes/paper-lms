import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DashboardPage from './DashboardPage';

// Route the shared request helper by path so the real api_learner module
// (path building + react-query hooks) runs.
const routes = {};
vi.mock('../services/api', () => ({
  api: {
    request: vi.fn((path, opts = {}) => {
      const key = `${opts.method || 'GET'} ${path.split('?')[0]}`;
      const handler = routes[key];
      if (!handler) return Promise.resolve({ data: [] });
      return Promise.resolve(handler(path, opts)).then((data) => ({ data }));
    }),
  },
  getCSRFToken: () => '',
  ApiError: class extends Error {},
}));
const learnerApi = {
  set dashboardCards(fn) { routes['GET /dashboard/dashboard_cards'] = fn; },
  set plannerItems(fn) { routes['GET /planner/items'] = fn; },
  set activityStream(fn) { routes['GET /users/self/activity_stream'] = fn; },
  set missingSubmissions(fn) { routes['GET /users/self/missing_submissions'] = fn; },
  set setPlannerOverride(fn) { routes['POST /planner/overrides'] = fn; },
};

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', email: 'test@example.com' },
    loading: false,
  }),
}));

vi.mock('../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

const card = (id, name, extra = {}) => ({
  id, longName: name, shortName: name, originalName: name, courseCode: `C${id}`,
  href: `/courses/${id}`, isFavorited: false, color: null, ...extra,
});

function renderDashboard() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <DashboardPage />
      </BrowserRouter>
    </QueryClientProvider>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    Object.keys(routes).forEach((k) => delete routes[k]);
  });

  test('renders course cards with nicknames', async () => {
    learnerApi.dashboardCards = () => ([
      card(1, 'Math'),
      card(2, 'Chem lab', { originalName: 'Chemistry', nickname: 'Chem lab', isFavorited: true }),
    ]);
    renderDashboard();
    await waitFor(() => expect(screen.getByText('Math')).toBeInTheDocument());
    expect(screen.getByText('Chem lab')).toBeInTheDocument();
    expect(screen.getByText('Chemistry')).toBeInTheDocument();
    expect(screen.getByText('Favorite courses')).toBeInTheDocument();
  });

  test('coming up lists planner items and marks them done', async () => {
    learnerApi.dashboardCards = () => ([card(1, 'Math')]);
    learnerApi.plannerItems = () => ([{
      plannable_type: 'assignment', plannable_id: 7, plannable_date: new Date(Date.now() + 3600e3).toISOString(),
      plannable: { title: 'Fractions worksheet' }, course_id: 1, context_name: 'Math', html_url: '/courses/1/assignments/7',
      completed: false,
    }]);
    const posted = vi.fn(() => ({}));
    learnerApi.setPlannerOverride = (path, opts) => posted(JSON.parse(opts.body));
    renderDashboard();
    const box = await screen.findByRole('checkbox', { name: /Mark done: Fractions worksheet/ });
    fireEvent.click(box);
    await waitFor(() => expect(posted).toHaveBeenCalledWith({
      // Only the flag being changed: a dismissal stays as it was.
      plannable_type: 'assignment', plannable_id: 7, marked_complete: true,
    }));
    expect(screen.getByText('1 due this week')).toBeInTheDocument();
  });

  test('shows the missing count and recent activity', async () => {
    learnerApi.dashboardCards = () => ([card(1, 'Math')]);
    learnerApi.missingSubmissions = () => ([{ id: 3, name: 'Old', dismissed: false }]);
    learnerApi.activityStream = () => ([{
      id: 'submission_1', type: 'Submission', title: 'Fractions Quiz', context_name: 'Math',
      html_url: '/courses/1/assignments/7', created_at: new Date().toISOString(), score: 9, points_possible: 10,
    }]);
    renderDashboard();
    await waitFor(() => expect(screen.getByText('Fractions Quiz')).toBeInTheDocument());
    expect(screen.getByText(/1 missing/)).toBeInTheDocument();
  });

  test('shows empty state when no courses exist', async () => {
    learnerApi.dashboardCards = () => ([]);
    renderDashboard();
    await waitFor(() => expect(screen.getByText('Welcome to Paper LMS')).toBeInTheDocument());
  });

  test('shows error message when fetch fails', async () => {
    learnerApi.dashboardCards = () => Promise.reject(new Error('Network error'));
    renderDashboard();
    await waitFor(() => expect(screen.getByText('Network error')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /Try again/i })).toBeInTheDocument();
  });
});
