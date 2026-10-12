import React from 'react';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import useCourseStaffRole from '../useCourseStaffRole';
import { api } from '../../services/api';

// F18: TAs are staff but not teachers — the course audit log (and course
// delete) are RequireCourseRole("TeacherEnrollment") routes, so the nav
// must not offer them to a TA.

let mockUser = null;
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
}));

vi.mock('../../services/api', () => ({
  api: { getMyCourseEnrollments: vi.fn() },
}));

async function resolveHook(courseId) {
  let value;
  const Probe = () => {
    value = useCourseStaffRole(courseId);
    return null;
  };
  const root = createRoot(document.createElement('div'));
  await act(async () => {
    root.render(<Probe />);
  });
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  act(() => root.unmount());
  return value;
}

describe('useCourseStaffRole', () => {
  beforeAll(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  });
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('marks a TA as staff but not teacher', async () => {
    mockUser = { id: 45, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 601, user_id: 45, type: 'TaEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    expect(await resolveHook(7)).toEqual({ staff: true, teacher: false });
  });

  it('marks a teacher as both', async () => {
    mockUser = { id: 46, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 602, user_id: 46, type: 'TeacherEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    expect(await resolveHook(7)).toEqual({ staff: true, teacher: true });
  });

  it('ignores an inactive teacher enrollment', async () => {
    mockUser = { id: 47, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 603, user_id: 47, type: 'TeacherEnrollment', enrollment_state: 'completed' }],
      is_admin: false,
    });
    expect(await resolveHook(7)).toEqual({ staff: false, teacher: false });
  });

  it('treats admins as teachers', async () => {
    mockUser = { id: 1, role: 'admin' };
    expect(await resolveHook(7)).toEqual({ staff: true, teacher: true });
  });
});
