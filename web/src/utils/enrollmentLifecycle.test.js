import { describe, test, expect } from 'vitest';
import {
  assignableRoles,
  buildEnrollmentPatch,
  canManageType,
  crosslistTargets,
  enrollmentActions,
  enrollmentForm,
  enrollmentStatus,
  fromDateInput,
  sectionLabel,
  sectionPeopleCounts,
  toDateInput,
} from './enrollmentLifecycle';

// Enrollment lifecycle UI rules (G105 / G107 / G110 / G122). Made-up data.

const teacher = { userId: 1, isTeacher: true, isStaff: true };
const ta = { userId: 2, isStaff: true };
const student = { userId: 3 };
const admin = { userId: 9, isAdmin: true };

describe('canManageType / assignableRoles', () => {
  test('teachers and admins manage every role, TAs only learners', () => {
    expect(canManageType(teacher, 'TeacherEnrollment')).toBe(true);
    expect(canManageType(admin, 'TaEnrollment')).toBe(true);
    expect(canManageType(ta, 'StudentEnrollment')).toBe(true);
    expect(canManageType(ta, 'ObserverEnrollment')).toBe(true);
    expect(canManageType(ta, 'TeacherEnrollment')).toBe(false);
    expect(canManageType(student, 'StudentEnrollment')).toBe(false);
    expect(canManageType(null, 'StudentEnrollment')).toBe(false);
  });

  test('assignable roles follow the same rule', () => {
    expect(assignableRoles(ta)).toEqual(['StudentEnrollment', 'ObserverEnrollment']);
    expect(assignableRoles(teacher)).toHaveLength(5);
    expect(assignableRoles(student)).toEqual([]);
  });
});

describe('enrollmentActions', () => {
  const row = (state, extra = {}) => ({ id: 7, user_id: 5, type: 'StudentEnrollment', enrollment_state: state, ...extra });

  test('actions depend on the state', () => {
    expect(enrollmentActions(row('active'), teacher)).toEqual(['edit', 'conclude', 'deactivate', 'delete']);
    expect(enrollmentActions(row('inactive'), teacher)).toContain('reactivate');
    expect(enrollmentActions(row('completed'), teacher)).toContain('reactivate');
    expect(enrollmentActions(row('completed'), teacher)).not.toContain('conclude');
    expect(enrollmentActions(row('invited'), teacher)).toEqual(['edit', 'delete']);
    expect(enrollmentActions(row('rejected'), teacher)).toEqual(['delete']);
    expect(enrollmentActions(row(undefined), teacher)).toContain('conclude');
  });

  test('a TA cannot manage a teacher, a student nothing', () => {
    expect(enrollmentActions(row('active', { type: 'TeacherEnrollment' }), ta)).toEqual([]);
    expect(enrollmentActions(row('active'), student)).toEqual([]);
  });

  test('nobody but an admin manages their own enrollment', () => {
    expect(enrollmentActions(row('active', { user_id: 1, type: 'TeacherEnrollment' }), teacher)).toEqual([]);
    expect(enrollmentActions(row('active', { user_id: 9 }), admin)).not.toEqual([]);
  });
});

describe('enrollmentStatus', () => {
  const now = new Date('2026-10-08T12:00:00Z');
  test('maps workflow states to labels', () => {
    expect(enrollmentStatus({ enrollment_state: 'completed' }, now).label).toBe('Concluded');
    expect(enrollmentStatus({ enrollment_state: 'inactive' }, now).key).toBe('inactive');
    expect(enrollmentStatus({ enrollment_state: 'invited' }, now).label).toBe('Invited');
    expect(enrollmentStatus({}, now).label).toBe('Active');
  });

  test('applies the enrollment dates when both are set', () => {
    const past = { enrollment_state: 'active', start_at: '2026-01-01T00:00:00Z', end_at: '2026-06-01T00:00:00Z' };
    const future = { enrollment_state: 'active', start_at: '2026-11-01T00:00:00Z', end_at: '2026-12-01T00:00:00Z' };
    const onlyEnd = { enrollment_state: 'active', end_at: '2026-06-01T00:00:00Z' };
    expect(enrollmentStatus(past, now).key).toBe('concluded');
    expect(enrollmentStatus(future, now).key).toBe('pending');
    expect(enrollmentStatus(onlyEnd, now).key).toBe('active');
  });
});

describe('sections', () => {
  test('sectionLabel names the default section', () => {
    expect(sectionLabel({ 4: { name: 'Evening' } }, 4)).toBe('Evening');
    expect(sectionLabel({}, null)).toBe('Default section');
    expect(sectionLabel({}, 12)).toBe('Section 12');
  });

  test('sectionPeopleCounts groups by section, default under 0', () => {
    expect(sectionPeopleCounts([
      { course_section_id: 4 }, { course_section_id: 4 }, { course_section_id: null }, { course_section_id: 5, enrollment_state: 'deleted' },
    ])).toEqual({ 4: 2, 0: 1 });
  });

  test('crosslistTargets excludes the section\'s own course and deleted courses', () => {
    const courses = [{ id: 1 }, { id: 2 }, { id: 3, workflow_state: 'deleted' }];
    expect(crosslistTargets(courses, { course_id: 1 }).map((c) => c.id)).toEqual([2]);
    expect(crosslistTargets(courses, null)).toEqual([]);
  });
});

describe('dates and the edit patch', () => {
  test('date input round trip', () => {
    expect(toDateInput('2026-03-04T00:00:00Z')).toBe('2026-03-04');
    expect(toDateInput(null)).toBe('');
    expect(fromDateInput('2026-03-04')).toBe('2026-03-04T00:00:00.000Z');
    expect(fromDateInput('')).toBeNull();
  });

  test('only changed fields are sent; clearing a section sends null', () => {
    const e = { type: 'StudentEnrollment', course_section_id: 4, start_at: null, end_at: null, limit_privileges_to_course_section: false };
    const form = enrollmentForm(e);
    expect(buildEnrollmentPatch(e, form).patch).toEqual({});

    const { patch } = buildEnrollmentPatch(e, { ...form, type: 'TaEnrollment', course_section_id: '', limit_privileges_to_course_section: true });
    expect(patch).toEqual({ type: 'TaEnrollment', course_section_id: null, limit_privileges_to_course_section: true });

    const dated = buildEnrollmentPatch(e, { ...form, start_at: '2026-01-10', end_at: '2026-05-10' });
    expect(dated.patch.start_at).toBe('2026-01-10T00:00:00.000Z');
    expect(dated.error).toBeNull();
  });

  test('reversed dates are an error', () => {
    const e = { type: 'StudentEnrollment' };
    const { error } = buildEnrollmentPatch(e, { ...enrollmentForm(e), start_at: '2026-05-10', end_at: '2026-01-10' });
    expect(error).toMatch(/end date/);
  });
});
