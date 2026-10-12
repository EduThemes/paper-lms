import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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
    listPeerReviews: vi.fn(),
    getOutcomeAlignments: vi.fn(),
    getCourseOutcomes: vi.fn(),
    updateAssignment: vi.fn(),
    getCourseFiles: vi.fn(),
    getAssignmentRubric: vi.fn(),
  },
}));
vi.mock('../../services/api_grading_ui', () => ({
  default: { assignPeerReview: vi.fn(), deletePeerReview: vi.fn(), listReceivedPeerReviews: vi.fn() },
}));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 10, name: 'Pat Teacher' } }) }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));
vi.mock('../../hooks/useUnsavedChanges', () => ({ default: () => {} }));
vi.mock('../../hooks/useCrossCourseCheck', () => ({
  default: () => ({ issues: [], checkAndSave: (_h, fn) => fn(), dismiss: vi.fn(), confirm: vi.fn() }),
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/SubmissionComments', () => ({ default: () => null }));
vi.mock('../../components/RichContentViewer', () => ({ default: () => null, sanitizeHTML: (h) => h, renderMathInElement: () => {} }));
vi.mock('../../components/RichContentEditor', () => ({ default: () => null }));
vi.mock('../../components/CrossCourseWarningDialog', () => ({ default: () => null }));

describe('AssignmentPage grading settings (teacher)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getEnrollments.mockResolvedValue({ data: [
      { user_id: 10, type: 'TeacherEnrollment', user: { name: 'Pat Teacher' } },
      { user_id: 11, type: 'TaEnrollment', user: { name: 'Sam Assistant' } },
    ] });
    api.getAssignmentOverrides.mockResolvedValue([]);
    api.getSections.mockResolvedValue({ data: [] });
    api.getAssignmentGroups.mockResolvedValue({ data: [] });
    api.getGroupCategories.mockResolvedValue({ data: [] });
    api.getSubmissions.mockResolvedValue({ data: [] });
    api.listPeerReviews.mockResolvedValue([]);
    api.getOutcomeAlignments.mockResolvedValue([]);
    api.getCourseOutcomes.mockResolvedValue([]);
    api.getCourseFiles.mockResolvedValue({ data: [] });
    api.getAssignmentRubric.mockRejectedValue(new Error('none'));
  });

  it('saves availability dates, attempts and moderated grading', async () => {
    api.getAssignment.mockResolvedValue({
      id: 2, name: 'Lab report', points_possible: 10, submission_types: ['online_upload'],
      allowed_attempts: null, allowed_extensions: [], moderated_grading: false,
    });
    api.updateAssignment.mockImplementation((_c, _a, payload) => Promise.resolve({ id: 2, ...payload }));
    render(
      <MemoryRouter initialEntries={['/courses/1/assignments/2']}>
        <Routes><Route path="/courses/:courseId/assignments/:assignmentId" element={<AssignmentPage />} /></Routes>
      </MemoryRouter>,
    );
    fireEvent.click(await screen.findByText('Edit'));
    fireEvent.change(screen.getByLabelText('Available from'), { target: { value: '2026-11-01T08:00' } });
    fireEvent.change(screen.getByLabelText('Until'), { target: { value: '2026-11-30T17:00' } });
    fireEvent.click(screen.getByLabelText('Limit submission attempts'));
    fireEvent.change(screen.getByLabelText('Allowed attempts'), { target: { value: '2' } });
    fireEvent.change(screen.getByLabelText('Allowed file extensions'), { target: { value: 'pdf, docx' } });
    fireEvent.click(screen.getByLabelText(/Several graders/));
    fireEvent.change(screen.getByLabelText('Moderator (final grader)'), { target: { value: '11' } });
    fireEvent.click(screen.getByText('Save Changes'));
    await waitFor(() => expect(api.updateAssignment).toHaveBeenCalled());
    const payload = api.updateAssignment.mock.calls[0][2];
    expect(payload.unlock_at).toBe(new Date('2026-11-01T08:00').toISOString());
    expect(payload.lock_at).toBe(new Date('2026-11-30T17:00').toISOString());
    expect(payload.allowed_attempts).toBe(2);
    expect(payload.allowed_extensions).toEqual(['pdf', 'docx']);
    expect(payload.moderated_grading).toBe(true);
    expect(payload.final_grader_id).toBe(11);
    expect(await screen.findByText(/2 attempts allowed/)).toBeInTheDocument();
    expect(screen.getByText('Moderate')).toBeInTheDocument();
  });
});
