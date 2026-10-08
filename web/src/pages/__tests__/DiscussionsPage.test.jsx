import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DiscussionsPage from '../DiscussionsPage';
import { discussionsApi } from '../../services/api_discussions';

vi.mock('../../services/api', () => ({
  api: {
    deleteDiscussionTopic: vi.fn(),
    getEnrollments: vi.fn(),
  },
}));

vi.mock('../../services/api_discussions', () => ({
  discussionsApi: {
    listTopics: vi.fn(),
    getTopic: vi.fn(),
    createTopic: vi.fn(),
    updateTopic: vi.fn(),
    sections: vi.fn(),
    groupCategories: vi.fn(),
    assignmentGroups: vi.fn(),
  },
}));

let mockIsTeacher = false;
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => mockIsTeacher,
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
  default: ({ value, onChange }) => (
    <textarea data-testid="rce" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/1/discussions']}>
        <Routes>
          <Route path="/courses/:courseId/discussions" element={<DiscussionsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const page = (topics, extra = {}) => ({ topics, page: 1, lastPage: 1, hasNext: false, ...extra });

describe('DiscussionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = false;
    discussionsApi.sections.mockResolvedValue([{ id: 4, name: 'Section A' }]);
    discussionsApi.groupCategories.mockResolvedValue([{ id: 9, name: 'Lab groups' }]);
    discussionsApi.assignmentGroups.mockResolvedValue([]);
  });

  test('renders skeleton while topics are loading', () => {
    discussionsApi.listTopics.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders the topic list after the api resolves', async () => {
    discussionsApi.listTopics.mockResolvedValue(page([
      { id: 1, title: 'Welcome', discussion_type: 'threaded', pinned: true, created_at: '2026-01-01T00:00:00Z' },
      { id: 2, title: 'Week 1 Q&A', discussion_type: 'side_comment', pinned: false, created_at: '2026-01-02T00:00:00Z' },
    ]));

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Welcome')).toBeInTheDocument();
    });
    expect(screen.getByText('Week 1 Q&A')).toBeInTheDocument();
    expect(screen.getByText('Threaded')).toBeInTheDocument();
    expect(screen.getByText('Side Comment')).toBeInTheDocument();
  });

  test('shows reply and unread counts and status badges (G089)', async () => {
    discussionsApi.listTopics.mockResolvedValue(page([
      {
        id: 1,
        title: 'Lab report',
        discussion_type: 'threaded',
        discussion_subentry_count: 5,
        unread_count: 2,
        assignment: { id: 3, points_possible: 10 },
        group_category_id: 9,
        created_at: '2026-01-01T00:00:00Z',
      },
    ]));
    renderPage();
    await waitFor(() => expect(screen.getByText('Lab report')).toBeInTheDocument());
    expect(screen.getByText('5 replies')).toBeInTheDocument();
    expect(screen.getByText('2 unread')).toBeInTheDocument();
    expect(screen.getByText('Graded · 10 pts')).toBeInTheDocument();
    expect(screen.getByText('Group discussion')).toBeInTheDocument();
  });

  test('search, unread filter and paging reach the API (G089)', async () => {
    discussionsApi.listTopics.mockResolvedValue(page([{ id: 1, title: 'Topic one' }], { hasNext: true, lastPage: 3 }));
    renderPage();
    await waitFor(() => expect(screen.getByText('Topic one')).toBeInTheDocument());
    expect(screen.getByText('Page 1 of 3')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() =>
      expect(discussionsApi.listTopics).toHaveBeenLastCalledWith('1', expect.objectContaining({ page: 2 })),
    );

    fireEvent.click(screen.getByRole('button', { name: 'Unread' }));
    await waitFor(() =>
      expect(discussionsApi.listTopics).toHaveBeenLastCalledWith('1', expect.objectContaining({ filter: 'unread', page: 1 })),
    );

    fireEvent.change(screen.getByLabelText('Search discussions'), { target: { value: 'mitosis' } });
    await waitFor(() =>
      expect(discussionsApi.listTopics).toHaveBeenLastCalledWith('1', expect.objectContaining({ search: 'mitosis' })),
    );
  });

  test('shows empty state when no topics exist', async () => {
    discussionsApi.listTopics.mockResolvedValue(page([]));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No discussions yet/i)).toBeInTheDocument();
    });
  });

  test('shows error message when fetch fails', async () => {
    discussionsApi.listTopics.mockRejectedValue(new Error('Bad topic'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Bad topic')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });

  test('teacher creates a graded, delayed discussion (G083, G084)', async () => {
    mockIsTeacher = true;
    discussionsApi.listTopics.mockResolvedValue(page([]));
    discussionsApi.createTopic.mockResolvedValue({ id: 50 });
    renderPage();
    await waitFor(() => expect(screen.getByText(/No discussions yet/i)).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /New Discussion/i }));
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Debate' } });
    fireEvent.change(screen.getByLabelText('Available from'), { target: { value: '2026-11-01T09:00' } });
    fireEvent.click(screen.getByLabelText(/Participants must post before seeing replies/));
    fireEvent.click(screen.getByLabelText(/^Graded/));
    fireEvent.change(screen.getByLabelText('Points'), { target: { value: '15' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Discussion' }));

    await waitFor(() => expect(discussionsApi.createTopic).toHaveBeenCalled());
    const [courseId, payload] = discussionsApi.createTopic.mock.calls[0];
    expect(courseId).toBe('1');
    expect(payload).toMatchObject({
      title: 'Debate',
      require_initial_post: true,
      assignment: { points_possible: 15 },
    });
    expect(payload.delayed_post_at).toBe(new Date('2026-11-01T09:00').toISOString());
  });

  test('anonymity rules out grading and groups in the form (G087)', async () => {
    mockIsTeacher = true;
    discussionsApi.listTopics.mockResolvedValue(page([]));
    renderPage();
    await waitFor(() => expect(screen.getByText(/No discussions yet/i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /New Discussion/i }));
    fireEvent.change(screen.getByLabelText('Anonymity'), { target: { value: 'full_anonymity' } });
    expect(screen.getByLabelText(/^Graded/)).toBeDisabled();
    expect(screen.getByLabelText('Group discussion')).toBeDisabled();
    expect(screen.getByLabelText(/^Only some sections/)).toBeDisabled();
  });
});
