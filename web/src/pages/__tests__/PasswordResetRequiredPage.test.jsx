import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import PasswordResetRequiredPage from '../PasswordResetRequiredPage';
import { api } from '../../services/api';

// Mock react-router's useNavigate so we can assert the redirect-to-"/"
// happens on a successful set without spinning up the full App.
const navigateMock = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

vi.mock('../../services/api', () => ({
  api: {
    auth: {
      setPassword: vi.fn(),
    },
  },
}));

// AuthContext provides finalizePasswordReset; in real life it calls
// api.auth.setPassword and updates user state. The stub here just
// invokes the api method directly so we can assert the contract.
vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    finalizePasswordReset: async (newPassword) => {
      const pendingToken = sessionStorage.getItem('password_reset_pending_token');
      return api.auth.setPassword({ pendingToken, newPassword });
    },
  }),
}));

vi.mock('../../components/brand/BrandLogo', () => ({
  default: () => <div data-testid="brand-logo" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/auth/password-set']}>
      <PasswordResetRequiredPage />
    </MemoryRouter>,
  );
}

describe('PasswordResetRequiredPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
  });

  test('renders heading, subtitle, and two password fields', () => {
    sessionStorage.setItem('password_reset_pending_token', 'pending-tok');
    renderPage();
    expect(screen.getByText(/Set a new password/i)).toBeInTheDocument();
    expect(screen.getByLabelText('New password')).toBeInTheDocument();
    expect(screen.getByLabelText('Confirm new password')).toBeInTheDocument();
  });

  test('rejects passwords shorter than 8 chars without calling the API', async () => {
    sessionStorage.setItem('password_reset_pending_token', 'pending-tok');
    renderPage();
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'short' } });
    fireEvent.change(screen.getByLabelText('Confirm new password'), { target: { value: 'short' } });
    fireEvent.click(screen.getByRole('button', { name: /Set password and continue/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/at least 8/i);
    });
    expect(api.auth.setPassword).not.toHaveBeenCalled();
  });

  test('rejects mismatched passwords without calling the API', async () => {
    sessionStorage.setItem('password_reset_pending_token', 'pending-tok');
    renderPage();
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'goodpassword1' } });
    fireEvent.change(screen.getByLabelText('Confirm new password'), { target: { value: 'differentpw1' } });
    fireEvent.click(screen.getByRole('button', { name: /Set password and continue/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/do not match/i);
    });
    expect(api.auth.setPassword).not.toHaveBeenCalled();
  });

  test('happy path: valid matching password POSTs and navigates to "/"', async () => {
    sessionStorage.setItem('password_reset_pending_token', 'pending-tok');
    api.auth.setPassword.mockResolvedValue({ token: 'new-session', user: { id: 1 } });
    renderPage();
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'goodpassword1' } });
    fireEvent.change(screen.getByLabelText('Confirm new password'), { target: { value: 'goodpassword1' } });
    fireEvent.click(screen.getByRole('button', { name: /Set password and continue/i }));
    await waitFor(() => {
      expect(api.auth.setPassword).toHaveBeenCalledWith({
        pendingToken: 'pending-tok',
        newPassword: 'goodpassword1',
      });
    });
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/');
    });
  });

  test('missing pending token shows clear "log in again" error', () => {
    renderPage();
    expect(screen.getByRole('alert')).toHaveTextContent(/No pending password-reset/i);
    // Buttons are disabled when there's no pending token.
    expect(screen.getByRole('button', { name: /Set password and continue/i })).toBeDisabled();
  });

  test('surfaces backend error and clears in-flight state', async () => {
    sessionStorage.setItem('password_reset_pending_token', 'pending-tok');
    api.auth.setPassword.mockRejectedValue(new Error('pending token invalid or expired'));
    renderPage();
    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'goodpassword1' } });
    fireEvent.change(screen.getByLabelText('Confirm new password'), { target: { value: 'goodpassword1' } });
    fireEvent.click(screen.getByRole('button', { name: /Set password and continue/i }));
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/invalid or expired/i);
    });
    expect(navigateMock).not.toHaveBeenCalledWith('/');
  });
});
