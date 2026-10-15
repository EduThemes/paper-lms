import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import i18n from '../../i18n';
import PageCssSettings, { PAGE_CSS_KEY } from '../admin/PageCssSettings';
import { adminAccountsApi } from '../../services/api_admin_accounts';

vi.mock('../../services/api_admin_accounts', () => ({
  adminAccountsApi: { getAccountSettings: vi.fn(), setAccountSetting: vi.fn() },
}));

// Made-up account data.
const item = (over = {}) => ({
  key: PAGE_CSS_KEY,
  label: 'Hand-written page CSS',
  description: '',
  value: null,
  effective: true,
  locked: false,
  locked_by: null,
  ...over,
});

function renderSection() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PageCssSettings />
    </QueryClientProvider>,
  );
}

describe('PageCssSettings', () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    await i18n.changeLanguage('en');
  });

  test('renders a labelled, described switch showing the inherited default', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([item()]);
    renderSection();

    const sw = await screen.findByRole('switch', { name: 'Let teachers write their own page CSS' });
    expect(sw).toHaveAttribute('aria-checked', 'true');
    expect(sw).toBeEnabled();
    expect(sw).toHaveAccessibleDescription(/Designed pages \(Page Designer\) keep their colours, dark mode and layout/);
    expect(sw).toHaveAccessibleDescription(/Custom CSS in the designer's Advanced tab/);
    expect(screen.getByText('Using the default (On).')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Use the default' })).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  test('toggling saves false and announces it', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([item()]);
    adminAccountsApi.setAccountSetting.mockResolvedValue(item({ value: false, effective: false }));
    renderSection();

    await userEvent.click(await screen.findByRole('switch'));
    expect(adminAccountsApi.setAccountSetting).toHaveBeenCalledWith(PAGE_CSS_KEY, false);
    expect(await within(screen.getByRole('status')).findByText('Saved')).toBeInTheDocument();
  });

  test('an explicit value offers "Use the default", which clears it', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([item({ value: false, effective: false })]);
    adminAccountsApi.setAccountSetting.mockResolvedValue(item());
    renderSection();

    const sw = await screen.findByRole('switch');
    expect(sw).toHaveAttribute('aria-checked', 'false');
    expect(screen.getByText('Set for this account.')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Use the default' }));
    expect(adminAccountsApi.setAccountSetting).toHaveBeenCalledWith(PAGE_CSS_KEY, null);
  });

  test('locked by the Paper administrator: disabled, off, and says why', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([
      item({ effective: false, locked: true, locked_by: { scope: 'instance' } }),
    ]);
    renderSection();

    const sw = await screen.findByRole('switch');
    expect(sw).toBeDisabled();
    expect(sw).toHaveAttribute('aria-checked', 'false');
    expect(sw).toHaveAccessibleDescription(/Turned off by your Paper administrator/);
    await userEvent.click(sw);
    expect(adminAccountsApi.setAccountSetting).not.toHaveBeenCalled();
  });

  test('locked by a parent account names it, and still allows inherit', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([
      item({
        value: true,
        effective: false,
        locked: true,
        locked_by: { scope: 'account', account_id: 4, account_name: 'Example District' },
      }),
    ]);
    renderSection();

    expect(await screen.findByText('Turned off by Example District')).toBeInTheDocument();
    expect(screen.getByRole('switch')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Use the default' })).toBeEnabled();
  });

  test('a failed save is shown as an alert', async () => {
    adminAccountsApi.getAccountSettings.mockResolvedValue([item()]);
    adminAccountsApi.setAccountSetting.mockRejectedValue(new Error('Turned off by Example District. This account can\'t turn it back on.'));
    renderSection();

    await userEvent.click(await screen.findByRole('switch'));
    expect(await screen.findByRole('alert')).toHaveTextContent('Turned off by Example District');
  });

  test('a load failure offers Try again', async () => {
    adminAccountsApi.getAccountSettings.mockRejectedValueOnce(new Error('down'));
    renderSection();
    expect(await screen.findByText("Couldn't load the page settings.")).toBeInTheDocument();
    adminAccountsApi.getAccountSettings.mockResolvedValue([item()]);
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByRole('switch')).toBeInTheDocument();
  });

  test('Spanish strings', async () => {
    await i18n.changeLanguage('es');
    adminAccountsApi.getAccountSettings.mockResolvedValue([
      item({ effective: false, locked: true, locked_by: { scope: 'instance' } }),
    ]);
    renderSection();
    expect(
      await screen.findByRole('switch', { name: 'Permitir que los profesores escriban su propio CSS de página' }),
    ).toBeDisabled();
    expect(screen.getByText('Desactivado por el administrador de Paper')).toBeInTheDocument();
  });
});
