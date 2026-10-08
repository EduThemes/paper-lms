import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  AUTOSAVE_PREFIX, readRceDraft, writeRceDraft, scheduleRceDraftWrite, clearRceDraft, clearAllRceDrafts,
} from '../rceDrafts';

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

describe('rce drafts (B103)', () => {
  let store;
  beforeEach(() => {
    store = memoryStorage();
    vi.stubGlobal('localStorage', store);
    Object.defineProperty(window, 'localStorage', { value: store, configurable: true });
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('clearRceDraft removes the draft after a save', () => {
    writeRceDraft('announcement-1-new-message', '<p>posted</p>');
    expect(readRceDraft('announcement-1-new-message')?.html).toBe('<p>posted</p>');
    clearRceDraft('announcement-1-new-message');
    expect(readRceDraft('announcement-1-new-message')).toBeNull();
  });

  it('clearing cancels a pending debounced write so the draft is not resurrected', () => {
    scheduleRceDraftWrite('page-1-new-body', '<p>typed</p>', 2000);
    clearRceDraft('page-1-new-body'); // save landed before the debounce fired
    vi.advanceTimersByTime(5000);
    expect(readRceDraft('page-1-new-body')).toBeNull();
  });

  it('a debounced write lands when nothing clears it', () => {
    scheduleRceDraftWrite('page-1-new-body', '<p>typed</p>', 2000);
    vi.advanceTimersByTime(2000);
    expect(readRceDraft('page-1-new-body')?.html).toBe('<p>typed</p>');
  });

  it('clearAllRceDrafts removes only editor drafts (logout)', () => {
    writeRceDraft('a', '<p>1</p>');
    writeRceDraft('b', '<p>2</p>');
    scheduleRceDraftWrite('c', '<p>3</p>', 2000);
    store.setItem('unrelated', 'keep');
    clearAllRceDrafts();
    vi.advanceTimersByTime(5000);
    const keys = store.keys().filter((k) => k.startsWith(AUTOSAVE_PREFIX));
    expect(keys).toEqual([]);
    expect(store.getItem('unrelated')).toBe('keep');
  });
});
