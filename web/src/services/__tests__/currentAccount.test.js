import { describe, test, expect, beforeEach, afterEach, vi } from 'vitest';
import { setCurrentAccountId, getCurrentAccountId } from '../currentAccount';
import { api } from '../api';

// B069: account-scoped admin routes must name the signed-in user's own
// account (the backend 404s any other one), so the client no longer
// hard-codes account 1.
describe('current account', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [],
      headers: new Headers(),
    });
  });
  afterEach(() => {
    setCurrentAccountId(1);
    vi.restoreAllMocks();
  });

  test('defaults to the root account', () => {
    expect(getCurrentAccountId()).toBe(1);
  });

  test('ignores invalid ids', () => {
    setCurrentAccountId(undefined);
    expect(getCurrentAccountId()).toBe(1);
    setCurrentAccountId('abc');
    expect(getCurrentAccountId()).toBe(1);
  });

  test('account-scoped API helpers use the signed-in tenant', async () => {
    setCurrentAccountId(5);
    await api.getEnrollmentTerms();
    expect(global.fetch.mock.calls[0][0]).toContain('/accounts/5/terms');
  });
});
