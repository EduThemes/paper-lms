import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import GradebookPage from '../GradebookPage';
import { api } from '../../services/api';
import { gradingPolicyApi } from '../../services/api_grading_policy';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    getAssignments: vi.fn(),
    getAllEnrollments: vi.fn(),
    getAssignmentGroups: vi.fn(),
    getGradingStandards: vi.fn(),
    getGradebook: vi.fn(),
    getCourseSubmissions: vi.fn(),
    gradeSubmission: vi.fn(),
  },
}));

vi.mock('../../services/api_grading_policy', () => ({
  gradingPolicyApi: {
    getCourseGrades: vi.fn(),
    getEffectiveDueDates: vi.fn(),
    getSections: vi.fn(),
  },
  LATE_STATUSES: [],
}));

// The grading-period filter (filter bar) lists the course's periods.
vi.mock('../../services/api_grading_ui', () => ({
  default: {
    listCourseGradingPeriods: vi.fn().mockResolvedValue([
      { id: 50, title: 'Quarter 1', start_date: '2026-01-01T00:00:00Z', end_date: '2026-12-31T00:00:00Z' },
    ]),
    listCourseGradeStatuses: vi.fn().mockResolvedValue([]),
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

// Clicking the stub "types" a B (letter-grade tests below).
vi.mock('../../components/GradeInput', () => ({
  default: ({ value, onSave }) => (
    <span data-testid="grade-input" onClick={() => onSave?.('B')}>
      {value}
    </span>
  ),
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
    gradingPolicyApi.getCourseGrades.mockRejectedValue(new Error('unavailable'));
    gradingPolicyApi.getEffectiveDueDates.mockResolvedValue({});
  });

  test('shows skeleton while data is loading', () => {
    api.getCourse.mockReturnValue(new Promise(() => {}));
    api.getAssignments.mockReturnValue(new Promise(() => {}));
    api.getAllEnrollments.mockReturnValue(new Promise(() => {}));
    api.getAssignmentGroups.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders the gradebook with students and assignments after fetch', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Calculus I' });
    api.getAssignments.mockResolvedValue({
      data: [{ id: 100, name: 'Problem Set 1', points_possible: 10, assignment_group_id: 1 }],
    });
    api.getAllEnrollments.mockResolvedValue({
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
    api.getAllEnrollments.mockResolvedValue({ data: [] });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('gradebook down')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });

  // F19: the gradebook builds its rows from the whole roster (per_page caps
  // at 100, so a fixed page size dropped students in large courses).
  test('loads every student, not just the first page', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Calculus I' });
    api.getAssignments.mockResolvedValue({ data: [] });
    api.getAllEnrollments.mockResolvedValue({
      data: Array.from({ length: 250 }, (_, i) => ({
        user_id: 1000 + i,
        type: 'StudentEnrollment',
        user: { id: 1000 + i, name: `Student ${i}`, sortable_name: `Student ${i}` },
      })),
    });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/250 students/i)).toBeInTheDocument();
    });
    expect(api.getAllEnrollments).toHaveBeenCalledWith('1');
  });

  // G020/G025/G029: the total column shows the server's grade (drop
  // rules, grading periods) and a final grade override; a grading period
  // filter narrows the columns to work due in it.
  test('shows server totals, overrides and filters by grading period', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Calculus I' });
    api.getAssignments.mockResolvedValue({
      data: [
        { id: 100, name: 'Problem Set 1', points_possible: 10 },
        { id: 101, name: 'Problem Set 2', points_possible: 10 },
      ],
    });
    api.getAllEnrollments.mockResolvedValue({
      data: [{ user_id: 7, type: 'StudentEnrollment', user: { id: 7, name: 'Jane Doe', sortable_name: 'Doe, Jane' } }],
    });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });
    gradingPolicyApi.getCourseGrades.mockResolvedValue({
      grading_periods: [{ id: 50, title: 'Quarter 1' }],
      students: [
        {
          user_id: 7,
          current_score: 87.5,
          current_points: 17.5,
          current_possible: 20,
          current_grade: 'B+',
          override_score: 95,
          override_grade: 'A',
          grading_periods: [{ grading_period_id: 50, current_score: 90, current_points: 9, current_possible: 10 }],
        },
      ],
    });
    gradingPolicyApi.getEffectiveDueDates.mockResolvedValue({
      100: { 7: { grading_period_id: 50 } },
      101: { 7: { grading_period_id: 51 } },
    });

    renderPage();
    expect(await screen.findByText('B+')).toBeInTheDocument();
    expect(screen.getByText('(87.5%)')).toBeInTheDocument();
    expect(screen.getByTitle('Final grade override')).toHaveTextContent('A');

    const select = await screen.findByLabelText('Grading period');
    fireEvent.change(select, { target: { value: '50' } });
    expect(await screen.findByText('(90.0%)')).toBeInTheDocument();
  });

  // G023: a letter-graded cell shows the grade the server stored after a
  // save, not the previous entered grade.
  test('a saved letter grade replaces the cell grade', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Calculus I' });
    api.getAssignments.mockResolvedValue({
      data: [{ id: 100, name: 'Essay', points_possible: 10, grading_type: 'letter_grade' }],
    });
    api.getAllEnrollments.mockResolvedValue({
      data: [{ user_id: 7, type: 'StudentEnrollment', user: { id: 7, name: 'Jane Doe', sortable_name: 'Doe, Jane' } }],
    });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });
    api.getGradebook.mockResolvedValue({
      submissions: { 7: { 100: { score: 7.5, grade: 'C', entered_grade: 'C', entered_score: 7.5, workflow_state: 'graded' } } },
    });
    api.gradeSubmission.mockResolvedValue({
      score: 8.9, grade: 'B', entered_grade: 'B', entered_score: 8.9, workflow_state: 'graded', posted_at: '2026-01-15T00:00:00Z',
    });

    renderPage();
    const cell = await screen.findByTestId('grade-input');
    expect(cell).toHaveTextContent('C');
    fireEvent.click(cell);
    await waitFor(() => expect(screen.getByTestId('grade-input')).toHaveTextContent('B'));
    expect(api.gradeSubmission).toHaveBeenCalledWith('1', 100, 7, { posted_grade: 'B' });
  });
});
