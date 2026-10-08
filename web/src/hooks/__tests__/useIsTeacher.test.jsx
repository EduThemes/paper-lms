import React from 'react';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import useIsTeacher from '../useIsTeacher';
import { api } from '../../services/api';

// B072: teacher detection must not depend on the caller's row appearing
// in the first page of the roster, and super_admin counts as staff.

let mockUser = null;
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: mockUser }),
}));

vi.mock('../../services/api', () => ({
  api: {
    getMyCourseEnrollments: vi.fn(),
    getEnrollments: vi.fn(),
  },
}));

async function resolveHook(courseId) {
  let value;
  const Probe = () => {
    value = useIsTeacher(courseId);
    return null;
  };
  const root = createRoot(document.createElement('div'));
  await act(async () => {
    root.render(<Probe />);
  });
  // Let the promise chain settle.
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  act(() => root.unmount());
  return value;
}

describe('useIsTeacher', () => {
  beforeAll(() => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  });
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('recognises a teacher from the self-enrollment endpoint without scanning the roster', async () => {
    mockUser = { id: 42, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 501, user_id: 42, type: 'TeacherEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    expect(await resolveHook(7)).toBe(true);
    expect(api.getMyCourseEnrollments).toHaveBeenCalledWith(7);
    expect(api.getEnrollments).not.toHaveBeenCalled();
  });

  it('returns false for a student', async () => {
    mockUser = { id: 43, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({
      enrollments: [{ id: 502, user_id: 43, type: 'StudentEnrollment', enrollment_state: 'active' }],
      is_admin: false,
    });
    expect(await resolveHook(7)).toBe(false);
  });

  it('treats super_admin as staff', async () => {
    mockUser = { id: 1, role: 'super_admin' };
    expect(await resolveHook(7)).toBe(true);
    expect(api.getMyCourseEnrollments).not.toHaveBeenCalled();
  });

  it('honours the server-reported admin flag', async () => {
    mockUser = { id: 44, role: 'user' };
    api.getMyCourseEnrollments.mockResolvedValue({ enrollments: [], is_admin: true });
    expect(await resolveHook(7)).toBe(true);
  });
});
