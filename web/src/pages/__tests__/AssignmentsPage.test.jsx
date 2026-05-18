import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AssignmentsPage from '../AssignmentsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getAssignments: vi.fn(),
    getAssignmentGroups: vi.fn(),
    getGroupCategories: vi.fn(),
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
    <MemoryRouter initialEntries={['/courses/1/assignments']}>
      <Routes>
        <Route path="/courses/:courseId/assignments" element={<AssignmentsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AssignmentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // useIsTeacher hits getEnrollments — return non-teacher by default
    api.getEnrollments.mockResolvedValue({ data: [] });
  });

  test('shows skeleton while assignments are loading', () => {
    api.getAssignments.mockReturnValue(new Promise(() => {}));
    api.getAssignmentGroups.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    // Skeleton component renders elements with animate-pulse class
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders assignments grouped by assignment group on success', async () => {
    api.getAssignments.mockResolvedValue({
      data: [
        { id: 10, name: 'Essay 1', assignment_group_id: 1, points_possible: 50, published: true },
        { id: 11, name: 'Essay 2', assignment_group_id: 1, points_possible: 50, published: true },
        { id: 12, name: 'Ungrouped Quiz', points_possible: 10, published: true },
      ],
    });
    api.getAssignmentGroups.mockResolvedValue({
      data: [{ id: 1, name: 'Essays', group_weight: 50 }],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Essay 1')).toBeInTheDocument();
    });
    expect(screen.getByText('Essay 2')).toBeInTheDocument();
    expect(screen.getByText('Essays')).toBeInTheDocument();
    expect(screen.getByText('Ungrouped Quiz')).toBeInTheDocument();
    expect(screen.getByText('Other Assignments')).toBeInTheDocument();
  });

  test('shows empty-state copy when there are no assignments', async () => {
    api.getAssignments.mockResolvedValue({ data: [] });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No assignments yet/i)).toBeInTheDocument();
    });
  });

  test('shows error UI when fetch rejects', async () => {
    api.getAssignments.mockRejectedValue(new Error('Boom'));
    api.getAssignmentGroups.mockRejectedValue(new Error('Boom'));

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Boom')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });

  test('filters assignments by the search input', async () => {
    api.getAssignments.mockResolvedValue({
      data: [
        { id: 10, name: 'Essay One', points_possible: 50, published: true },
        { id: 11, name: 'Quiz Alpha', points_possible: 20, published: true },
      ],
    });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Essay One')).toBeInTheDocument();
    });

    const search = screen.getByLabelText(/Search assignments/i);
    fireEvent.change(search, { target: { value: 'quiz' } });

    expect(screen.queryByText('Essay One')).not.toBeInTheDocument();
    expect(screen.getByText('Quiz Alpha')).toBeInTheDocument();
  });
});
