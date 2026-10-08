import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import IndividualGradebookPage, { assignmentStats } from '../IndividualGradebookPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCourse: vi.fn(),
    getAssignments: vi.fn(),
    getAllEnrollments: vi.fn(),
    getGradebook: vi.fn(),
    gradeSubmission: vi.fn(),
  },
}));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/SubmissionComments', () => ({ default: () => <div data-testid="comments" /> }));

describe('IndividualGradebookPage', () => {
  it('computes assignment statistics', () => {
    expect(assignmentStats([4, null, 8, 6])).toEqual({ count: 3, mean: 6, median: 6, low: 4, high: 8 });
    expect(assignmentStats([])).toBeNull();
  });

  it('grades one student on one assignment', async () => {
    api.getCourse.mockResolvedValue({ id: 1, name: 'Biology', hide_final_grades: true });
    api.getAssignments.mockResolvedValue({ data: [
      { id: 100, name: 'Lab 1', points_possible: 10, workflow_state: 'published' },
      { id: 101, name: 'Practice', points_possible: 10, workflow_state: 'published', omit_from_final_grade: true },
    ] });
    api.getAllEnrollments.mockResolvedValue({ data: [
      { user_id: 7, type: 'StudentEnrollment', user: { name: 'Ada', sortable_name: 'Ada' } },
      { user_id: 8, type: 'StudentEnrollment', user: { name: 'Ben', sortable_name: 'Ben' } },
    ] });
    api.getGradebook.mockResolvedValue({ submissions: {
      7: { 100: { score: 8, workflow_state: 'graded' }, 101: { score: 2, workflow_state: 'graded' } },
      8: { 100: { score: 6, workflow_state: 'graded' } },
    } });
    api.gradeSubmission.mockResolvedValue({ score: 9, workflow_state: 'graded' });
    render(
      <MemoryRouter initialEntries={['/courses/1/gradebook/individual']}>
        <Routes><Route path="/courses/:courseId/gradebook/individual" element={<IndividualGradebookPage />} /></Routes>
      </MemoryRouter>,
    );
    fireEvent.change(await screen.findByLabelText('Student'), { target: { value: '7' } });
    // The omitted practice assignment does not count: 8 / 10.
    expect(screen.getByText(/80% \(8 \/ 10\)/)).toBeInTheDocument();
    expect(screen.getByText('(hidden from the student)')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Assignment'), { target: { value: '100' } });
    expect(screen.getByText('Average').nextSibling).toHaveTextContent('7');
    fireEvent.change(screen.getByLabelText('Score'), { target: { value: '9' } });
    fireEvent.click(screen.getByText('Save grade'));
    await waitFor(() => expect(api.gradeSubmission).toHaveBeenCalledWith('1', 100, 7, { posted_grade: '9' }));
  });
});
