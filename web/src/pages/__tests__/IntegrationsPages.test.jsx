import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import DeveloperKeysPage from '../DeveloperKeysPage';
import ExternalToolsPage from '../ExternalToolsPage';
import OAuthConsentPage from '../OAuthConsentPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getDeveloperKeys: vi.fn(),
    createDeveloperKey: vi.fn(),
    getCourse: vi.fn(),
    getExternalTools: vi.fn(),
    createExternalTool: vi.fn(),
    updateExternalTool: vi.fn(),
    getOAuthAuthorizeInfo: vi.fn(),
    submitOAuthConsent: vi.fn(),
  },
}));

vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

describe('DeveloperKeysPage (B182)', () => {
  beforeEach(() => vi.clearAllMocks());

  test('lists and creates keys in the caller\'s own tenant, not account 1', async () => {
    api.getDeveloperKeys.mockResolvedValue({ data: [] });
    api.createDeveloperKey.mockResolvedValue({ id: 3, api_key: 'dk_x', client_secret: 'secret' });

    const { container } = render(<MemoryRouter><DeveloperKeysPage /></MemoryRouter>);
    await waitFor(() => expect(api.getDeveloperKeys).toHaveBeenCalled());
    expect(api.getDeveloperKeys.mock.calls[0][0]).toBe('self');

    fireEvent.click(screen.getByRole('button', { name: /Add Developer Key/i }));
    fireEvent.change(container.querySelector('input[name="name"]'), { target: { value: 'Example App' } });
    fireEvent.change(container.querySelector('textarea[name="redirect_uris"]'), {
      target: { value: 'https://app.example.com/a\nhttps://app.example.com/b' },
    });
    fireEvent.click(container.querySelector('input[name="require_scopes"]'));
    fireEvent.submit(container.querySelector('form'));

    await waitFor(() => expect(api.createDeveloperKey).toHaveBeenCalled());
    const [accountId, body] = api.createDeveloperKey.mock.calls[0];
    expect(accountId).toBe('self');
    expect(body.redirect_uris).toEqual(['https://app.example.com/a', 'https://app.example.com/b']);
    expect(body.require_scopes).toBe(true);
  });
});

function renderToolsPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/external_tools']}>
      <Routes>
        <Route path="/courses/:courseId/external_tools" element={<ExternalToolsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ExternalToolsPage (B084)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getCourse.mockResolvedValue({ id: 1, name: 'Biology' });
  });

  test('sends a numeric developer_key_id and object custom_fields', async () => {
    api.getExternalTools.mockResolvedValue({ data: [] });
    api.createExternalTool.mockResolvedValue({ id: 9 });
    const { container } = renderToolsPage();
    await waitFor(() => expect(api.getExternalTools).toHaveBeenCalled());

    fireEvent.click(screen.getByRole('button', { name: /Install New Tool/i }));
    fireEvent.change(container.querySelector('input[name="name"]'), { target: { value: 'Example Tool' } });
    fireEvent.change(container.querySelector('input[name="developer_key_id"]'), { target: { value: '5' } });
    fireEvent.change(container.querySelector('textarea[name="custom_fields"]'), { target: { value: '{"unit":"3"}' } });
    fireEvent.submit(container.querySelector('form'));

    await waitFor(() => expect(api.createExternalTool).toHaveBeenCalled());
    const [, tool] = api.createExternalTool.mock.calls[0];
    expect(tool.developer_key_id).toBe(5);
    expect(tool.custom_fields).toEqual({ unit: '3' });
  });

  test('edit form shows stored custom_fields text without double-encoding', async () => {
    api.getExternalTools.mockResolvedValue({
      data: [{ id: 9, name: 'Example Tool', developer_key_id: 5, custom_fields: '{"unit":"3"}', workflow_state: 'active' }],
    });
    const { container } = renderToolsPage();
    await waitFor(() => expect(screen.getByText('Example Tool')).toBeInTheDocument());

    fireEvent.click(screen.getByTitle('Edit tool'));
    const textarea = container.querySelector('textarea[name="custom_fields"]');
    expect(JSON.parse(textarea.value)).toEqual({ unit: '3' });
    expect(textarea.value).not.toContain('\\"');
  });
});

describe('OAuthConsentPage (B076)', () => {
  beforeEach(() => vi.clearAllMocks());

  test('issues nothing until the user approves, then follows the redirect', async () => {
    api.getOAuthAuthorizeInfo.mockResolvedValue({
      client_name: 'Example App', redirect_uri: 'https://app.example.com/cb', scopes: ['url:GET|/api/v1/courses'],
    });
    api.submitOAuthConsent.mockResolvedValue({ redirect_url: 'https://app.example.com/cb?code=abc&state=s1' });
    const assign = vi.fn();
    const original = window.location;
    Object.defineProperty(window, 'location', { configurable: true, value: { ...original, assign } });

    render(
      <MemoryRouter initialEntries={['/login/oauth2/confirm?client_id=dk_example&redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&state=s1']}>
        <Routes>
          <Route path="/login/oauth2/confirm" element={<OAuthConsentPage />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => expect(screen.getByText('Example App')).toBeInTheDocument());
    expect(api.submitOAuthConsent).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Authorize' }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('https://app.example.com/cb?code=abc&state=s1'));
    expect(api.submitOAuthConsent).toHaveBeenCalledWith(expect.objectContaining({
      client_id: 'dk_example', redirect_uri: 'https://app.example.com/cb', state: 's1', approve: true,
    }));

    Object.defineProperty(window, 'location', { configurable: true, value: original });
  });
});
