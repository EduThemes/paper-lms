import { describe, test, expect } from 'vitest';
import { buildRecentGrades } from './dashboard';

describe('buildRecentGrades', () => {
  test('titles each graded submission with its assignment name (B207)', () => {
    const rows = buildRecentGrades(
      [
        { id: 3, assignment_id: 7, workflow_state: 'graded', score: 9, graded_at: '2026-01-02T00:00:00Z' },
        { id: 4, assignment_id: 8, workflow_state: 'submitted', score: null },
      ],
      [{ id: 7, name: 'Fractions Quiz' }, { id: 8, name: 'Essay' }],
      { id: 1, name: 'Math' },
    );
    expect(rows).toEqual([
      {
        id: 3, assignment_id: 7, title: 'Fractions Quiz', score: 9,
        graded_at: '2026-01-02T00:00:00Z', course_name: 'Math', course_id: 1,
      },
    ]);
  });

  test('falls back to a generic title when the assignment is unknown', () => {
    const [row] = buildRecentGrades([{ id: 1, assignment_id: 99, workflow_state: 'graded', score: 1 }], [], { id: 1 });
    expect(row.title).toBe('Assignment');
  });
});
