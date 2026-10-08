import {
  studentKey,
  submissionKeyOf,
  studentStatus,
  filterStudents,
  sortStudents,
  mergeGradedSubmission,
} from '../speedGrader';

const anon = (id, name, sub) => ({ anonymous_id: id, user_id: null, user_name: name, submission: sub });
const named = (id, name, sub, extra = {}) => ({ user_id: id, user_name: name, submission: sub, ...extra });

describe('speedGrader helpers', () => {
  it('addresses anonymized rows by anonymous id only', () => {
    const row = anon('ab12c', 'Student 1', null);
    expect(studentKey(row)).toBe('anon:ab12c');
    expect(submissionKeyOf(row)).toEqual({ anonymousId: 'ab12c' });
    expect(submissionKeyOf(named(7, 'Jane Doe', null))).toEqual({ userId: 7 });
  });

  it('lets a grader-set status win over computed flags', () => {
    expect(studentStatus({ late: true })).toBe('late');
    expect(studentStatus({ late: true, late_policy_status: 'none' })).toBe('none');
    expect(studentStatus({ missing: true, late_policy_status: 'extended' })).toBe('extended');
    expect(studentStatus({ late_policy_status: 'missing' })).toBe('missing');
    expect(studentStatus(null)).toBe('none');
  });

  it('filters by status', () => {
    const rows = [
      named(1, 'A', { submitted_at: '2026-01-01', workflow_state: 'submitted', score: null }),
      named(2, 'B', { submitted_at: '2026-01-01', workflow_state: 'graded', score: 5 }),
      named(3, 'C', { workflow_state: 'unsubmitted', missing: true }),
      named(4, 'D', { excused: true, workflow_state: 'graded' }),
    ];
    expect(filterStudents(rows, 'needs_grading').map((r) => r.user_id)).toEqual([1]);
    expect(filterStudents(rows, 'graded').map((r) => r.user_id)).toEqual([2, 4]);
    expect(filterStudents(rows, 'not_submitted').map((r) => r.user_id)).toEqual([3, 4]);
    expect(filterStudents(rows, 'missing').map((r) => r.user_id)).toEqual([3]);
    expect(filterStudents(rows, 'excused').map((r) => r.user_id)).toEqual([4]);
    expect(filterStudents(rows, 'all')).toHaveLength(4);
  });

  it('keeps the server order for anonymous rows sorted by name', () => {
    const rows = [anon('zz', 'Student 2'), anon('aa', 'Student 1')];
    expect(sortStudents(rows, 'name', { anonymous: true }).map((r) => r.anonymous_id)).toEqual(['zz', 'aa']);
    expect(sortStudents(rows, 'name').map((r) => r.anonymous_id)).toEqual(['aa', 'zz']);
  });

  it('sorts by submission date and groups by group', () => {
    const rows = [
      named(1, 'A', { submitted_at: '2026-03-02T00:00:00Z' }, { group_name: 'Team 2' }),
      named(2, 'B', { submitted_at: '2026-03-01T00:00:00Z' }, { group_name: 'Team 2' }),
      named(3, 'C', null, { group_name: 'Team 1' }),
    ];
    expect(sortStudents(rows, 'submitted_at').map((r) => r.user_id)).toEqual([2, 1, 3]);
    expect(sortStudents(rows, 'name', { groupBy: true }).map((r) => r.user_id)).toEqual([3, 1, 2]);
  });

  it('merges only grade fields from the server answer', () => {
    const current = { body: '<p>x</p>', attachments: [{ id: 1, url: '/anon' }], score: null };
    const merged = mergeGradedSubmission(current, { score: 8, excused: false, attachments: [{ id: 1, url: '/real' }], user_id: 9 });
    expect(merged.score).toBe(8);
    expect(merged.attachments[0].url).toBe('/anon');
    expect(merged.user_id).toBeUndefined();
  });
});
