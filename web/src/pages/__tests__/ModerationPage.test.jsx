import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ModerationPage from '../ModerationPage';
import gradingApi from '../../services/api_grading_ui';

vi.mock('../../services/api_grading_ui', () => ({
  default: {
    getModeration: vi.fn(),
    selectProvisionalGrade: vi.fn(),
    saveProvisionalGrade: vi.fn(),
    publishModeratedGrades: vi.fn(),
  },
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));

const payload = {
  assignment: { id: 2, name: 'Essay', points_possible: 10, grader_count: 2, grades_published: false, anonymize_students: true },
  graders: [{ grader_id: 1, name: 'Grader A1B2C' }, { grader_id: 2, name: 'Sam Assistant' }],
  students: [
    {
      submission_id: 9, anonymous_id: 'ab12c', user_id: null, name: 'Student 1',
      provisional_grades: [
        { id: 31, score: 7, final: false, scorer_name: 'Grader A1B2C' },
        { id: 32, score: 9, final: false, scorer_name: 'Sam Assistant' },
      ],
      selected_provisional_grade_id: null,
    },
  ],
};

const renderPage = () => render(
  <MemoryRouter initialEntries={['/courses/1/assignments/2/moderate']}>
    <Routes>
      <Route path="/courses/:courseId/assignments/:assignmentId/moderate" element={<ModerationPage />} />
    </Routes>
  </MemoryRouter>,
);

describe('ModerationPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('selects a grader’s grade and releases the grades', async () => {
    gradingApi.getModeration.mockResolvedValue(payload);
    gradingApi.selectProvisionalGrade.mockResolvedValue({});
    gradingApi.publishModeratedGrades.mockResolvedValue({ published: 1 });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderPage();
    expect(await screen.findByText('Student 1')).toBeInTheDocument();
    expect(screen.getByText(/1 student has provisional grades/)).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Select Sam Assistant's grade for Student 1"));
    await waitFor(() => expect(gradingApi.selectProvisionalGrade).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c', provisionalGradeId: 32 }));
    fireEvent.click(screen.getByText('Release grades'));
    await waitFor(() => expect(gradingApi.publishModeratedGrades).toHaveBeenCalledWith('1', '2'));
  });

  it('enters and selects the moderator’s own grade', async () => {
    gradingApi.getModeration.mockResolvedValue(payload);
    gradingApi.saveProvisionalGrade.mockResolvedValue({ id: 40, score: 8, final: true });
    gradingApi.selectProvisionalGrade.mockResolvedValue({});
    renderPage();
    fireEvent.change(await screen.findByLabelText("Moderator's grade for Student 1"), { target: { value: '8' } });
    fireEvent.click(screen.getByText('Save'));
    await waitFor(() => expect(gradingApi.saveProvisionalGrade).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c', postedGrade: '8', final: true }));
    await waitFor(() => expect(gradingApi.selectProvisionalGrade).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c', provisionalGradeId: 40 }));
  });
});
