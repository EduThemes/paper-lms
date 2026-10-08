import { describe, test, expect, vi, beforeEach } from 'vitest';

vi.mock('../services/api', () => ({ api: { getObservees: vi.fn() } }));

import { api } from '../services/api';
import { hasObserveesFromResponse, lookup, resetObserveeCache } from './useHasObservees';

describe('useHasObservees helpers (B099)', () => {
  beforeEach(() => {
    resetObserveeCache();
    vi.clearAllMocks();
  });

  test('a linked parent is detected from the observees response', () => {
    expect(hasObserveesFromResponse([{ id: 2, name: 'Jane Doe' }])).toBe(true);
    expect(hasObserveesFromResponse({ data: [{ id: 2 }] })).toBe(true);
    expect(hasObserveesFromResponse([])).toBe(false);
    expect(hasObserveesFromResponse(undefined)).toBe(false);
  });

  test('lookup fetches once per user and treats errors as "not an observer"', async () => {
    api.getObservees.mockResolvedValueOnce([{ id: 2 }]);
    expect(await lookup(5)).toBe(true);
    expect(await lookup(5)).toBe(true);
    expect(api.getObservees).toHaveBeenCalledTimes(1);

    api.getObservees.mockRejectedValueOnce(new Error('403'));
    expect(await lookup(6)).toBe(false);
  });
});
