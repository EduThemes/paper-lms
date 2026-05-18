import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AssignmentPage from '../AssignmentPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getAssignment: vi.fn(),
    getEnrollments: vi.fn(),
    getAssignmentOverrides: vi.fn(),
    getSections: vi.fn(),
    getAssignmentGroups: vi.fn(),
    getGroupCategories: vi.fn(),
    getSubmissions: vi.fn(),
    getSubmission: vi.fn(),
    listPeerReviews: vi.fn(),
    listMyPeerReviews: vi.fn(),
    getOutcomeAlignments: vi.fn(),
    getCourseOutcomes: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'student' },
    loading: false,
  }),
}));

// The student path is the simpler one — keep it predictable.
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => false,
}));

vi.mock('../../hooks/useUnsavedChanges', () => ({
  default: () => {},
}));

vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({
    issues: [],
    checkAndSave: (_html, fn) => fn(),
    dismiss: vi.fn(),
    confirm: vi.fn(),
  }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

vi.mock('../../components/SubmissionForm', () => ({
  default: () => <div data-testid="submission-form" />,
}));

vi.mock('../../components/SubmissionComments', () => ({
  default: () => <div data-testid="submission-comments" />,
}));

vi.mock('../../components/RichContentViewer', () => ({
  default: ({ content }) => <div data-testid="rich-content">{content}</div>,
  sanitizeHTML: (html) => html,
}));

vi.mock('../../components/RichContentEditor', () => ({
  default: () => <div data-testid="rich-content-editor" />,
}));

vi.mock('../../components/CrossCourseWarningDialog', () => ({
  default: () => null,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/assignments/2']}>
      <Routes>
        <Route
          path="/courses/:courseId/assignments/:assignmentId"
          element={<AssignmentPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AssignmentPage (student view)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getEnrollments.mockResolvedValue({ data: [] });
    api.getSubmission.mockResolvedValue(null);
    api.listMyPeerReviews.mockResolvedValue([]);
  });

  test('renders the loading spinner while the assignment is being fetched', () => {
    api.getAssignment.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
  });

  test('renders the assignment name and points after fetch resolves', async () => {
    api.getAssignment.mockResolvedValue({
      id: 2,
      name: 'Final Project',
      description: '<p>Write a paper.</p>',
      points_possible: 100,
      submission_types: ['online_text_entry'],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Final Project')).toBeInTheDocument();
    });
    expect(screen.getByText(/100 points/i)).toBeInTheDocument();
    expect(screen.getByTestId('submission-form')).toBeInTheDocument();
  });

  test('renders the error block when getAssignment rejects', async () => {
    api.getAssignment.mockRejectedValue(new Error('assignment missing'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('assignment missing')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });

  test('renders the on-paper notice when submission types are paper-only', async () => {
    api.getAssignment.mockResolvedValue({
      id: 2,
      name: 'In-Class Quiz',
      description: '',
      points_possible: 10,
      submission_types: ['on_paper'],
    });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/submitted on paper/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId('submission-form')).not.toBeInTheDocument();
  });
});
