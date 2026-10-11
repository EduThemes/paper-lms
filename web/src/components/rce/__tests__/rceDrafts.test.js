import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  AUTOSAVE_PREFIX, readRceDraft, writeRceDraft, scheduleRceDraftWrite, clearRceDraft, clearAllRceDrafts,
  setRceDraftKeyProvider, setRceDraftUser,
} from '../rceDrafts';

vi.mock('../../../services/api_rce_editor', () => ({
  fetchRceDraftKey: vi.fn(async () => null),
}));

// Minimal in-memory Storage (some Node versions shadow jsdom's localStorage).
function memoryStorage() {
  const map = new Map();
  return {
    get length() { return map.size; },
    key: (i) => Array.from(map.keys())[i] ?? null,
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => { map.set(k, String(v)); },
    removeItem: (k) => { map.delete(k); },
    clear: () => map.clear(),
    keys: () => Array.from(map.keys()),
  };
}

// Made-up 32-byte keys, base64 (what /users/self/rce_draft_key returns).
const KEY_A = btoa(String.fromCharCode(...Array.from({ length: 32 }, (_, i) => i + 1)));
const KEY_B = btoa(String.fromCharCode(...Array.from({ length: 32 }, (_, i) => 200 - i)));
const userA = async () => ({ key: KEY_A, user_id: 7, algorithm: 'AES-GCM' });
const userB = async () => ({ key: KEY_B, user_id: 8, algorithm: 'AES-GCM' });

/** Let queued promise callbacks (WebCrypto) run. */
async function settle() {
  for (let i = 0; i < 20; i += 1) await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
}

describe('rce drafts (B103, G012)', () => {
  let store;
  beforeEach(() => {
    store = memoryStorage();
    vi.stubGlobal('localStorage', store);
    Object.defineProperty(window, 'localStorage', { value: store, configurable: true });
    setRceDraftKeyProvider(userA);
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    clearAllRceDrafts();
  });

  it('stores drafts encrypted and scoped to the user', async () => {
    await writeRceDraft('page-1-new-body', '<p>secret plan</p>');
    const keys = store.keys();
    expect(keys).toEqual([`${AUTOSAVE_PREFIX}u7.page-1-new-body`]);
    const raw = store.getItem(keys[0]);
    expect(raw).not.toContain('secret plan');
    expect(JSON.parse(raw)).toMatchObject({ v: 2 });
    expect((await readRceDraft('page-1-new-body'))?.html).toBe('<p>secret plan</p>');
  });

  it('another user on the same browser cannot read the draft', async () => {
    await writeRceDraft('page-1-new-body', '<p>mine</p>');
    setRceDraftKeyProvider(userB);
    expect(await readRceDraft('page-1-new-body')).toBeNull();
    // Even forcing user B's id onto user A's ciphertext fails to decrypt.
    store.setItem(`${AUTOSAVE_PREFIX}u8.page-1-new-body`, store.getItem(`${AUTOSAVE_PREFIX}u7.page-1-new-body`));
    expect(await readRceDraft('page-1-new-body')).toBeNull();
  });

  it('the same user can restore the draft in a later session', async () => {
    await writeRceDraft('a', '<p>typed before the session expired</p>');
    setRceDraftKeyProvider(userA); // new session: key fetched again
    expect((await readRceDraft('a'))?.html).toBe('<p>typed before the session expired</p>');
  });

  it('keeps nothing when no key is available (fails closed)', async () => {
    setRceDraftKeyProvider(async () => { throw new Error('401'); });
    await writeRceDraft('a', '<p>x</p>');
    expect(store.keys()).toEqual([]);
    expect(await readRceDraft('a')).toBeNull();
  });

  it('deletes plaintext drafts left by older builds', async () => {
    store.setItem(`${AUTOSAVE_PREFIX}page-1-new-body`, JSON.stringify({ html: '<p>old</p>', savedAt: 1 }));
    await readRceDraft('page-1-new-body');
    expect(store.keys()).toEqual([]);
  });

  it('clearRceDraft removes the draft after a save', async () => {
    await writeRceDraft('announcement-1-new-message', '<p>posted</p>');
    expect((await readRceDraft('announcement-1-new-message'))?.html).toBe('<p>posted</p>');
    clearRceDraft('announcement-1-new-message');
    expect(await readRceDraft('announcement-1-new-message')).toBeNull();
    expect(store.keys()).toEqual([]);
  });

  it('clearing cancels a pending debounced write so the draft is not resurrected', async () => {
    vi.useFakeTimers();
    scheduleRceDraftWrite('page-1-new-body', '<p>typed</p>', 2000);
    clearRceDraft('page-1-new-body'); // save landed before the debounce fired
    vi.advanceTimersByTime(5000);
    vi.useRealTimers();
    await settle();
    expect(await readRceDraft('page-1-new-body')).toBeNull();
  });

  it('clearing discards a write that is already encrypting', async () => {
    const pending = writeRceDraft('page-1-new-body', '<p>typed</p>');
    clearRceDraft('page-1-new-body');
    await pending;
    expect(store.keys()).toEqual([]);
  });

  it('a debounced write lands when nothing clears it', async () => {
    vi.useFakeTimers();
    scheduleRceDraftWrite('page-1-new-body', '<p>typed</p>', 2000);
    vi.advanceTimersByTime(2000);
    vi.useRealTimers();
    // The write encrypts asynchronously after the timer fires.
    await vi.waitFor(async () => {
      expect((await readRceDraft('page-1-new-body'))?.html).toBe('<p>typed</p>');
    });
  });

  // The key provider stands in for the server: it answers for whoever holds
  // the session right now. Only the auth context's identity change
  // (setRceDraftUser) may drop the cached key, as in the real app.
  function sessionProvider() {
    let current = userA;
    const provider = vi.fn(() => current());
    provider.switchTo = (next) => { current = next; };
    return provider;
  }

  it('a different user signing in on the same tab does not reuse the previous key', async () => {
    const provider = sessionProvider();
    setRceDraftKeyProvider(provider);
    setRceDraftUser(7);
    await writeRceDraft('page-1-new-body', '<p>user A draft</p>');

    // A's session expires (no logout), then user B signs in.
    provider.switchTo(userB);
    setRceDraftUser(8);
    expect(await readRceDraft('page-1-new-body')).toBeNull();
    await writeRceDraft('page-1-new-body', '<p>user B draft</p>');
    expect(store.keys().sort()).toEqual([
      `${AUTOSAVE_PREFIX}u7.page-1-new-body`,
      `${AUTOSAVE_PREFIX}u8.page-1-new-body`,
    ]);

    // A signs back in and gets their own draft, not B's.
    provider.switchTo(userA);
    setRceDraftUser(7);
    expect((await readRceDraft('page-1-new-body'))?.html).toBe('<p>user A draft</p>');
  });

  it('the same user signing back in keeps the cached key', async () => {
    const provider = sessionProvider();
    setRceDraftKeyProvider(provider);
    setRceDraftUser(7);
    await writeRceDraft('a', '<p>x</p>');
    setRceDraftUser(7);
    expect((await readRceDraft('a'))?.html).toBe('<p>x</p>');
    expect(provider).toHaveBeenCalledTimes(1);
  });

  it("a pending write from the previous user is not saved as the new user's draft", async () => {
    const provider = sessionProvider();
    setRceDraftKeyProvider(provider);
    setRceDraftUser(7);
    vi.useFakeTimers();
    scheduleRceDraftWrite('page-1-new-body', '<p>typed by A</p>', 2000);
    provider.switchTo(userB);
    setRceDraftUser(8);
    vi.advanceTimersByTime(5000);
    vi.useRealTimers();
    await settle();
    expect(store.keys()).toEqual([]);
  });

  it('a key returned for someone other than the signed-in user is not used', async () => {
    setRceDraftUser(8);
    setRceDraftKeyProvider(userA); // stale response for user 7
    await writeRceDraft('a', '<p>x</p>');
    expect(store.keys()).toEqual([]);
  });

  it('clearAllRceDrafts removes only editor drafts and forgets the key (logout)', async () => {
    await writeRceDraft('a', '<p>1</p>');
    await writeRceDraft('b', '<p>2</p>');
    const inFlight = writeRceDraft('c', '<p>3</p>');
    store.setItem('unrelated', 'keep');
    clearAllRceDrafts();
    await inFlight;
    await settle();
    expect(store.keys().filter((k) => k.startsWith(AUTOSAVE_PREFIX))).toEqual([]);
    expect(store.getItem('unrelated')).toBe('keep');
  });
});
