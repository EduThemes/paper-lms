import React from 'react';
import { describe, test, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

vi.mock('../../services/api_learner_outcomes', () => ({
  messageStudentsIndividually: vi.fn(),
}));

import { messageStudentsIndividually } from '../../services/api_learner_outcomes';
import MessageStudentsModal from './MessageStudentsModal';

const students = [
  { id: 1, name: 'Ann Doe', statuses: { 7: 'on_time' }, scores: { 7: 9 }, tardiness_breakdown: { missing: 0 } },
  { id: 2, name: 'Bo Roe', statuses: { 7: 'missing' }, scores: {}, tardiness_breakdown: { missing: 1 } },
  { id: 3, name: 'Cy Poe', statuses: { 7: 'floating' }, scores: {}, tardiness_breakdown: { missing: 0 } },
];
const assignments = [{ assignment_id: 7, title: 'Lab report' }];

describe('MessageStudentsModal (G154 "Message students who…")', () => {
  beforeEach(() => vi.clearAllMocks());

  test('filters by criterion and sends one message per chosen student', async () => {
    messageStudentsIndividually.mockResolvedValue({ sent: 1, failed: [] });
    render(<MessageStudentsModal students={students} assignments={assignments} onClose={() => {}} />);

    // Default criterion: haven't submitted the first assignment -> Bo, Cy.
    expect(screen.getByText('Recipients (2 of 2)')).toBeTruthy();
    fireEvent.click(screen.getByLabelText('Cy Poe'));
    expect(screen.getByText('Recipients (1 of 2)')).toBeTruthy();

    fireEvent.change(screen.getByLabelText('Subject'), { target: { value: 'Lab report' } });
    fireEvent.change(screen.getByLabelText('Message'), { target: { value: 'Please turn it in.' } });
    fireEvent.click(screen.getByRole('button', { name: /Send/ }));

    await waitFor(() => expect(messageStudentsIndividually).toHaveBeenCalledWith([2], 'Lab report', 'Please turn it in.'));
    expect(await screen.findByText(/Sent 1 message\./)).toBeTruthy();
  });

  test('preselected students skip the criterion', () => {
    render(<MessageStudentsModal students={students} assignments={assignments} initialStudentIds={['1']} onClose={() => {}} />);
    expect(screen.queryByLabelText('Students who')).toBeNull();
    expect(screen.getByText('Recipients (1 of 1)')).toBeTruthy();
  });
});
