import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import GroupsPage from '../GroupsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getGroupCategories: vi.fn(),
    getGroupsByCategory: vi.fn(),
    getGroupMemberships: vi.fn(),
    getAllEnrollments: vi.fn(),
    deleteGroupMembership: vi.fn(),
    createGroupMembership: vi.fn(),
    createGroupCategory: vi.fn(),
    // api_groups_conferences.js goes through api.request (G100).
    request: vi.fn(),
  },
  requestRaw: vi.fn(),
  uploadFile: vi.fn(),
}));

let mockUnassigned = [];
function routeRequest(path) {
  if (/\/group_categories\/\d+\/users\?unassigned=true$/.test(path)) return Promise.resolve({ data: mockUnassigned });
  if (path.endsWith('/assign_unassigned_members')) return Promise.resolve({ data: { assigned: mockUnassigned.length, memberships: [] } });
  return Promise.resolve({ data: {} });
}

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
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/courses/5/groups']}>
        <Routes>
          <Route path="/courses/:courseId/groups" element={<GroupsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function mockGroupData(selfSignup = '', members = [{ id: 300, group_id: 21, user_id: 51, user: { id: 51, name: 'John Doe' } }]) {
  api.getGroupCategories.mockResolvedValue({ data: [{ id: 7, name: 'Project teams', self_signup: selfSignup }] });
  api.getGroupsByCategory.mockResolvedValue({ data: [{ id: 21, group_category_id: 7, name: 'Team A' }] });
  api.getGroupMemberships.mockResolvedValue({ data: members });
  api.getAllEnrollments.mockResolvedValue({ data: [] });
}

describe('GroupsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = true;
    mockUnassigned = [];
    api.request.mockImplementation(routeRequest);
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

  // F20: the Add member picker draws on the whole roster, not the first 10.
  test('loads the whole roster for the add-member picker', async () => {
    mockGroupData();
    renderPage();
    await waitFor(() => expect(api.getAllEnrollments).toHaveBeenCalledWith('5'));
  });

  // G100: the group-set toolbar shows unassigned students and auto-assigns.
  test('teacher sees unassigned students and can auto-assign them', async () => {
    mockGroupData();
    api.getGroupCategories.mockResolvedValue({ data: [{ id: 7, course_id: 5, name: 'Project teams', self_signup: '' }] });
    mockUnassigned = [{ id: 60, name: 'Jo Example' }, { id: 61, name: 'Lee Sample' }];

    renderPage();
    const toggle = await screen.findByRole('button', { name: /2 unassigned/ });
    fireEvent.click(toggle);
    expect(screen.getByText('Jo Example')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Keep each group to one section'));
    fireEvent.click(screen.getByRole('button', { name: /Auto-assign/ }));
    await waitFor(() => expect(api.request).toHaveBeenCalledWith(
      '/group_categories/7/assign_unassigned_members',
      { method: 'POST', body: JSON.stringify({ group_by_section: true }) },
    ));
  });

  test('students do not get the group-set toolbar', async () => {
    mockIsTeacher = false;
    mockGroupData('enabled', []);
    api.getGroupCategories.mockResolvedValue({ data: [{ id: 7, course_id: 5, name: 'Project teams', self_signup: 'enabled' }] });
    renderPage();
    await waitFor(() => expect(screen.getByText('Team A')).toBeInTheDocument());
    expect(screen.queryByTestId('group-set-tools-7')).not.toBeInTheDocument();
  });

  // G100: leaders are marked, and a teacher can hand leadership over.
  test('teacher makes a member the group leader', async () => {
    mockGroupData();
    api.getGroupsByCategory.mockResolvedValue({ data: [{ id: 21, group_category_id: 7, name: 'Team A', leader_id: null }] });
    renderPage();
    await waitFor(() => expect(screen.getByText('John Doe')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Make leader'));
    await waitFor(() => expect(api.request).toHaveBeenCalledWith('/groups/21', {
      method: 'PUT', body: JSON.stringify({ group: { leader_id: 51 } }),
    }));
  });

  test('the current leader is marked and can be removed as leader', async () => {
    mockGroupData();
    api.getGroupsByCategory.mockResolvedValue({ data: [{ id: 21, group_category_id: 7, name: 'Team A', leader_id: 51 }] });
    renderPage();
    await waitFor(() => expect(screen.getByLabelText('Group leader')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Remove as leader'));
    await waitFor(() => expect(api.request).toHaveBeenCalledWith('/groups/21', {
      method: 'PUT', body: JSON.stringify({ group: { leader_id: null } }),
    }));
  });

  test('creating a set can create and fill its groups', async () => {
    mockGroupData();
    api.createGroupCategory.mockResolvedValue({ id: 8 });
    renderPage();
    await waitFor(() => expect(screen.getByText('Team A')).toBeInTheDocument());
    fireEvent.click(screen.getByText('New Category'));
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Lab partners' } });
    fireEvent.change(screen.getByLabelText('Create groups now'), { target: { value: '4' } });
    fireEvent.click(screen.getByLabelText('Assign students to the new groups automatically'));
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    await waitFor(() => expect(api.createGroupCategory).toHaveBeenCalled());
    expect(api.createGroupCategory.mock.calls[0][1]).toMatchObject({
      name: 'Lab partners', create_group_count: 4, assign_unassigned_members: true,
    });
  });
});
