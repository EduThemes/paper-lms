import { render, screen, fireEvent } from '@testing-library/react';
import { ModuleProgressGrid } from '../ModuleProgressPage';

// G061/G062: the teacher's Student progress grid.
describe('ModuleProgressGrid', () => {
  const data = {
    modules: [
      { id: 1, name: 'Week 1' },
      { id: 2, name: 'Week 2' },
    ],
    students: [
      { user_id: 7, name: 'Jane Doe', completed: 1, states: { 1: 'completed', 2: 'locked' } },
      { user_id: 8, name: 'John Roe', completed: 0, states: { 1: 'started' } },
    ],
  };

  test('shows each student state per module and the completed count', () => {
    render(<ModuleProgressGrid data={data} />);
    expect(screen.getByRole('columnheader', { name: 'Week 1' })).toBeInTheDocument();
    const jane = screen.getByRole('row', { name: /Jane Doe/ });
    expect(jane).toHaveTextContent('Completed');
    expect(jane).toHaveTextContent('Locked');
    expect(jane).toHaveTextContent('1 / 2');
    const john = screen.getByRole('row', { name: /John Roe/ });
    expect(john).toHaveTextContent('In progress');
    expect(john).toHaveTextContent('—');
  });

  test('selecting a student opens the drill-down', () => {
    const onSelect = vi.fn();
    render(<ModuleProgressGrid data={data} onSelect={onSelect} />);
    fireEvent.click(screen.getByRole('button', { name: 'John Roe' }));
    expect(onSelect).toHaveBeenCalledWith(data.students[1]);
  });

  test('empty roster', () => {
    render(<ModuleProgressGrid data={{ modules: [], students: [] }} />);
    expect(screen.getByText(/No active students/)).toBeInTheDocument();
  });
});
