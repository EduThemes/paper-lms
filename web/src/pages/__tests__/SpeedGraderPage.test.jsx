import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import SpeedGraderPage from '../SpeedGraderPage';
import { api } from '../../services/api';
import gradingApi from '../../services/api_grading_ui';

vi.mock('../../services/api', () => ({
  api: {
    getSpeedGraderData: vi.fn(),
    getAssignmentRubric: vi.fn(),
    getRubricAssessments: vi.fn(),
  },
}));
vi.mock('../../services/api_grading_ui', () => ({
  default: {
    updateSubmission: vi.fn(),
    saveProvisionalGrade: vi.fn(),
    selectProvisionalGrade: vi.fn(),
    getAnonymousRubricAssessment: vi.fn(),
    saveAnonymousRubricAssessment: vi.fn(),
    listComments: vi.fn(),
    createComment: vi.fn(),
    uploadCommentFile: vi.fn(),
  },
}));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 10, name: 'Pat Teacher' } }) }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/LiveRegion', () => ({ useLiveRegion: () => ({ announce: vi.fn() }) }));
vi.mock('../../components/DocumentViewer', () => ({ default: () => <div data-testid="docviewer" /> }));
vi.mock('../../components/ui/tooltip', () => ({
  TooltipProvider: ({ children }) => <>{children}</>,
  Tooltip: ({ children }) => <>{children}</>,
  TooltipTrigger: ({ children }) => <>{children}</>,
  TooltipContent: () => null,
}));

const renderPage = () => render(
  <MemoryRouter initialEntries={['/courses/1/assignments/2/speedgrader']}>
    <Routes>
      <Route path="/courses/:courseId/assignments/:assignmentId/speedgrader" element={<SpeedGraderPage />} />
    </Routes>
  </MemoryRouter>,
);

const anonymizedPayload = {
  assignment: {
    id: 2, name: 'Lab report', points_possible: 10, anonymous_grading: true, anonymize_students: true,
    moderated_grading: false, grades_published: true, allowed_attempts: 2, is_group_assignment: false,
  },
  custom_grade_statuses: [{ id: 4, name: 'Incomplete', color: '#FFAA00' }],
  students: [
    {
      user_id: null, anonymous_id: 'ab12c', user_name: 'Student 1', attempts_remaining: 1,
      submission: { id: 50, user_id: null, anonymous_id: 'ab12c', workflow_state: 'submitted', submitted_at: '2026-10-01T10:00:00Z', attempt: 2, body: '<p>Second try</p>', score: null },
      submission_history: [
        { attempt: 1, body: '<p>First try</p>', submitted_at: '2026-09-30T10:00:00Z', attachments: [] },
        { attempt: 2, body: '<p>Second try</p>', submitted_at: '2026-10-01T10:00:00Z', attachments: [] },
      ],
      comments: [],
    },
    {
      user_id: null, anonymous_id: 'zz99y', user_name: 'Student 2', attempts_remaining: 2,
      submission: { id: 51, workflow_state: 'unsubmitted', score: null }, submission_history: [], comments: [],
    },
  ],
};

describe('SpeedGraderPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getAssignmentRubric.mockRejectedValue(new Error('no rubric'));
  });

  it('grades, excuses and sets statuses by anonymous id', async () => {
    api.getSpeedGraderData.mockResolvedValue(anonymizedPayload);
    gradingApi.updateSubmission.mockImplementation((_c, _a, _k, update) => Promise.resolve({
      score: update.posted_grade ? Number(update.posted_grade) : null,
      workflow_state: 'graded',
      excused: !!update.excused,
      custom_grade_status_id: update.custom_grade_status_id ?? null,
      user_id: 20,
    }));
    renderPage();
    // The first student is selected once the preview shows their work.
    expect(await screen.findByText('Second try')).toBeInTheDocument();
    expect(screen.getByText('Students anonymized')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Grade score'), { target: { value: '8' } });
    fireEvent.click(screen.getByText('Update Grade'));
    await waitFor(() => expect(gradingApi.updateSubmission).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c' }, { posted_grade: '8' }));

    fireEvent.change(screen.getByLabelText('Submission status'), { target: { value: 'custom:4' } });
    await waitFor(() => expect(gradingApi.updateSubmission).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c' }, { custom_grade_status_id: 4 }));

    fireEvent.click(screen.getByLabelText('Excuse this student'));
    await waitFor(() => expect(gradingApi.updateSubmission).toHaveBeenCalledWith('1', '2', { anonymousId: 'ab12c' }, { excused: true }));
  });

  it('shows earlier attempts and filters the student list', async () => {
    api.getSpeedGraderData.mockResolvedValue(anonymizedPayload);
    renderPage();
    expect(await screen.findByText('Second try')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Submission attempt'), { target: { value: '1' } });
    expect(await screen.findByText('First try')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Filter students'), { target: { value: 'not_submitted' } });
    const list = screen.getByRole('listbox', { name: 'Students' });
    expect(within(list).getByText('Student 2')).toBeInTheDocument();
    expect(within(list).queryByText('Student 1')).not.toBeInTheDocument();
  });

  it('saves provisional grades on an unpublished moderated assignment', async () => {
    api.getSpeedGraderData.mockResolvedValue({
      ...anonymizedPayload,
      assignment: { ...anonymizedPayload.assignment, anonymous_grading: false, anonymize_students: false, moderated_grading: true, grades_published: false, is_moderator: true },
      students: [{
        ...anonymizedPayload.students[0], anonymous_id: undefined, user_id: 20, user_name: 'Jane Doe',
        provisional_grades: [{ id: 70, score: 6, mine: false, scorer_name: 'Grader A1B2C', selected: false }],
      }],
    });
    gradingApi.saveProvisionalGrade.mockResolvedValue({ id: 71, score: 7, final: false });
    gradingApi.selectProvisionalGrade.mockResolvedValue({});
    renderPage();
    expect(await screen.findByText('Second try')).toBeInTheDocument();
    expect(screen.getByText('Save provisional grade')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Grade score'), { target: { value: '7' } });
    fireEvent.click(screen.getByText('Save provisional grade'));
    await waitFor(() => expect(gradingApi.saveProvisionalGrade).toHaveBeenCalledWith('1', '2', { userId: 20, anonymousId: undefined, postedGrade: '7' }));
    expect(gradingApi.updateSubmission).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByRole('button', { name: 'Select' })[0]);
    await waitFor(() => expect(gradingApi.selectProvisionalGrade).toHaveBeenCalledWith('1', '2', { userId: 20, anonymousId: undefined, provisionalGradeId: 70 }));
  });
});
