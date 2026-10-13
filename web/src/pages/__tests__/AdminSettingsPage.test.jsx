import { describe, test, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AdminSettingsPage from '../AdminSettingsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: { superAdminSettings: { getSetting: vi.fn(), setSetting: vi.fn() } },
}));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/admin/StorageQuotaSettings', () => ({ default: () => null }));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}><MemoryRouter><AdminSettingsPage /></MemoryRouter></QueryClientProvider>,
  );
}

describe('AdminSettingsPage upload limit', () => {
  test('is read-only for account admins (the setting endpoint answers 403)', async () => {
    api.superAdminSettings.getSetting.mockRejectedValue(Object.assign(new Error('Forbidden'), { status: 403 }));
    renderPage();
    expect(await screen.findByText(/Only a platform super-admin can change it/)).toBeInTheDocument();
    expect(screen.getByLabelText('Maximum upload size (MB)')).toBeDisabled();
    expect(screen.queryByRole('button', { name: /save/i })).not.toBeInTheDocument();
  });

  test('is editable for super-admins', async () => {
    api.superAdminSettings.getSetting.mockResolvedValue({ value: '2048', source: 'instance', is_secret: false });
    renderPage();
    expect(await screen.findByRole('button', { name: /save/i })).toBeInTheDocument();
    expect(screen.getByLabelText('Maximum upload size (MB)')).toBeEnabled();
    expect(screen.queryByText(/Only a platform super-admin/)).not.toBeInTheDocument();
  });
});
