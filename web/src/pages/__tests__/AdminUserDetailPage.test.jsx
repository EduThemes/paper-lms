import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import AdminUserDetailPage from '../AdminUserDetailPage';

// G113 / G114 / G115 / G121 — the admin user detail page. All data is
// made up.

const updateMutate = vi.fn();
const statusMutate = vi.fn();
const loginMutate = vi.fn();
let detail;

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1, role: 'admin' }, refreshUser: vi.fn() }) }));
vi.mock('../../services/api', () => ({ api: { searchUsers: vi.fn(async () => ({ data: [] })), getAuthProviders: vi.fn(async () => ({ data: [] })) } }));
vi.mock('../../services/api_admin_accounts', () => ({ adminAccountsApi: { startMasquerade: vi.fn() } }));
vi.mock('../../services/adminAccountsQueries', () => ({
  useAdminUser: () => ({ isLoading: false, isError: false, data: detail, refetch: vi.fn() }),
  useAdminUserHistory: () => ({ isLoading: false, data: { logins: [{ id: 1, event_type: 'auth.login_succeeded', created_at: '2026-01-01T00:00:00Z', action: 'logged_in' }], activity: [], grade_changes: [] } }),
  useUpdateAdminUser: () => ({ mutate: updateMutate, isPending: false }),
  useUserStatusMutation: () => ({ mutate: statusMutate, isPending: false }),
  useLoginMutation: () => ({ mutate: loginMutate, isPending: false }),
  useObserverLinkMutation: () => ({ mutate: vi.fn(), isPending: false }),
}));

function baseDetail(overrides = {}) {
  return {
    id: 5, name: 'Jane Doe', short_name: 'Jane', sortable_name: 'Doe, Jane', email: 'jane@example.com',
    login_id: 'jane.doe', sis_user_id: 'J-1', time_zone: 'America/New_York', role: 'user',
    suspended: false, requires_password_reset: false,
    enrollments: [{ id: 9, course_id: 3, course_name: 'Biology', course_code: 'BIO1', type: 'StudentEnrollment', role: 'StudentEnrollment', enrollment_state: 'active' }],
    logins: [{ id: 11, unique_id: 'jdoe-alt', has_password: true, workflow_state: 'active' }],
    observees: [], observers: [{ id: 6, name: 'Pat Parent' }],
    permissions: { can_manage: true, can_masquerade: true },
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/admin/users/5']}>
      <Routes><Route path="/admin/users/:userId" element={<AdminUserDetailPage />} /></Routes>
    </MemoryRouter>,
  );
}

describe('AdminUserDetailPage', () => {
  beforeEach(() => {
    detail = baseDetail();
    updateMutate.mockReset();
    statusMutate.mockReset();
    loginMutate.mockReset();
  });

  it('shows enrollments, logins, observers, history and the act-as action', () => {
    renderPage();
    expect(screen.getByRole('heading', { name: 'Jane Doe' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Biology' })).toHaveAttribute('href', '/courses/3');
    expect(screen.getByText('jdoe-alt')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Pat Parent' })).toHaveAttribute('href', '/admin/users/6');
    expect(screen.getByText('auth.login_succeeded')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Act as user/ })).toBeInTheDocument();
  });

  it('saves only the changed profile fields', () => {
    renderPage();
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'jane.d@example.com' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save profile' }));
    expect(updateMutate).toHaveBeenCalledWith({ email: 'jane.d@example.com' });
  });

  it('suspends and adds logins', () => {
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Suspend' }));
    expect(statusMutate).toHaveBeenCalledWith({ action: 'suspend' });
    fireEvent.click(screen.getByRole('button', { name: 'Suspend login jdoe-alt' }));
    expect(loginMutate).toHaveBeenCalledWith({ action: 'update', loginId: 11, login: { workflow_state: 'suspended' } });
    fireEvent.change(screen.getByLabelText('New login'), { target: { value: 'jane-second' } });
    fireEvent.click(screen.getByRole('button', { name: /Add login/ }));
    expect(loginMutate).toHaveBeenCalledWith({ action: 'create', login: { unique_id: 'jane-second' } });
  });

  it('locks editing and hides act-as when the caller may not manage the user', () => {
    detail = baseDetail({ role: 'super_admin', permissions: { can_manage: false, can_masquerade: false } });
    renderPage();
    expect(screen.queryByRole('button', { name: /Act as user/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Suspend' })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Email')).toBeDisabled();
  });
});
