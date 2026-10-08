import {
  isEntryUnread,
  isEntryDeleted,
  wasEntryEdited,
  countUnreadEntries,
  unwrapList,
} from '../discussionEntries';
import { fetchAllPages } from '../fetchAllPages';

describe('discussionEntries helpers', () => {
  test('read_state is a boolean: false means unread (B156)', () => {
    expect(isEntryUnread({ read_state: false })).toBe(true);
    expect(isEntryUnread({ read_state: true })).toBe(false);
    // The old string comparison never matched anything the API sends.
    expect(isEntryUnread({ read_state: 'unread' })).toBe(false);
    expect(isEntryUnread({ read_state: false, workflow_state: 'deleted' })).toBe(false);
  });

  test('countUnreadEntries walks nested replies', () => {
    const tree = [
      { id: 1, read_state: false, replies: [{ id: 2, read_state: false, replies: [{ id: 3, read_state: true }] }] },
      { id: 4, read_state: true, replies: [] },
    ];
    expect(countUnreadEntries(tree)).toBe(2);
  });

  test('a single edit counts as edited (B160)', () => {
    expect(wasEntryEdited({ version_count: 0 })).toBe(false);
    expect(wasEntryEdited({ version_count: 1 })).toBe(true);
    expect(wasEntryEdited({ version_count: 2 })).toBe(true);
  });

  test('unwrapList returns the array inside an api.js { data } result', () => {
    // The checkpoints panel calls .map on what the adapter returns; the raw
    // api.js result is an object, which crashed the topic page.
    expect(unwrapList({ data: [{ id: 1 }], pagination: {} })).toEqual([{ id: 1 }]);
    expect(unwrapList([{ id: 2 }])).toEqual([{ id: 2 }]);
    expect(unwrapList({ data: null })).toEqual([]);
    expect(unwrapList(undefined)).toEqual([]);
  });

  test('deleted placeholder detection (B157)', () => {
    expect(isEntryDeleted({ workflow_state: 'deleted' })).toBe(true);
    expect(isEntryDeleted({ workflow_state: 'active' })).toBe(false);
  });
});

describe('fetchAllPages (B170)', () => {
  test('follows rel="next" until the last page', async () => {
    const pages = {
      1: { data: Array.from({ length: 100 }, (_, i) => i), pagination: { next: '/x?page=2' } },
      2: { data: Array.from({ length: 100 }, (_, i) => 100 + i), pagination: { next: '/x?page=3' } },
      3: { data: [200, 201], pagination: {} },
    };
    const fetchPage = vi.fn((page) => Promise.resolve(pages[page]));
    const all = await fetchAllPages(fetchPage);
    expect(all).toHaveLength(202);
    expect(all[201]).toBe(201);
    expect(fetchPage).toHaveBeenCalledTimes(3);
    expect(fetchPage).toHaveBeenNthCalledWith(3, 3);
  });

  test('stops on a response without pagination info', async () => {
    const fetchPage = vi.fn(() => Promise.resolve({ data: [1, 2] }));
    expect(await fetchAllPages(fetchPage)).toEqual([1, 2]);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  test('is bounded by maxPages', async () => {
    const fetchPage = vi.fn(() => Promise.resolve({ data: [1], pagination: { next: 'more' } }));
    const all = await fetchAllPages(fetchPage, { maxPages: 4 });
    expect(all).toHaveLength(4);
    expect(fetchPage).toHaveBeenCalledTimes(4);
  });
});
