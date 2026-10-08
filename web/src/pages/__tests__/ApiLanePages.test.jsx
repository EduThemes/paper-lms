import React from 'react';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AccessTokensPage from '../AccessTokensPage';
import OAuthConsentPage from '../OAuthConsentPage';
import { api } from '../../services/api';
import * as apiLane from '../../services/api_api';

// G143 token lifecycle, G136 PKCE pass-through. (G138 account reports
// are covered with the admin page: AccountReportsPage.test.jsx.)

let currentUser = { id: 7, name: 'Ada Admin', role: 'admin' };

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: currentUser, loading: false }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../services/api', () => ({
  api: {
    getEnrollmentTerms: vi.fn(),
    getAccessTokens: vi.fn(),
    createAccessToken: vi.fn(),
    deleteAccessToken: vi.fn(),
    getOAuthAuthorizeInfo: vi.fn(),
    submitOAuthConsent: vi.fn(),
  },
}));

vi.mock('../../services/api_api', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...actual,
    listAccountUsers: vi.fn(),
    listUserTokens: vi.fn(),
    updateAccessToken: vi.fn(),
    deleteUserToken: vi.fn(),
  };
});

function renderWithQuery(ui, path = '/') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('AccessTokensPage (G143)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentUser = { id: 7, name: 'Ada Admin', role: 'admin' };
  });

  it('creates a token with an expiry date', async () => {
    api.getAccessTokens.mockResolvedValue({ data: [] });
    api.createAccessToken.mockResolvedValue({ id: 1, token: 'abc123', kind: 'personal' });
    renderWithQuery(<AccessTokensPage />);

    fireEvent.click(await screen.findByRole('button', { name: /Generate New Token/ }));
    fireEvent.change(screen.getByLabelText(/Purpose/), { target: { value: 'Grade sync' } });
    fireEvent.change(screen.getByLabelText('Expires'), { target: { value: '2030-01-15' } });
    fireEvent.click(screen.getByRole('button', { name: 'Generate Token' }));

    await waitFor(() => expect(api.createAccessToken).toHaveBeenCalled());
    const [, token] = api.createAccessToken.mock.calls[0];
    expect(token.purpose).toBe('Grade sync');
    expect(new Date(token.expires_at).getFullYear()).toBe(2030);
    expect(await screen.findByText('abc123')).toBeInTheDocument();
  });

  it('regenerates an expired token only with a new expiry', async () => {
    api.getAccessTokens.mockResolvedValue({
      data: [{ id: 5, purpose: 'Old script', kind: 'personal', workflow_state: 'expired', expires_at: '2020-01-01T00:00:00Z', can_manually_regenerate: true, scopes: [] }],
    });
    apiLane.updateAccessToken.mockResolvedValue({ id: 5, token: 'fresh-secret', kind: 'personal' });
    renderWithQuery(<AccessTokensPage />);

    expect(await screen.findByText('Expired')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate Old script' }));
    const dialog = screen.getByRole('dialog', { name: 'Regenerate Token' });
    const confirm = within(dialog).getByRole('button', { name: 'Regenerate' });
    expect(confirm).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/New expiry date/), { target: { value: '2031-02-01' } });
    fireEvent.click(confirm);
    await waitFor(() => expect(apiLane.updateAccessToken).toHaveBeenCalled());
    const [userId, tokenId, body] = apiLane.updateAccessToken.mock.calls[0];
    expect([userId, tokenId, body.regenerate]).toEqual([7, 5, true]);
    expect(body.expires_at).toBeTruthy();
    expect(await screen.findByText('fresh-secret')).toBeInTheDocument();
  });

  it('lets an admin find a user and revoke their token', async () => {
    api.getAccessTokens.mockResolvedValue({ data: [] });
    apiLane.listAccountUsers.mockResolvedValue([{ id: 20, name: 'Jane Doe', login_id: 'jane@example.test' }]);
    apiLane.listUserTokens.mockResolvedValue([{ id: 41, purpose: 'Sync', kind: 'personal', scopes: [], can_manually_regenerate: false }]);
    apiLane.deleteUserToken.mockResolvedValue({ delete: true });
    renderWithQuery(<AccessTokensPage />);

    fireEvent.change(await screen.findByLabelText('Search people'), { target: { value: 'jane' } });
    fireEvent.click(await screen.findByRole('button', { name: /Jane Doe/ }, { timeout: 2000 }));
    expect(apiLane.listAccountUsers).toHaveBeenCalledWith('self', { search_term: 'jane', include: ['email'] });
    const revoke = await screen.findByRole('button', { name: 'Delete Sync' });
    expect(screen.queryByRole('button', { name: 'Regenerate Sync' })).not.toBeInTheDocument();
    fireEvent.click(revoke);
    fireEvent.click(within(screen.getByRole('dialog', { name: 'Revoke Token' })).getByRole('button', { name: 'Revoke Token' }));
    await waitFor(() => expect(apiLane.deleteUserToken).toHaveBeenCalledWith(20, 41));
  });

  it('hides the admin section from other users', async () => {
    currentUser = { id: 8, name: 'Sam Student', role: 'user' };
    api.getAccessTokens.mockResolvedValue({ data: [] });
    renderWithQuery(<AccessTokensPage />);
    await screen.findByText('No Access Tokens');
    expect(screen.queryByLabelText('Search people')).not.toBeInTheDocument();
  });
});

describe('OAuthConsentPage PKCE (G136)', () => {
  it('carries the code challenge to the consent decision', async () => {
    api.getOAuthAuthorizeInfo.mockResolvedValue({ client_name: 'Example App', redirect_uri: 'https://app.example.com/cb', scopes: [] });
    api.submitOAuthConsent.mockResolvedValue({});
    renderWithQuery(
      <OAuthConsentPage />,
      '/login/oauth2/confirm?client_id=dk_x&redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&code_challenge=abcABC&code_challenge_method=S256',
    );
    fireEvent.click(await screen.findByRole('button', { name: 'Authorize' }));
    await waitFor(() =>
      expect(api.submitOAuthConsent).toHaveBeenCalledWith(
        expect.objectContaining({ approve: true, code_challenge: 'abcABC', code_challenge_method: 'S256' }),
      ),
    );
  });
});
