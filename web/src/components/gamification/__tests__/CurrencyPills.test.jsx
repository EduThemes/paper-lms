import React from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import CurrencyPills from '../CurrencyPills';
import { api } from '../../../services/api';

vi.mock('../../../services/api', () => ({
  api: {
    gamification: {
      getUserWallet: vi.fn(),
      listUserWalletTransactions: vi.fn(),
    },
  },
}));

vi.mock('../../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 42, name: 'Test Student' },
  }),
}));

const xpBalance = {
  currency_type_id: 11,
  code: 'xp',
  display_label: 'XP',
  display_label_plural: 'XP',
  icon: 'zap',
  color: '#F59E0B',
  balance: 250,
  lifetime_earned: 250,
  spendable: false,
  monotonic: true,
  visible_in_topbar: true,
  display_order: 1,
};

const gemsBalance = {
  currency_type_id: 12,
  code: 'gems',
  display_label: 'Gem',
  display_label_plural: 'Gems',
  icon: 'gem',
  color: '#A855F7',
  balance: 8,
  lifetime_earned: 12,
  spendable: true,
  monotonic: false,
  visible_in_topbar: true,
  display_order: 2,
};

const masteryBalance = {
  currency_type_id: 13,
  code: 'mastery_points',
  display_label: 'Mastery',
  display_label_plural: 'Mastery Points',
  icon: 'target',
  color: '#0EA5E9',
  balance: 5,
  lifetime_earned: 5,
  spendable: false,
  monotonic: true,
  visible_in_topbar: false,
  display_order: 3,
};

describe('CurrencyPills', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('renders only topbar-visible currencies in display_order', async () => {
    api.gamification.getUserWallet.mockResolvedValue({
      user_id: 42,
      balances: [gemsBalance, masteryBalance, xpBalance],
    });

    render(<CurrencyPills />);

    await waitFor(() => {
      expect(screen.getAllByRole('button')).toHaveLength(2);
    });

    const buttons = screen.getAllByRole('button');
    expect(buttons[0]).toHaveAttribute('title', 'XP');
    expect(buttons[1]).toHaveAttribute('title', 'Gem');
    expect(screen.queryByTitle('Mastery')).not.toBeInTheDocument();
  });

  test('hides itself when there are no topbar balances', async () => {
    api.gamification.getUserWallet.mockResolvedValue({
      user_id: 42,
      balances: [masteryBalance],
    });

    const { container } = render(<CurrencyPills />);
    await waitFor(() => {
      expect(api.gamification.getUserWallet).toHaveBeenCalled();
    });
    expect(container.querySelector('[role="group"]')).toBeNull();
  });

  test('formats large balances compactly', async () => {
    api.gamification.getUserWallet.mockResolvedValue({
      user_id: 42,
      balances: [{ ...xpBalance, balance: 12450 }],
    });

    render(<CurrencyPills />);
    await waitFor(() => {
      expect(screen.getByText('12.4k')).toBeInTheDocument();
    });
  });

  test('clicking a pill opens WalletDrawer and loads that currency’s transactions', async () => {
    api.gamification.getUserWallet.mockResolvedValue({
      user_id: 42,
      balances: [xpBalance, gemsBalance],
    });
    api.gamification.listUserWalletTransactions.mockResolvedValue({
      user_id: 42,
      currency_type_id: 11,
      transactions: [
        { id: 101, delta: 25, reason: 'rule:7', occurred_at: '2026-05-13T09:30:00Z' },
      ],
      total_count: 1,
      page: 1,
      per_page: 20,
    });

    render(<CurrencyPills />);
    await waitFor(() => {
      expect(screen.getByTitle('XP')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTitle('XP'));

    await waitFor(() => {
      expect(api.gamification.listUserWalletTransactions).toHaveBeenCalledWith(
        42,
        11,
        { page: 1, perPage: 20 },
      );
    });

    expect(screen.getByText('+25')).toBeInTheDocument();
    expect(screen.getByText('Rule #7')).toBeInTheDocument();
  });

  test('re-fetches wallet on wallet:refresh event', async () => {
    api.gamification.getUserWallet.mockResolvedValue({
      user_id: 42,
      balances: [xpBalance],
    });

    render(<CurrencyPills />);
    await waitFor(() => {
      expect(api.gamification.getUserWallet).toHaveBeenCalledTimes(1);
    });

    act(() => {
      window.dispatchEvent(new Event('wallet:refresh'));
    });

    await waitFor(() => {
      expect(api.gamification.getUserWallet).toHaveBeenCalledTimes(2);
    });
  });
});
