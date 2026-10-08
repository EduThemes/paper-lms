import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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

describe('AnnouncementsPage section targeting (G090)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getAllEnrollments.mockResolvedValue({ data: [] });
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 501, user_id: 500, type: 'TeacherEnrollment', role: 'TeacherEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    api.request.mockImplementation(async (url) => {
      if (url.startsWith('/courses/7/sections')) {
        return { data: [{ id: 3, name: 'Period 1' }, { id: 4, name: 'Period 2' }] };
      }
      return { data: [] };
    });
  });

  it('posts to the chosen sections only', async () => {
    renderPage();
    const newButton = await screen.findByRole('button', { expanded: false });
    fireEvent.click(newButton);
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Lab moved' } });
    expect(await screen.findByText('Specific Sections')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Audience'), { target: { value: 'sections' } });

    // No section chosen yet: the form refuses to submit.
    fireEvent.click(screen.getByRole('button', { name: 'Post Announcement' }));
    expect(await screen.findByText('Choose at least one section')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Period 2'));
    fireEvent.click(screen.getByRole('button', { name: 'Post Announcement' }));
    await waitFor(() => {
      const post = api.request.mock.calls.find(([url, opts]) => url === '/courses/7/announcements' && opts?.method === 'POST');
      expect(post).toBeTruthy();
      const body = JSON.parse(post[1].body);
      expect(body.target_audience).toBe('sections');
      expect(body.section_ids).toEqual([4]);
    });
  });
});
