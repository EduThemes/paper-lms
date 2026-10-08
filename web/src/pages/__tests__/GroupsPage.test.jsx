import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import GroupsPage from '../GroupsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getGroupCategories: vi.fn(),
    getGroupsByCategory: vi.fn(),
    getGroupMemberships: vi.fn(),
    getEnrollments: vi.fn(),
    deleteGroupMembership: vi.fn(),
    createGroupMembership: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 50, name: 'Jane Doe', role: 'user' }, loading: false }),
}));

let mockIsTeacher = true;
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => mockIsTeacher,
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/5/groups']}>
      <Routes>
        <Route path="/courses/:courseId/groups" element={<GroupsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

function mockGroupData(selfSignup = '', members = [{ id: 300, group_id: 21, user_id: 51, user: { id: 51, name: 'John Doe' } }]) {
  api.getGroupCategories.mockResolvedValue({ data: [{ id: 7, name: 'Project teams', self_signup: selfSignup }] });
  api.getGroupsByCategory.mockResolvedValue({ data: [{ id: 21, group_category_id: 7, name: 'Team A' }] });
  api.getGroupMemberships.mockResolvedValue({ data: members });
  api.getEnrollments.mockResolvedValue({ data: [] });
}

describe('GroupsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = true;
  });

  // B162: deleteGroupMembership takes (groupId, membershipId).
  test('removing a member sends the group id and the membership id', async () => {
    mockGroupData();
    api.deleteGroupMembership.mockResolvedValue({ delete: true });
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderPage();
    await waitFor(() => expect(screen.getByText('John Doe')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Remove member'));

    await waitFor(() => expect(api.deleteGroupMembership).toHaveBeenCalled());
    expect(api.deleteGroupMembership).toHaveBeenCalledWith(21, 300);
  });

  // B163: students can join a self sign-up group themselves.
  test('student sees Join for a self sign-up group and joins as themselves', async () => {
    mockIsTeacher = false;
    mockGroupData('enabled', []);
    api.createGroupMembership.mockResolvedValue({ id: 301 });

    renderPage();
    await waitFor(() => expect(screen.getByText('Team A')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Join' }));

    await waitFor(() => expect(api.createGroupMembership).toHaveBeenCalledWith(21, {}));
  });

  test('no Join button when self sign-up is off', async () => {
    mockIsTeacher = false;
    mockGroupData('', []);
    renderPage();
    await waitFor(() => expect(screen.getByText('Team A')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: 'Join' })).not.toBeInTheDocument();
  });
});
