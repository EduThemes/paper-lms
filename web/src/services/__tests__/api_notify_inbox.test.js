import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { announcementRepliesApi } from '../api_notify_inbox';

// The replies panel renders a whole thread, but the API caps a page at
// 100 rows: the list must walk every page instead of dropping reply 101+.

function page(items, { next = false } = {}) {
  const links = ['<http://localhost/api/v1/announcements/7/replies?page=1&per_page=100>; rel="current"'];
  if (next) links.push('<http://localhost/api/v1/announcements/7/replies?page=2&per_page=100>; rel="next"');
  return { ok: true, json: async () => items, headers: new Headers({ Link: links.join(', ') }) };
}

const rows = (from, n) => Array.from({ length: n }, (_, i) => ({ id: from + i, message: '<p>hi</p>' }));

describe('announcementRepliesApi.list', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  test('loads every page of a long thread', async () => {
    global.fetch
      .mockResolvedValueOnce(page(rows(1, 100), { next: true }))
      .mockResolvedValueOnce(page(rows(101, 5)));

    const replies = await announcementRepliesApi.list(7);

    expect(replies).toHaveLength(105);
    expect(global.fetch.mock.calls[0][0]).toBe('/api/v1/announcements/7/replies?page=1&per_page=100');
    expect(global.fetch.mock.calls[1][0]).toBe('/api/v1/announcements/7/replies?page=2&per_page=100');
  });
});
