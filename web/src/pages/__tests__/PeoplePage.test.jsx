import React from 'react';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import PeoplePage from '../PeoplePage';
import CourseInvitationsBanner from '../../components/enrollments/CourseInvitationsBanner';
import { enrollmentsAdminApi } from '../../services/api_admin_enrollments';

// People page lifecycle actions (G105), section names (G109) and the
// invitations banner (G122). All data is made up.

vi.mock('../../services/api_admin_enrollments', () => ({
  enrollmentsAdminApi: {
    listRoster: vi.fn(),
    listSections: vi.fn(),
    applyTask: vi.fn(),
    reactivate: vi.fn(),
    updateEnrollment: vi.fn(),
    createEnrollment: vi.fn(),
    listInvitations: vi.fn(),
    acceptInvitation: vi.fn(),
    rejectInvitation: vi.fn(),
  },
}));

const mockRole = { current: { staff: true, teacher: true } };
vi.mock('../../hooks/useCourseStaffRole', () => ({ default: () => mockRole.current }));
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: 'Terry Teacher', role: 'user' } }),
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));

const roster = [
  { id: 10, user_id: 1, type: 'TeacherEnrollment', enrollment_state: 'active', course_section_id: null, user: { id: 1, name: 'Terry Teacher' } },
  { id: 11, user_id: 2, type: 'StudentEnrollment', enrollment_state: 'active', course_section_id: 4, user: { id: 2, name: 'Sam Student' } },
  { id: 12, user_id: 3, type: 'StudentEnrollment', enrollment_state: 'inactive', course_section_id: 4, user: { id: 3, name: 'Bea Student' } },
];

function renderPeople() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/courses/9/people']}>
        <Routes>
          <Route path="/courses/:courseId/people" element={<PeoplePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('PeoplePage lifecycle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRole.current = { staff: true, teacher: true };
    enrollmentsAdminApi.listRoster.mockResolvedValue(roster);
    enrollmentsAdminApi.listSections.mockResolvedValue([{ id: 4, name: 'Evening' }]);
    enrollmentsAdminApi.applyTask.mockResolvedValue({});
    enrollmentsAdminApi.reactivate.mockResolvedValue({});
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  test('shows section names and statuses, and fetches current people by default', async () => {
    renderPeople();
    await screen.findByText('Sam Student');
    expect(enrollmentsAdminApi.listRoster).toHaveBeenCalledWith('9', ['active', 'invited']);
    const sam = screen.getByText('Sam Student').closest('tr');
    expect(within(sam).getByText('Evening')).toBeInTheDocument();
    expect(within(sam).getByText('Active')).toBeInTheDocument();
    const bea = screen.getByText('Bea Student').closest('tr');
    expect(within(bea).getByText('Inactive')).toBeInTheDocument();
    // The teacher's own row has no manage menu.
    expect(screen.queryByLabelText('Manage Terry Teacher')).not.toBeInTheDocument();
  });

  test('concluding a student calls the conclude task', async () => {
    renderPeople();
    fireEvent.click(await screen.findByLabelText('Manage Sam Student'));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Conclude' }));
    await waitFor(() => expect(enrollmentsAdminApi.applyTask).toHaveBeenCalledWith('9', 11, 'conclude'));
    await waitFor(() => expect(enrollmentsAdminApi.listRoster.mock.calls.length).toBeGreaterThan(1));
  });

  test('an inactive student can be re-activated', async () => {
    renderPeople();
    fireEvent.click(await screen.findByLabelText('Manage Bea Student'));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Re-activate' }));
    await waitFor(() => expect(enrollmentsAdminApi.reactivate).toHaveBeenCalledWith('9', 12));
  });

  test('the status filter asks the server for that state', async () => {
    renderPeople();
    await screen.findByText('Sam Student');
    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'concluded' } });
    await waitFor(() => expect(enrollmentsAdminApi.listRoster).toHaveBeenCalledWith('9', ['completed']));
  });

  test('students get no manage menus or status filter', async () => {
    mockRole.current = { staff: false, teacher: false };
    renderPeople();
    await screen.findByText('Sam Student');
    expect(enrollmentsAdminApi.listRoster).toHaveBeenCalledWith('9', []);
    expect(screen.queryByLabelText(/^Manage /)).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Status')).not.toBeInTheDocument();
    expect(screen.queryByText('Add People')).not.toBeInTheDocument();
  });

  test('a remove that the server refuses shows the error', async () => {
    enrollmentsAdminApi.applyTask.mockRejectedValue(new Error('You are not allowed to manage this enrollment'));
    renderPeople();
    fireEvent.click(await screen.findByLabelText('Manage Sam Student'));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove from course' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('not allowed');
    expect(enrollmentsAdminApi.applyTask).toHaveBeenCalledWith('9', 11, 'delete');
  });
});

describe('CourseInvitationsBanner', () => {
  beforeEach(() => vi.clearAllMocks());

  test('lists invitations and accepts one', async () => {
    enrollmentsAdminApi.listInvitations.mockResolvedValue([
      { id: 5, course_id: 9, course_name: 'Biology', type: 'StudentEnrollment' },
    ]);
    enrollmentsAdminApi.acceptInvitation.mockResolvedValue({ success: true });
    const onAccepted = vi.fn();
    render(<CourseInvitationsBanner onAccepted={onAccepted} />);
    expect(await screen.findByText('Biology')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }));
    await waitFor(() => expect(enrollmentsAdminApi.acceptInvitation).toHaveBeenCalledWith(9, 5));
    await waitFor(() => expect(screen.queryByText('Biology')).not.toBeInTheDocument());
    expect(onAccepted).toHaveBeenCalled();
  });

  test('declines an invitation', async () => {
    enrollmentsAdminApi.listInvitations.mockResolvedValue([{ id: 6, course_id: 8, course_name: 'Chemistry', type: 'TaEnrollment' }]);
    enrollmentsAdminApi.rejectInvitation.mockResolvedValue({ success: true });
    render(<CourseInvitationsBanner />);
    expect(await screen.findByText(/as a ta/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Decline' }));
    await waitFor(() => expect(enrollmentsAdminApi.rejectInvitation).toHaveBeenCalledWith(8, 6));
  });

  test('renders nothing when the lookup fails', async () => {
    enrollmentsAdminApi.listInvitations.mockRejectedValue(new Error('boom'));
    const { container } = render(<CourseInvitationsBanner />);
    await waitFor(() => expect(enrollmentsAdminApi.listInvitations).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});
