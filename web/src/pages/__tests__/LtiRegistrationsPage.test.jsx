import React from 'react';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import LtiRegistrationsPage from '../LtiRegistrationsPage';
import { ltiApi } from '../../services/api_lti';

vi.mock('../../services/api_lti', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...actual,
    ltiApi: {
      listRegistrations: vi.fn(),
      createRegistration: vi.fn(),
      updateRegistration: vi.fn(),
      deleteRegistration: vi.fn(),
      setAccountInstall: vi.fn(),
      platformInfo: vi.fn(),
      rotatePlatformKey: vi.fn(),
      dynamicRegistrationToken: vi.fn(),
      listAccountTools: vi.fn(),
      createAccountTool: vi.fn(),
      deleteAccountTool: vi.fn(),
    },
  };
});
vi.mock('../../hooks/useAccountId', () => ({ default: () => 42 }));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => ({ user: { id: 1, role: 'super_admin' } }) }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));

const reg = {
  id: 3,
  client_id: 'dk_example',
  name: 'Example Tool',
  workflow_state: 'active',
  redirect_uris: ['https://tool.example.com/launch'],
  tool_configuration: {
    title: 'Example Tool',
    target_link_uri: 'https://tool.example.com/launch',
    oidc_initiation_url: 'https://tool.example.com/login',
    public_jwk_url: 'https://tool.example.com/jwks',
    privacy_level: 'anonymous',
    custom_fields: {},
    placements: [{ placement: 'course_navigation' }],
    scopes: [],
  },
  account_install: null,
  deployment_id: null,
};

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <LtiRegistrationsPage />
    </QueryClientProvider>
  );
}

describe('LtiRegistrationsPage (G123, G126, G132)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    ltiApi.listRegistrations.mockResolvedValue([reg]);
    ltiApi.listAccountTools.mockResolvedValue([]);
    ltiApi.platformInfo.mockResolvedValue({
      issuer: 'https://lms.example.com',
      jwks_uri: 'https://lms.example.com/api/v1/lti/jwks',
      keys: { current_kid: 'kid-1', published_kids: ['kid-1', 'kid-2'], rotatable: true },
    });
  });

  test('lists registrations of the caller\'s account with platform details', async () => {
    renderPage();
    expect(await screen.findByText('dk_example')).toBeTruthy();
    expect(ltiApi.listRegistrations).toHaveBeenCalledWith(42);
    expect(await screen.findByText('https://lms.example.com/api/v1/lti/jwks')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Rotate key/ })).toBeTruthy();
  });

  test('registers a tool with placements and services', async () => {
    ltiApi.createRegistration.mockResolvedValue({ ...reg, id: 4 });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /Register tool/ }));
    fireEvent.change(screen.getByLabelText(/Tool name/), { target: { value: 'New Tool' } });
    fireEvent.change(screen.getByLabelText(/Target link URI/), { target: { value: 'https://new.example.com/launch' } });
    fireEvent.change(screen.getByLabelText(/OpenID Connect login URL/), { target: { value: 'https://new.example.com/login' } });
    fireEvent.change(screen.getByLabelText(/Public JWK set URL/), { target: { value: 'https://new.example.com/jwks' } });
    fireEvent.click(screen.getByLabelText('Rich content editor button'));
    fireEvent.click(screen.getByLabelText('AGS: post scores'));
    fireEvent.click(screen.getByRole('button', { name: 'Save tool' }));

    await waitFor(() => expect(ltiApi.createRegistration).toHaveBeenCalled());
    const [accountId, payload] = ltiApi.createRegistration.mock.calls[0];
    expect(accountId).toBe(42);
    expect(payload.tool_configuration).toMatchObject({
      title: 'New Tool',
      target_link_uri: 'https://new.example.com/launch',
      public_jwk_url: 'https://new.example.com/jwks',
      placements: [{ placement: 'editor_button' }],
      scopes: ['https://purl.imsglobal.org/spec/lti-ags/scope/score'],
    });
  });

  test('deploys a tool to every course', async () => {
    ltiApi.setAccountInstall.mockResolvedValue({ ...reg, deployment_id: '9' });
    renderPage();
    const row = (await screen.findByText('dk_example')).closest('tr');
    fireEvent.click(within(row).getByLabelText(/Deploy Example Tool to every course/));
    await waitFor(() => expect(ltiApi.setAccountInstall).toHaveBeenCalledWith(42, 3, true));
  });
});
