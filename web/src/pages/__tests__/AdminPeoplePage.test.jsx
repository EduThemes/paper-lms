import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AdminPeoplePage from '../AdminPeoplePage';

// B175: a super_admin row must render as "Super admin" and be locked for
// account admins (it used to fall through to "User", so any change
// demoted the platform operator).

const mutate = vi.fn();
let currentRole = 'admin';

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: 'Ada Admin', role: currentRole }, loading: false }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../services/apiQueries', () => ({
  useUsersList: () => ({
    isLoading: false,
    isError: false,
    data: {
      data: [
        { id: 1, name: 'Ada Admin', email: 'ada@example.com', role: 'admin' },
        { id: 2, name: 'Sam Super', email: 'sam@example.com', role: 'super_admin' },
        { id: 3, name: 'Tia Teacher', email: 'tia@example.com', role: 'teacher' },
      ],
    },
    refetch: vi.fn(),
  }),
  useUpdateUserRole: () => ({ mutate, isPending: false, isError: false }),
}));

const bulkMutate = vi.fn();
vi.mock('../../services/adminAccountsQueries', () => ({
  useBulkSuspend: () => ({ mutate: bulkMutate, isPending: false }),
}));

vi.mock('../../components/admin/AddUserForm', () => ({
  default: () => <form aria-label="Add a user" />,
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <AdminPeoplePage />
    </MemoryRouter>,
  );
}

describe('AdminPeoplePage super_admin rows', () => {
  beforeEach(() => {
    mutate.mockReset();
  });

  it('shows a super_admin as Super admin and locks the select for an account admin', () => {
    currentRole = 'admin';
    renderPage();
    const select = screen.getByLabelText('Change role for Sam Super');
    expect(select.value).toBe('super_admin');
    expect(select).toBeDisabled();
    expect(screen.getByLabelText('Change role for Tia Teacher')).not.toBeDisabled();
  });

  it('lets a super_admin change another super_admin', () => {
    currentRole = 'super_admin';
    renderPage();
    expect(screen.getByLabelText('Change role for Sam Super')).not.toBeDisabled();
  });
});

describe('AdminPeoplePage user management (G114)', () => {
  it('links each name to the user detail page and bulk-suspends the selection', async () => {
    currentRole = 'admin';
    bulkMutate.mockReset();
    const { fireEvent } = await import('@testing-library/react');
    renderPage();
    expect(screen.getByRole('link', { name: 'Tia Teacher' })).toHaveAttribute('href', '/admin/users/3');
    // A super_admin row can't be selected by an account admin; self neither.
    expect(screen.getByLabelText('Select Sam Super')).toBeDisabled();
    expect(screen.getByLabelText('Select Ada Admin')).toBeDisabled();
    fireEvent.click(screen.getByLabelText('Select Tia Teacher'));
    fireEvent.click(screen.getByRole('button', { name: 'Suspend' }));
    expect(bulkMutate).toHaveBeenCalledWith({ userIds: [3], suspended: true });
  });

  it('opens the add-user form', async () => {
    const { fireEvent } = await import('@testing-library/react');
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /Add user/ }));
    expect(screen.getByRole('form', { name: 'Add a user' })).toBeInTheDocument();
  });
});
