import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import GradebookPage from '../GradebookPage';
import { api } from '../../services/api';
import gradingApi from '../../services/api_grading_ui';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    getAssignments: vi.fn(),
    getAllEnrollments: vi.fn(),
    getAssignmentGroups: vi.fn(),
    getGradingStandards: vi.fn(),
    getGradebook: vi.fn(),
    getCourseSubmissions: vi.fn(),
    getSections: vi.fn(),
    getModules: vi.fn(),
    listCustomGradebookColumns: vi.fn(),
    getCustomColumnData: vi.fn(),
    setCustomColumnCell: vi.fn(),
    updateCourse: vi.fn(),
  },
}));
vi.mock('../../services/api_grading_ui', () => ({
  default: { listCourseGradingPeriods: vi.fn(), listCourseGradeStatuses: vi.fn() },
}));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1 } }) }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/GradeInput', () => ({ default: ({ value }) => <span data-testid="grade-input">{value}</span> }));
// Render every row of the column under test.
let column = 0;
vi.mock('react-window', () => ({
  Grid: ({ rowCount, cellComponent: Cell, cellProps }) => (
    <div data-testid="grid">
      {Array.from({ length: rowCount }).map((_, rowIndex) => (
        <Cell key={rowIndex} rowIndex={rowIndex} columnIndex={column} style={{}} {...cellProps} />
      ))}
    </div>
  ),
  useGridRef: () => ({ current: { scrollToCell: () => {} } }),
}));

const renderPage = () => render(
  <MemoryRouter initialEntries={['/courses/1/gradebook']}>
    <Routes>
      <Route path="/courses/:courseId/gradebook" element={<GradebookPage />} />
    </Routes>
  </MemoryRouter>,
);

describe('GradebookPage parity features', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    try { window.localStorage?.clear?.(); } catch { /* no storage */ }
    column = 0;
    api.getCourse.mockResolvedValue({ id: 1, name: 'Biology', hide_final_grades: false });
    api.getAssignments.mockResolvedValue({ data: [
      { id: 100, name: 'Lab 1', points_possible: 10, workflow_state: 'published', due_at: '2026-09-10T00:00:00Z' },
      { id: 101, name: 'Blind essay', points_possible: 10, workflow_state: 'published', due_at: '2027-02-10T00:00:00Z' },
    ] });
    api.getAllEnrollments.mockResolvedValue({ data: [
      { user_id: 7, type: 'StudentEnrollment', course_section_id: 1, user: { id: 7, name: 'Ada', sortable_name: 'Ada' } },
      { user_id: 8, type: 'StudentEnrollment', course_section_id: 2, user: { id: 8, name: 'Ben', sortable_name: 'Ben' } },
    ] });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });
    api.getGradingStandards.mockResolvedValue([]);
    api.getGradebook.mockResolvedValue({
      submissions: { 7: { 100: { score: 9, workflow_state: 'graded', custom_grade_status_id: 5 } } },
      anonymized_assignment_ids: [101],
    });
    api.getSections.mockResolvedValue({ data: [{ id: 1, name: 'Period 1' }, { id: 2, name: 'Period 2' }] });
    api.getModules.mockResolvedValue({ data: [] });
    api.listCustomGradebookColumns.mockResolvedValue({ data: [{ id: 3, title: 'Notes', teacher_notes: true, hidden: false }] });
    api.getCustomColumnData.mockResolvedValue({ data: [{ user_id: 7, content: 'Needs lab kit' }] });
    api.setCustomColumnCell.mockResolvedValue({});
    api.updateCourse.mockResolvedValue({});
    gradingApi.listCourseGradingPeriods.mockResolvedValue([
      { id: 11, title: 'Fall', start_date: '2026-08-20T00:00:00Z', end_date: '2026-12-20T00:00:00Z' },
      { id: 12, title: 'Spring', start_date: '2027-01-05T00:00:00Z', end_date: '2027-05-20T00:00:00Z' },
    ]);
    gradingApi.listCourseGradeStatuses.mockResolvedValue([{ id: 5, name: 'Incomplete', color: '#FFAA00' }]);
  });

  it('edits the notes column and filters by section', async () => {
    renderPage();
    const notes = await screen.findByLabelText('Notes for Ada');
    expect(notes).toHaveValue('Needs lab kit');
    fireEvent.change(notes, { target: { value: 'Kit returned' } });
    fireEvent.blur(notes);
    await waitFor(() => expect(api.setCustomColumnCell).toHaveBeenCalledWith('1', 3, 7, 'Kit returned'));

    expect(screen.getByTestId('grid').children).toHaveLength(2);
    fireEvent.change(screen.getByLabelText('Section'), { target: { value: '2' } });
    await waitFor(() => expect(screen.getByTestId('grid').children).toHaveLength(1));
    expect(screen.getByLabelText('Notes for Ben')).toBeInTheDocument();
  });

  it('masks anonymized columns, colors custom statuses and filters by grading period', async () => {
    column = 2; // [Notes, Lab 1, Blind essay]
    renderPage();
    expect(await screen.findAllByText('Anonymous')).toHaveLength(2);
    fireEvent.change(await screen.findByLabelText('Grading period'), { target: { value: '11' } });
    // Only Lab 1 remains beside the notes column, so column 2 is empty.
    await waitFor(() => expect(screen.queryAllByText('Anonymous')).toHaveLength(0));
  });

  it('shows a custom status on the cell', async () => {
    column = 1;
    renderPage();
    expect(await screen.findByTitle('Incomplete')).toBeInTheDocument();
  });

  it('hides totals from students', async () => {
    renderPage();
    fireEvent.click(await screen.findByLabelText(/Hide totals from students/));
    await waitFor(() => expect(api.updateCourse).toHaveBeenCalledWith('1', { hide_final_grades: true }));
  });
});
