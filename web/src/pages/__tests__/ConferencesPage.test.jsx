import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ConferencesPage from '../ConferencesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getConferences: vi.fn(),
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
  return render(
    <MemoryRouter initialEntries={['/courses/1/conferences']}>
      <Routes>
        <Route path="/courses/:courseId/conferences" element={<ConferencesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ConferencesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('shows loading spinner while fetching conferences', () => {
    api.getConferences.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
    expect(screen.getByText(/Loading conferences/i)).toBeInTheDocument();
  });

  test('renders conferences when api resolves', async () => {
    api.getConferences.mockResolvedValue({
      data: [
        { id: 1, title: 'Office Hours', description: 'Weekly', conference_type: 'BigBlueButton', duration: 60, started_at: null, ended_at: null },
        { id: 2, title: 'Midterm Review', description: '', conference_type: 'BigBlueButton', duration: 90, started_at: null, ended_at: null },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Office Hours')).toBeInTheDocument();
    });
    expect(screen.getByText('Midterm Review')).toBeInTheDocument();
  });

  test('shows error UI when api rejects', async () => {
    api.getConferences.mockRejectedValue(new Error('Unavailable'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Unavailable')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });
});
