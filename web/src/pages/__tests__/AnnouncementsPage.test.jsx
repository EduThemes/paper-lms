import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AnnouncementsPage from '../AnnouncementsPage';
import { api } from '../../services/api';

// B072: the teacher controls must not depend on the caller's row being on
// the first roster page. Here the roster's first page holds only students
// (as in a SIS-loaded course where the teacher was enrolled last); the
// caller's own enrollment comes from /enrollments/self.

vi.mock('../../services/api', () => ({
  api: {
    request: vi.fn(),
    getEnrollments: vi.fn(),
    getAllEnrollments: vi.fn(),
    getMyCourseEnrollments: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 500, name: 'Terry Teacher', role: 'user' },
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

const firstPageOfStudents = Array.from({ length: 100 }, (_, i) => ({
  id: i + 1, user_id: i + 1, type: 'StudentEnrollment', role: 'StudentEnrollment',
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/7/announcements']}>
      <Routes>
        <Route path="/courses/:courseId/announcements" element={<AnnouncementsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AnnouncementsPage teacher detection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.request.mockResolvedValue({ data: [] });
    api.getEnrollments.mockResolvedValue({ data: firstPageOfStudents });
    api.getAllEnrollments.mockResolvedValue({ data: firstPageOfStudents });
  });

  it('shows the teacher controls when the teacher is not on the first roster page', async () => {
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 501, user_id: 500, type: 'TeacherEnrollment', role: 'TeacherEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    renderPage();
    await waitFor(() => expect(api.getMyCourseEnrollments).toHaveBeenCalledWith('7'));
    await waitFor(() => expect(screen.getByRole('button', { expanded: false })).toBeInTheDocument());
  });

  it('hides the teacher controls from a student', async () => {
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 9, user_id: 500, type: 'StudentEnrollment', role: 'StudentEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    renderPage();
    await waitFor(() => expect(api.getMyCourseEnrollments).toHaveBeenCalled());
    await waitFor(() => expect(api.request).toHaveBeenCalled());
    expect(screen.queryByRole('button', { expanded: false })).not.toBeInTheDocument();
    expect(api.getAllEnrollments).not.toHaveBeenCalled();
  });
});
