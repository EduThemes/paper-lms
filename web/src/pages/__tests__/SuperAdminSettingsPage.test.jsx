import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import SuperAdminSettingsPage from '../SuperAdminSettingsPage';
import { api } from '../../services/api';

// Mock the full settings API surface. Each test sets up the
// responses it needs.
vi.mock('../../services/api', () => ({
  api: {
    superAdminSettings: {
      getGroups: vi.fn(),
      listSettings: vi.fn(),
      setSetting: vi.fn(),
      clearSetting: vi.fn(),
      testEmail: vi.fn(),
      testOIDC: vi.fn(),
      testAnthropic: vi.fn(),
      testS3: vi.fn(),
    },
  },
}));

// Layout is unrelated to the unit under test.
vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

const sampleGroups = {
  definitions: [
    {
      key: 'smtp.host',
      group: 'Email',
      label: 'SMTP host',
      description: 'Outbound SMTP server.',
      value_type: 'string',
      is_secret: false,
      scopes: ['instance', 'account'],
      env_fallback: 'SMTP_HOST',
      has_default: false,
      test_action: 'email',
    },
    {
      key: 'smtp.password',
      group: 'Email',
      label: 'SMTP password',
      description: 'SMTP auth password.',
      value_type: 'secret',
      is_secret: true,
      scopes: ['instance', 'account'],
      env_fallback: 'SMTP_PASSWORD',
      has_default: false,
      test_action: '',
    },
    {
      key: 'storage.s3.bucket',
      group: 'File storage',
      label: 'S3 bucket',
      description: 'S3 bucket name.',
      value_type: 'string',
      is_secret: false,
      scopes: ['instance'],
      env_fallback: 'S3_BUCKET',
      has_default: false,
      test_action: 's3',
    },
  ],
};

const sampleSettings = {
  settings: [
    {
      key: 'smtp.host',
      group: 'Email',
      label: 'SMTP host',
      value_type: 'string',
      is_secret: false,
      source: 'instance',
      has_value: true,
      value: 'mail.example.test',
    },
    {
      key: 'smtp.password',
      group: 'Email',
      label: 'SMTP password',
      value_type: 'secret',
      is_secret: true,
      source: 'instance',
      has_value: true,
      // CRITICAL: value field MUST be absent. Server omits when is_secret.
    },
    {
      key: 'storage.s3.bucket',
      group: 'File storage',
      label: 'S3 bucket',
      value_type: 'string',
      is_secret: false,
      source: 'env',
      has_value: true,
      value: 'paper-lms-prod',
    },
  ],
};

describe('SuperAdminSettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.superAdminSettings.getGroups.mockResolvedValue(sampleGroups);
    api.superAdminSettings.listSettings.mockResolvedValue(sampleSettings);
  });

  test('renders Email group by default with both entries', async () => {
    render(<SuperAdminSettingsPage />);
    await waitFor(() => {
      expect(screen.getByText('SMTP host')).toBeInTheDocument();
      expect(screen.getByText('SMTP password')).toBeInTheDocument();
    });
  });

  test('non-secret value renders verbatim', async () => {
    render(<SuperAdminSettingsPage />);
    await waitFor(() => {
      expect(screen.getByText('mail.example.test')).toBeInTheDocument();
    });
  });

  test('secret value does NOT render plaintext anywhere', async () => {
    // Even if a future bug accidentally rendered ev.value, the server
    // doesn't return a value field for secrets — so the plaintext
    // shouldn't be in the response at all. This test asserts
    // "Set" badge appears instead.
    render(<SuperAdminSettingsPage />);
    await waitFor(() => {
      // The password row should show "Set" not a value.
      const passwordSection = screen.getByText('SMTP password').closest('section');
      expect(passwordSection).toHaveTextContent(/Set/);
    });
  });

  test('switching to File storage group surfaces s3.bucket', async () => {
    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    fireEvent.click(screen.getByRole('button', { name: /File storage/i }));

    await waitFor(() => {
      expect(screen.getByText('S3 bucket')).toBeInTheDocument();
      expect(screen.queryByText('SMTP host')).not.toBeInTheDocument();
    });
  });

  test('env-sourced value shows the env hint', async () => {
    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    fireEvent.click(screen.getByRole('button', { name: /File storage/i }));

    await waitFor(() => {
      expect(screen.getByText(/Configured via environment variable/i)).toBeInTheDocument();
    });
  });

  test('saving a non-secret string triggers setSetting and reloads', async () => {
    api.superAdminSettings.setSetting.mockResolvedValue({});
    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    // Find the editor input for SMTP host. The row has multiple
    // inputs (radios for scope) — filter to the text input that
    // accepts "New string value".
    const stringInputs = screen.getAllByLabelText(/New string value/i);
    expect(stringInputs.length).toBeGreaterThan(0);

    fireEvent.change(stringInputs[0], { target: { value: 'new.host.example' } });

    // Click Save (multiple Save buttons exist — one per setting; we
    // click the one whose section contains SMTP host).
    const saveButtons = screen.getAllByRole('button', { name: /Save/i });
    fireEvent.click(saveButtons[0]);

    await waitFor(() => {
      expect(api.superAdminSettings.setSetting).toHaveBeenCalledWith(
        'smtp.host',
        expect.objectContaining({ scope: 'instance', value: 'new.host.example' }),
      );
    });
  });

  test('clearing a value opens a confirm dialog then fires clearSetting', async () => {
    api.superAdminSettings.clearSetting.mockResolvedValue({});
    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    // First "Clear" button belongs to SMTP host (the secret password's
    // clear lives behind a Replace flow).
    const clearButtons = screen.getAllByRole('button', { name: /Clear/i });
    expect(clearButtons.length).toBeGreaterThan(0);

    fireEvent.click(clearButtons[0]);

    // Confirm dialog appears
    await waitFor(() => {
      expect(screen.getByRole('alertdialog')).toBeInTheDocument();
      expect(screen.getByText(/Yes, clear/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Yes, clear/i }));

    await waitFor(() => {
      expect(api.superAdminSettings.clearSetting).toHaveBeenCalledWith(
        'smtp.host',
        expect.objectContaining({ scope: 'instance' }),
      );
    });
  });

  test('test-action button fires the correct endpoint', async () => {
    api.superAdminSettings.testEmail.mockResolvedValue({
      ok: true,
      detail: 'test email sent to ops@example.com',
      duration_ms: 123,
      action: 'email',
    });

    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    // The Email group has a "Test email" button on the smtp.host row.
    const testButtons = screen.getAllByRole('button', { name: /Test email/i });
    fireEvent.click(testButtons[0]);

    await waitFor(() => {
      expect(api.superAdminSettings.testEmail).toHaveBeenCalled();
      expect(screen.getByRole('status')).toHaveTextContent(/Success/);
      expect(screen.getByRole('status')).toHaveTextContent(/test email sent/);
    });
  });

  test('test action failure renders the diagnostic detail without leaking secrets', async () => {
    api.superAdminSettings.testEmail.mockResolvedValue({
      ok: false,
      detail: 'SMTP authentication failed',
      duration_ms: 250,
      action: 'email',
    });

    render(<SuperAdminSettingsPage />);
    await waitFor(() => screen.getByText('SMTP host'));

    const testButtons = screen.getAllByRole('button', { name: /Test email/i });
    fireEvent.click(testButtons[0]);

    await waitFor(() => {
      const status = screen.getByRole('status');
      expect(status).toHaveTextContent(/Failed/);
      expect(status).toHaveTextContent(/SMTP authentication failed/);
      // The detail must never carry the plaintext credential — verified
      // server-side, but locking the UI contract here too.
      expect(status.textContent).not.toMatch(/password|secret_key|api_key/i);
    });
  });
});
