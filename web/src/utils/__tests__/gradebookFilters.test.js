import { assignmentInPeriod, filterAssignments, moduleAssignmentIds, filterStudentsBySection } from '../gradebookFilters';

const periods = [
  { id: 1, title: 'Fall', start_date: '2026-08-20T00:00:00Z', end_date: '2026-12-20T00:00:00Z' },
  { id: 2, title: 'Spring', start_date: '2027-01-05T00:00:00Z', end_date: '2027-05-20T00:00:00Z' },
];

describe('gradebook filters', () => {
  it('places assignments in the period of their due date (undated in the last)', () => {
    expect(assignmentInPeriod({ due_at: '2026-10-01T00:00:00Z' }, periods[0], periods)).toBe(true);
    expect(assignmentInPeriod({ due_at: '2026-10-01T00:00:00Z' }, periods[1], periods)).toBe(false);
    expect(assignmentInPeriod({ due_at: null }, periods[1], periods)).toBe(true);
    expect(assignmentInPeriod({ due_at: null }, periods[0], periods)).toBe(false);
  });

  it('filters by grading period and module', () => {
    const assignments = [
      { id: 1, due_at: '2026-09-01T00:00:00Z' },
      { id: 2, due_at: '2027-02-01T00:00:00Z' },
      { id: 3, due_at: '2026-11-01T00:00:00Z' },
    ];
    const modules = moduleAssignmentIds([
      { id: 9, items: [{ type: 'Assignment', content_id: 3 }, { type: 'Page', content_id: 1 }] },
    ]);
    expect(filterAssignments(assignments, { gradingPeriodId: '1' }, { periods }).map((a) => a.id)).toEqual([1, 3]);
    expect(filterAssignments(assignments, { moduleId: '9' }, { modules }).map((a) => a.id)).toEqual([3]);
    expect(filterAssignments(assignments, {}, {})).toHaveLength(3);
  });

  it('filters students by section', () => {
    const students = [{ id: 1, section_ids: [5] }, { id: 2, section_ids: [6] }];
    expect(filterStudentsBySection(students, '6').map((s) => s.id)).toEqual([2]);
    expect(filterStudentsBySection(students, '')).toHaveLength(2);
  });
});
