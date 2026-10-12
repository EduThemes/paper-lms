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
