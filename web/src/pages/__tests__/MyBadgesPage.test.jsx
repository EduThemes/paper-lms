import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import MyBadgesPage from '../MyBadgesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    gamification: {
      listUserBadges: vi.fn(),
    },
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 42 } }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

describe('MyBadgesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('shows the empty state when the learner has no awards', async () => {
    api.gamification.listUserBadges.mockResolvedValue({ user_id: 42, badges: [] });
    render(<MyBadgesPage />);
    await waitFor(() => {
      expect(screen.getByText(/No badges yet/i)).toBeInTheDocument();
    });
    expect(
      screen.getByText(/Badges get awarded for completed work/i),
    ).toBeInTheDocument();
  });

  test('renders one card per earned badge with name + description + date', async () => {
    api.gamification.listUserBadges.mockResolvedValue({
      user_id: 42,
      badges: [
        {
          award_id: 1,
          badge_id: 50,
          code: 'first_quiz',
          name: 'First Quiz',
          description: 'Pass your first quiz.',
          icon: 'trophy',
          color: '#F59E0B',
          awarded_at: '2026-05-13T09:00:00Z',
        },
        {
          award_id: 2,
          badge_id: 51,
          code: 'streak_7',
          name: 'Week-long Streak',
          description: 'Seven days in a row.',
          icon: 'flame',
          color: '#EF4444',
          awarded_at: '2026-05-12T09:00:00Z',
        },
      ],
    });

    render(<MyBadgesPage />);
    await waitFor(() => {
      expect(screen.getByText('First Quiz')).toBeInTheDocument();
    });
    expect(screen.getByText('Week-long Streak')).toBeInTheDocument();
    expect(screen.getByText('Pass your first quiz.')).toBeInTheDocument();
    expect(screen.getByText('Seven days in a row.')).toBeInTheDocument();
    // 2 awards → 2 list items
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
  });

  test('surfaces a load error inline', async () => {
    api.gamification.listUserBadges.mockRejectedValue(new Error('boom'));
    render(<MyBadgesPage />);
    await waitFor(() => {
      expect(screen.getByText(/boom/)).toBeInTheDocument();
    });
  });
});
