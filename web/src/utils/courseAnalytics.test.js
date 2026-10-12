import { describe, test, expect } from 'vitest';
import {
  courseRouteInfo,
  groupByWeek,
  fillDays,
  filterStudentsForMessage,
  formatDuration,
} from './courseAnalytics';

describe('courseRouteInfo (G154 page-view tracking)', () => {
  test('maps course paths to a course id and content category', () => {
    expect(courseRouteInfo('/courses/12')).toEqual({ courseId: '12', category: 'home' });
    expect(courseRouteInfo('/courses/12/assignments/3')).toEqual({ courseId: '12', category: 'assignments' });
    expect(courseRouteInfo('/courses/12/discussions/9')).toEqual({ courseId: '12', category: 'discussions' });
    expect(courseRouteInfo('/courses/12/quizzes/4/take')).toEqual({ courseId: '12', category: 'quizzes' });
    expect(courseRouteInfo('/courses/12/whatever')).toEqual({ courseId: '12', category: 'other' });
  });

  test('ignores non-course and instructor-only paths', () => {
    expect(courseRouteInfo('/dashboard')).toBeNull();
    expect(courseRouteInfo('/courses')).toBeNull();
    expect(courseRouteInfo('/courses/12/settings')).toBeNull();
    expect(courseRouteInfo('/courses/12/analytics/users/5')).toBeNull();
    expect(courseRouteInfo(undefined)).toBeNull();
  });
});

describe('groupByWeek', () => {
  test('sums days into Monday-start weeks and fills empty weeks', () => {
    const weeks = groupByWeek([
      { date: '2026-03-02', views: 2, participations: 1 }, // Monday
      { date: '2026-03-08', views: 3, participations: 0 }, // Sunday, same week
      { date: '2026-03-17', views: 1, participations: 2 }, // two weeks later
    ]);
    expect(weeks).toEqual([
      { week_start: '2026-03-02', views: 5, participations: 1 },
      { week_start: '2026-03-09', views: 0, participations: 0 },
      { week_start: '2026-03-16', views: 1, participations: 2 },
    ]);
  });

  test('empty input', () => {
    expect(groupByWeek([])).toEqual([]);
    expect(groupByWeek(null)).toEqual([]);
  });
});

test('fillDays fills idle days with zeros', () => {
  expect(fillDays([
    { date: '2026-03-03', views: 1, participations: 0 },
    { date: '2026-03-01', views: 2, participations: 1 },
  ])).toEqual([
    { date: '2026-03-01', views: 2, participations: 1 },
    { date: '2026-03-02', views: 0, participations: 0 },
    { date: '2026-03-03', views: 1, participations: 0 },
  ]);
});

describe('filterStudentsForMessage ("Message students who…")', () => {
  const now = new Date('2026-03-20T00:00:00Z').getTime();
  const students = [
    { id: 1, name: 'Ann', statuses: { 7: 'on_time' }, scores: { 7: 9 }, tardiness_breakdown: { missing: 0 }, last_activity_at: '2026-03-19T10:00:00Z' },
    { id: 2, name: 'Bo', statuses: { 7: 'missing' }, scores: {}, tardiness_breakdown: { missing: 2 }, last_activity_at: '2026-03-01T10:00:00Z' },
    { id: 3, name: 'Cy', statuses: { 7: 'late' }, scores: { 7: 4 }, tardiness_breakdown: { missing: 0 }, last_activity_at: null },
    { id: 4, name: 'Di', statuses: { 7: 'floating' }, scores: {}, tardiness_breakdown: { missing: 0 } },
  ];
  const ids = (rows) => rows.map((s) => s.id);

  test("haven't submitted", () => {
    expect(ids(filterStudentsForMessage(students, { criterion: 'not_submitted', assignmentId: 7 }))).toEqual([2, 4]);
  });
  test('scored less / more than', () => {
    expect(ids(filterStudentsForMessage(students, { criterion: 'scored_less_than', assignmentId: 7, value: '5' }))).toEqual([3]);
    expect(ids(filterStudentsForMessage(students, { criterion: 'scored_more_than', assignmentId: 7, value: 5 }))).toEqual([1]);
    expect(filterStudentsForMessage(students, { criterion: 'scored_less_than', assignmentId: 7, value: '' })).toEqual([]);
  });
  test('missing assignments and inactivity', () => {
    expect(ids(filterStudentsForMessage(students, { criterion: 'missing_any' }))).toEqual([2]);
    expect(ids(filterStudentsForMessage(students, { criterion: 'inactive_days', value: 7, now }))).toEqual([2, 3, 4]);
  });
});

test('formatDuration', () => {
  expect(formatDuration(42)).toBe('42s');
  expect(formatDuration(600)).toBe('10m');
  expect(formatDuration(3720)).toBe('1h 2m');
  expect(formatDuration(null)).toBe('0s');
});
