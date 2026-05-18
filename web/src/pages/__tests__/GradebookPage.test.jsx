import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import GradebookPage from '../GradebookPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    getAssignments: vi.fn(),
    getEnrollments: vi.fn(),
    getAssignmentGroups: vi.fn(),
    getGradingStandards: vi.fn(),
    getGradebook: vi.fn(),
    getCourseSubmissions: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

// Force teacher access so the redirect Navigate doesn't preempt loading/error states.
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => true,
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

vi.mock('../../components/GradeInput', () => ({
  default: ({ value }) => <span data-testid="grade-input">{value}</span>,
}));

// react-window's Grid renders a virtualized canvas — replace it with a flat
// element-count renderer so we can assert on rendered students without
// fighting the layout/measurement machinery.
vi.mock('react-window', () => ({
  Grid: ({ rowCount, cellComponent: Cell, cellProps }) => (
    <div data-testid="grid">
      {Array.from({ length: rowCount }).map((_, rowIndex) => (
        <Cell key={rowIndex} rowIndex={rowIndex} columnIndex={0} style={{}} {...cellProps} />
      ))}
    </div>
  ),
  useGridRef: () => ({ current: { scrollToCell: () => {} } }),
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/gradebook']}>
      <Routes>
        <Route path="/courses/:courseId/gradebook" element={<GradebookPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('GradebookPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getGradingStandards.mockResolvedValue([]);
    api.getGradebook.mockResolvedValue({});
    api.getCourseSubmissions.mockResolvedValue({ data: [] });
  });

  test('shows skeleton while data is loading', () => {
    api.getCourse.mockReturnValue(new Promise(() => {}));
    api.getAssignments.mockReturnValue(new Promise(() => {}));
    api.getEnrollments.mockReturnValue(new Promise(() => {}));
    api.getAssignmentGroups.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders the gradebook with students and assignments after fetch', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Calculus I' });
    api.getAssignments.mockResolvedValue({
      data: [{ id: 100, name: 'Problem Set 1', points_possible: 10, assignment_group_id: 1 }],
    });
    api.getEnrollments.mockResolvedValue({
      data: [
        {
          user_id: 7,
          type: 'StudentEnrollment',
          user: { id: 7, name: 'Alice', sortable_name: 'Alice' },
        },
      ],
    });
    api.getAssignmentGroups.mockResolvedValue({ data: [{ id: 1, name: 'Homework' }] });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/Gradebook: Calculus I/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/1 student/i)).toBeInTheDocument();
    expect(screen.getByText(/1 assignment/i)).toBeInTheDocument();
  });

  test('shows error UI when getCourse rejects', async () => {
    api.getCourse.mockRejectedValue(new Error('gradebook down'));
    api.getAssignments.mockResolvedValue({ data: [] });
    api.getEnrollments.mockResolvedValue({ data: [] });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('gradebook down')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });
});
