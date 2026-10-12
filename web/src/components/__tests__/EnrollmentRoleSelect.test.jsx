import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import EnrollmentRoleSelect from '../admin/EnrollmentRoleSelect';

// G102 — the Add People picker offers the account's custom course roles
// on top of their base type. All data is made up.

vi.mock('../../services/api_admin_accounts', () => ({
  adminAccountsApi: {
    getAssignableRoles: vi.fn(async () => [
      { id: 7, name: 'Grader', label: 'Grader', base_role_type: 'ta', enrollment_type: 'TaEnrollment' },
    ]),
  },
}));

describe('EnrollmentRoleSelect', () => {
  it('lists custom roles and reports Type|roleId', async () => {
    const onChange = vi.fn();
    render(<EnrollmentRoleSelect courseId={3} value="StudentEnrollment" onChange={onChange} />);
    await waitFor(() => expect(screen.getByRole('option', { name: 'Grader' })).toBeInTheDocument());
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'TaEnrollment|7' } });
    expect(onChange).toHaveBeenCalledWith('TaEnrollment|7');
  });
});
