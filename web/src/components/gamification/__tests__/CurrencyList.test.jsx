import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import CurrencyList from '../CurrencyList';
import { api } from '../../../services/api';

vi.mock('../../../services/api', () => ({
  api: {
    gamification: {
      listCurrencies: vi.fn(),
      createCurrency: vi.fn(),
      updateCurrency: vi.fn(),
      deleteCurrency: vi.fn(),
    },
  },
}));

const siteXP = {
  id: 11,
  scope_type: 'site',
  scope_id: 1,
  code: 'xp',
  display_label: 'XP',
  display_label_plural: 'XP',
  icon: 'zap',
  color: '#F59E0B',
  display_order: 1,
  spendable: false,
  monotonic: true,
  visible_to_student: true,
  visible_in_topbar: true,
  system_owned: true,
};
const siteGems = {
  ...siteXP,
  id: 12,
  code: 'gems',
  display_label: 'Gem',
  icon: 'gem',
  color: '#A855F7',
  spendable: true,
  monotonic: false,
  system_owned: true,
  display_order: 2,
};
const courseStars = {
  ...siteXP,
  id: 50,
  scope_type: 'course',
  scope_id: 99,
  code: 'stars',
  display_label: 'Star',
  system_owned: false,
};

describe('CurrencyList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // jsdom: confirm dialog used by handleDelete
    window.confirm = vi.fn(() => true);
  });

  test('site scope: lists only site-scoped currencies', async () => {
    api.gamification.listCurrencies.mockResolvedValue({
      currencies: [siteXP, siteGems, courseStars],
    });

    render(<CurrencyList />);

    await waitFor(() => {
      expect(screen.getByText('XP')).toBeInTheDocument();
    });
    expect(screen.getByText('Gem')).toBeInTheDocument();
    expect(screen.queryByText('Star')).not.toBeInTheDocument();
    expect(screen.getByText('Scope: site')).toBeInTheDocument();
  });

  test('course scope: lists only this-course-scoped currencies', async () => {
    api.gamification.listCurrencies.mockResolvedValue({
      currencies: [siteXP, courseStars, { ...courseStars, id: 51, scope_id: 77 }],
    });

    render(<CurrencyList courseId={99} />);

    await waitFor(() => {
      expect(screen.getByText('Star')).toBeInTheDocument();
    });
    expect(screen.queryByText('XP')).not.toBeInTheDocument();
    // The course=77 row must not appear in the course=99 view.
    const starRows = screen.getAllByText('Star');
    expect(starRows).toHaveLength(1);
  });

  test('delete button disabled on system_owned rows', async () => {
    api.gamification.listCurrencies.mockResolvedValue({ currencies: [siteXP, siteGems] });

    render(<CurrencyList />);
    await waitFor(() => expect(screen.getByText('XP')).toBeInTheDocument());

    const deleteButtons = screen.getAllByTitle(/cannot be deleted|Delete/i);
    deleteButtons.forEach((btn) => {
      if (btn.title.includes('cannot be deleted')) {
        expect(btn).toBeDisabled();
      }
    });
  });

  test('clicking "New currency" opens the editor and a create call dispatches wallet:refresh', async () => {
    api.gamification.listCurrencies.mockResolvedValue({ currencies: [siteXP] });
    api.gamification.createCurrency.mockResolvedValue({ id: 99, ...courseStars, code: 'coins', display_label: 'Coin' });

    const refreshHandler = vi.fn();
    window.addEventListener('wallet:refresh', refreshHandler);

    render(<CurrencyList />);
    await waitFor(() => expect(screen.getByText('XP')).toBeInTheDocument());

    fireEvent.click(screen.getByText('New currency'));
    // Editor dialog mounts; the title should be "New currency"
    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    // Fill in code + label
    const codeInput = screen.getByPlaceholderText('coins');
    const labelInput = screen.getByPlaceholderText('Coin');
    fireEvent.change(codeInput, { target: { value: 'coins' } });
    fireEvent.change(labelInput, { target: { value: 'Coin' } });

    fireEvent.click(screen.getByText('Create currency'));

    await waitFor(() => {
      expect(api.gamification.createCurrency).toHaveBeenCalled();
    });
    const [body, opts] = api.gamification.createCurrency.mock.calls[0];
    expect(body.code).toBe('coins');
    expect(body.display_label).toBe('Coin');
    expect(opts).toEqual({ courseId: undefined });

    await waitFor(() => {
      expect(refreshHandler).toHaveBeenCalled();
    });
    window.removeEventListener('wallet:refresh', refreshHandler);
  });

  test('course scope passes courseId to createCurrency', async () => {
    api.gamification.listCurrencies.mockResolvedValue({ currencies: [] });
    api.gamification.createCurrency.mockResolvedValue({ id: 99 });

    render(<CurrencyList courseId={99} />);
    await waitFor(() => expect(screen.getByText('New currency')).toBeInTheDocument());

    fireEvent.click(screen.getByText('New currency'));
    fireEvent.change(screen.getByPlaceholderText('coins'), { target: { value: 'stars' } });
    fireEvent.change(screen.getByPlaceholderText('Coin'), { target: { value: 'Star' } });
    fireEvent.click(screen.getByText('Create currency'));

    await waitFor(() => {
      expect(api.gamification.createCurrency).toHaveBeenCalled();
    });
    const [, opts] = api.gamification.createCurrency.mock.calls[0];
    expect(opts).toEqual({ courseId: 99 });
  });
});
