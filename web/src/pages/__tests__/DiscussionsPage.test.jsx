import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import DiscussionsPage from '../DiscussionsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getDiscussionTopics: vi.fn(),
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

vi.mock('../../components/rce/RichContentEditorV2', () => ({
  default: () => <div data-testid="rce" />,
}));

vi.mock('../../components/CrossCourseWarningDialog', () => ({
  default: () => null,
}));

vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({
    issues: [],
    checkAndSave: (_html, fn) => fn(),
    dismiss: vi.fn(),
    confirm: vi.fn(),
  }),
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/discussions']}>
      <Routes>
        <Route path="/courses/:courseId/discussions" element={<DiscussionsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('DiscussionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getEnrollments.mockResolvedValue({ data: [] });
  });

  test('renders skeleton while topics are loading', () => {
    api.getDiscussionTopics.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders the topic list after the api resolves', async () => {
    api.getDiscussionTopics.mockResolvedValue({
      data: [
        { id: 1, title: 'Welcome', discussion_type: 'threaded', pinned: true, created_at: '2026-01-01T00:00:00Z' },
        { id: 2, title: 'Week 1 Q&A', discussion_type: 'side_comment', pinned: false, created_at: '2026-01-02T00:00:00Z' },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Welcome')).toBeInTheDocument();
    });
    expect(screen.getByText('Week 1 Q&A')).toBeInTheDocument();
    expect(screen.getByText('Threaded')).toBeInTheDocument();
    expect(screen.getByText('Side Comment')).toBeInTheDocument();
  });

  test('shows empty state when no topics exist', async () => {
    api.getDiscussionTopics.mockResolvedValue({ data: [] });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No discussions yet/i)).toBeInTheDocument();
    });
  });

  test('shows error message when fetch fails', async () => {
    api.getDiscussionTopics.mockRejectedValue(new Error('Bad topic'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Bad topic')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });
});
