import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { api, API_MAX_PER_PAGE } from '../api';

// B014: the API caps per_page at 100. Callers that ask for more (gradebook
// roster, a student's grades, the assignments index) must get every row,
// not a silently truncated first page.

function page(items, { next = false, url = '/api/v1/x' } = {}) {
  const links = [`<http://localhost${url}?page=1&per_page=100>; rel="current"`];
  if (next) links.push(`<http://localhost${url}?page=2&per_page=100>; rel="next"`);
  return {
    ok: true,
    json: async () => items,
    headers: new Headers({ Link: links.join(', ') }),
  };
}

const rows = (from, n) => Array.from({ length: n }, (_, i) => ({ id: from + i }));

describe('auto-paginated list helpers', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('getEnrollments(…, 200) walks server pages of 100', async () => {
    global.fetch
      .mockResolvedValueOnce(page(rows(1, 100), { next: true }))
      .mockResolvedValueOnce(page(rows(101, 60)));

    const { data } = await api.getEnrollments(7, 1, 200);

    expect(data).toHaveLength(160);
    expect(global.fetch).toHaveBeenCalledTimes(2);
    expect(global.fetch.mock.calls[0][0]).toBe('/api/v1/courses/7/enrollments?page=1&per_page=100');
    expect(global.fetch.mock.calls[1][0]).toBe('/api/v1/courses/7/enrollments?page=2&per_page=100');
  });

  test('getCourseSubmissions keeps the user filter on every page', async () => {
    global.fetch
      .mockResolvedValueOnce(page(rows(1, 100), { next: true }))
      .mockResolvedValueOnce(page(rows(101, 30)));

    const { data } = await api.getCourseSubmissions(7, 1, 10000, 'self');

    expect(data).toHaveLength(130);
    expect(global.fetch.mock.calls[0][0]).toBe('/api/v1/courses/7/submissions?user_id=self&page=1&per_page=100');
    expect(global.fetch.mock.calls[1][0]).toBe('/api/v1/courses/7/submissions?user_id=self&page=2&per_page=100');
  });

  test('stops when there is no rel="next"', async () => {
    global.fetch.mockResolvedValueOnce(page(rows(1, 12)));

    const { data } = await api.getAssignments(7, 1, 10000);

    expect(data).toHaveLength(12);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  test('a request within the server cap is a single call', async () => {
    global.fetch.mockResolvedValueOnce(page(rows(1, 25)));

    const { data } = await api.getAssignments(7, 1, 25);

    expect(data).toHaveLength(25);
    expect(global.fetch.mock.calls[0][0]).toBe('/api/v1/courses/7/assignments?page=1&per_page=25');
    expect(API_MAX_PER_PAGE).toBe(100);
  });
});
