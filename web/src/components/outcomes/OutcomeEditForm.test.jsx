import React from 'react';
import { describe, test, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import OutcomeEditForm from './OutcomeEditForm';

const outcome = {
  id: 4,
  title: 'Analyzes sources',
  description: 'Uses evidence',
  mastery_points: 3,
  calculation_method: 'n_mastery',
  calculation_int: 65, // a legacy, impossible value
  ratings: JSON.stringify([{ description: 'Exceeds', points: 4 }, { description: 'Meets', points: 3 }, { description: 'Below', points: 0 }]),
};

describe('OutcomeEditForm (G151)', () => {
  test('repairs an impossible n and saves the full outcome', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<OutcomeEditForm outcome={outcome} onSave={onSave} onCancel={() => {}} />);
    expect(screen.getByLabelText('Times mastery is required').value).toBe('5');

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'Analyzes primary sources' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const payload = onSave.mock.calls[0][0];
    expect(payload).toMatchObject({
      title: 'Analyzes primary sources',
      calculation_method: 'n_mastery',
      calculation_int: 5,
      mastery_points: 3,
      points_possible: 4,
    });
    expect(JSON.parse(payload.ratings).map((r) => r.points)).toEqual([4, 3, 0]);
  });

  test('switching to a method without a parameter drops it; errors are shown', async () => {
    const onSave = vi.fn().mockRejectedValue(new Error('server said no'));
    render(<OutcomeEditForm outcome={outcome} onSave={onSave} onCancel={() => {}} />);
    fireEvent.change(screen.getByLabelText('Calculation method'), { target: { value: 'highest' } });
    expect(screen.queryByLabelText('Times mastery is required')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('server said no'));
    expect(onSave.mock.calls[0][0].calculation_int).toBeUndefined();
  });

  test('blocks a mastery threshold above the top rating', () => {
    const onSave = vi.fn();
    render(<OutcomeEditForm outcome={outcome} onSave={onSave} onCancel={() => {}} />);
    fireEvent.change(screen.getByLabelText('Mastery at (points)'), { target: { value: '9' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(screen.getByRole('alert').textContent).toMatch(/above the highest rating/);
    expect(onSave).not.toHaveBeenCalled();
  });
});
