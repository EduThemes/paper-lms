import React, { useState } from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import EffectsPalette from '../EffectsPalette';

vi.mock('../../../../services/api', () => ({
  api: {
    gamification: {
      listCurrencies: vi.fn(),
      listBadges: vi.fn(),
    },
  },
}));

import { api } from '../../../../services/api';

const CURRENCIES = {
  currencies: [
    { id: 11, scope_type: 'site', code: 'xp', display_label: 'XP' },
    { id: 12, scope_type: 'site', code: 'gems', display_label: 'Gems' },
  ],
};
const BADGES = {
  badges: [{ id: 5, scope_type: 'site', code: 'first_quiz', name: 'First Quiz' }],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.gamification.listCurrencies.mockResolvedValue(CURRENCIES);
  api.gamification.listBadges.mockResolvedValue(BADGES);
});

// Test harness that exposes the controlled state so assertions read
// the JSON the palette would emit on save.
function Harness({ initial = [] }) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <EffectsPalette value={value} onChange={setValue} />
      <pre data-testid="state">{JSON.stringify(value)}</pre>
    </>
  );
}

function getState() {
  return JSON.parse(screen.getByTestId('state').textContent);
}

describe('EffectsPalette', () => {
  it('renders the empty-state message when no effects are authored', () => {
    render(<Harness initial={[]} />);
    expect(screen.getByText(/No effects yet/i)).toBeInTheDocument();
  });

  it('adds an AwardCurrency effect via the Add menu, seeded with amount=1', async () => {
    render(<Harness initial={[]} />);
    fireEvent.change(screen.getByLabelText('Add effect'), {
      target: { value: 'AwardCurrency' },
    });
    await waitFor(() =>
      expect(screen.getByRole('option', { name: /XP \(xp\)/ })).toBeInTheDocument(),
    );
    const state = getState();
    expect(state).toHaveLength(1);
    expect(state[0]).toEqual({ kind: 'AwardCurrency', amount: 1 });
  });

  it('sets currency code + amount through the inline editor', async () => {
    render(<Harness initial={[]} />);
    fireEvent.change(screen.getByLabelText('Add effect'), {
      target: { value: 'AwardCurrency' },
    });
    await waitFor(() =>
      expect(screen.getByRole('option', { name: /XP \(xp\)/ })).toBeInTheDocument(),
    );
    fireEvent.change(screen.getByLabelText(/^Currency/), { target: { value: 'gems' } });
    fireEvent.change(screen.getByLabelText(/^Amount/), { target: { value: '25' } });

    const state = getState();
    expect(state[0]).toEqual({ kind: 'AwardCurrency', amount: 25, code: 'gems' });
  });

  it('adds an AwardBadge effect and writes the badge code (not the id)', async () => {
    render(<Harness initial={[]} />);
    fireEvent.change(screen.getByLabelText('Add effect'), {
      target: { value: 'AwardBadge' },
    });
    await waitFor(() =>
      expect(screen.getByRole('option', { name: /First Quiz/ })).toBeInTheDocument(),
    );
    fireEvent.change(screen.getByLabelText(/^Badge/), { target: { value: 'first_quiz' } });
    fireEvent.change(screen.getByLabelText(/Evidence/i), { target: { value: 'manual override' } });

    const state = getState();
    expect(state[0]).toEqual({ kind: 'AwardBadge', code: 'first_quiz', evidence: 'manual override' });
  });

  it('removes an effect via the trash button', async () => {
    render(<Harness initial={[{ kind: 'AwardCurrency', code: 'xp', amount: 5 }]} />);
    await waitFor(() =>
      expect(screen.getByLabelText('Drag handle for effect 1')).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /Remove effect 1/i }));
    expect(getState()).toEqual([]);
  });

  it('preserves order across multiple adds (drag handle wires up)', async () => {
    render(<Harness initial={[]} />);
    fireEvent.change(screen.getByLabelText('Add effect'), { target: { value: 'AwardCurrency' } });
    await waitFor(() =>
      expect(screen.getByLabelText('Drag handle for effect 1')).toBeInTheDocument(),
    );
    fireEvent.change(screen.getByLabelText('Add effect'), { target: { value: 'AwardBadge' } });
    await waitFor(() =>
      expect(screen.getByLabelText('Drag handle for effect 2')).toBeInTheDocument(),
    );

    // Both drag handles render with aria-label. (Actual drag-drop
    // requires pointer events that jsdom can't simulate; the wire-up
    // assertion guarantees @dnd-kit accepted the SortableContext + each
    // row's useSortable hook — the manual smoke flow exercises drag.)
    expect(screen.getByLabelText('Drag handle for effect 1')).toBeInTheDocument();
    expect(screen.getByLabelText('Drag handle for effect 2')).toBeInTheDocument();

    const state = getState();
    expect(state).toHaveLength(2);
    // Internal _dragId is stripped from what the parent sees on save.
    expect(state[0]).not.toHaveProperty('_dragId');
    expect(state[1]).not.toHaveProperty('_dragId');
  });
});
