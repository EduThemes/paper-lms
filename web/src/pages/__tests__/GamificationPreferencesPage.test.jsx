import React from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import GamificationPreferencesPage from '../GamificationPreferencesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    gamification: {
      getMyPreferences: vi.fn(),
      updateMyPreferences: vi.fn(),
    },
  },
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

describe('GamificationPreferencesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test('renders the toggle in the OFF state for a fresh learner', async () => {
    api.gamification.getMyPreferences.mockResolvedValue({ leaderboard_opt_out: false });
    render(<GamificationPreferencesPage />);
    await waitFor(() => {
      expect(screen.getByLabelText(/Hide me from public leaderboards/i)).toBeInTheDocument();
    });
    expect(screen.getByRole('checkbox')).not.toBeChecked();
  });

  test('renders the toggle in the ON state when the learner has opted out', async () => {
    api.gamification.getMyPreferences.mockResolvedValue({ leaderboard_opt_out: true });
    render(<GamificationPreferencesPage />);
    await waitFor(() => {
      expect(screen.getByRole('checkbox')).toBeChecked();
    });
  });

  test('toggling fires updateMyPreferences and reflects the server response', async () => {
    api.gamification.getMyPreferences.mockResolvedValue({ leaderboard_opt_out: false });
    api.gamification.updateMyPreferences.mockResolvedValue({ leaderboard_opt_out: true });

    render(<GamificationPreferencesPage />);
    await waitFor(() => expect(screen.getByRole('checkbox')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('checkbox'));

    await waitFor(() => {
      expect(api.gamification.updateMyPreferences).toHaveBeenCalledWith({ leaderboard_opt_out: true });
    });
    expect(screen.getByRole('checkbox')).toBeChecked();
    expect(screen.getByText(/Saved/)).toBeInTheDocument();
  });

  test('reverts the optimistic toggle on save failure', async () => {
    api.gamification.getMyPreferences.mockResolvedValue({ leaderboard_opt_out: false });
    api.gamification.updateMyPreferences.mockRejectedValue(new Error('boom'));

    render(<GamificationPreferencesPage />);
    await waitFor(() => expect(screen.getByRole('checkbox')).toBeInTheDocument());

    await act(async () => {
      fireEvent.click(screen.getByRole('checkbox'));
    });

    await waitFor(() => {
      expect(screen.getByText(/boom/)).toBeInTheDocument();
    });
    // Failure → revert to the original (false) value so the UI doesn't
    // diverge from server state.
    expect(screen.getByRole('checkbox')).not.toBeChecked();
  });

  test('copy makes the no-progress-loss contract explicit', async () => {
    // SYNTHESIS §5: opting out is a visibility control, not an awards
    // reset. The page must surface this loud or learners avoid the
    // toggle out of fear they'll lose XP.
    api.gamification.getMyPreferences.mockResolvedValue({ leaderboard_opt_out: false });
    render(<GamificationPreferencesPage />);
    await waitFor(() => expect(screen.getByRole('checkbox')).toBeInTheDocument());
    expect(
      screen.getByText(/Your XP, gems, mastery points, and badges are not affected/i),
    ).toBeInTheDocument();
  });
});
