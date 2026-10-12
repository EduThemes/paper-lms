import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from '../api';

// B072: the roster helpers must read every page — the server caps
// per_page at 100, so one per_page=200 request silently dropped people.
// B073: Add People looks users up through the course-scoped endpoint,
// not the admin-only GET /users.

const page = (n, count) => Array.from({ length: count }, (_, i) => ({ id: n * 1000 + i, user_id: n * 1000 + i }));

const okJSON = (body, link) => ({
  ok: true,
  status: 200,
  json: async () => body,
  headers: new Headers(link ? { Link: link } : {}),
});

describe('enrollment API helpers', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('getAllEnrollments follows pages until a short page', async () => {
    global.fetch
      .mockResolvedValueOnce(okJSON(page(1, 100)))
      .mockResolvedValueOnce(okJSON(page(2, 100)))
      .mockResolvedValueOnce(okJSON(page(3, 37)));

    const { data } = await api.getAllEnrollments(9);
    expect(data).toHaveLength(237);
    expect(global.fetch).toHaveBeenCalledTimes(3);
    expect(global.fetch.mock.calls[0][0]).toContain('/courses/9/enrollments?page=1&per_page=100');
    expect(global.fetch.mock.calls[2][0]).toContain('page=3');
  });

  test('getAllEnrollments stops on an empty page', async () => {
    global.fetch
      .mockResolvedValueOnce(okJSON(page(1, 100)))
      .mockResolvedValueOnce(okJSON([]));
    const { data } = await api.getAllEnrollments(9);
    expect(data).toHaveLength(100);
    expect(global.fetch).toHaveBeenCalledTimes(2);
  });

  test('searchEnrollableUsers calls the course-scoped lookup', async () => {
    global.fetch.mockResolvedValueOnce(okJSON([{ id: 5, name: 'Sam Student' }]));
    const users = await api.searchEnrollableUsers(9, 'sam@example.com');
    expect(users).toEqual([{ id: 5, name: 'Sam Student' }]);
    expect(global.fetch.mock.calls[0][0]).toContain('/courses/9/enrollable_users?search_term=sam%40example.com');
  });

  test('getMyCourseEnrollments reads the self endpoint', async () => {
    global.fetch.mockResolvedValueOnce(okJSON({ enrollments: [], is_admin: false }));
    const res = await api.getMyCourseEnrollments(9);
    expect(res).toEqual({ enrollments: [], is_admin: false });
    expect(global.fetch.mock.calls[0][0]).toContain('/courses/9/enrollments/self');
  });
});
