import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import DashboardPage from './DashboardPage';
import { api } from '../services/api';

vi.mock('../services/api', () => ({
  api: { getCourses: vi.fn() },
}));

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', email: 'test@example.com' },
    loading: false,
    login: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  }),
}));

vi.mock('../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../components/RichContentViewer', () => ({
  sanitizeHTML: (html) => html,
}));

function renderDashboard() {
  return render(
    <BrowserRouter>
      <DashboardPage />
    </BrowserRouter>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('shows loading skeleton while courses are being fetched', () => {
    // Keep the promise pending so loading state persists
    api.getCourses.mockReturnValue(new Promise(() => {}));
    const { container } = renderDashboard();

    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
    expect(screen.queryByText('Welcome to Paper LMS')).not.toBeInTheDocument();
  });

  test('renders courses after successful fetch', async () => {
    api.getCourses.mockResolvedValue({
      data: [
        { id: 1, name: 'Math', course_code: 'MTH101' },
        { id: 2, name: 'Science', course_code: 'SCI201' },
      ],
    });
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('Math')).toBeInTheDocument();
    });

    expect(screen.getByText('MTH101')).toBeInTheDocument();
    expect(screen.getByText('Science')).toBeInTheDocument();
    expect(screen.getByText('SCI201')).toBeInTheDocument();
    expect(api.getCourses).toHaveBeenCalledWith(1, 50);
  });

  test('shows empty state when no courses exist', async () => {
    api.getCourses.mockResolvedValue({ data: [] });
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('Welcome to Paper LMS')).toBeInTheDocument();
    });

    expect(
      screen.getByText(/You are not enrolled in any courses yet/i),
    ).toBeInTheDocument();
    expect(screen.queryByText('Loading courses...')).not.toBeInTheDocument();
  });

  test('shows error message when fetch fails', async () => {
    api.getCourses.mockRejectedValue(new Error('Network error'));
    renderDashboard();

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
    expect(screen.queryByText('Loading courses...')).not.toBeInTheDocument();
  });
});
