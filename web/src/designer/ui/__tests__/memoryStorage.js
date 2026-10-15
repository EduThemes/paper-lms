import { vi } from 'vitest';

/** An in-memory localStorage (the test runtime may not provide one). Returns the store. */
export function stubLocalStorage() {
  const map = new Map();
  const store = {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
    clear: () => map.clear(),
    key: (i) => [...map.keys()][i] ?? null,
    get length() {
      return map.size;
    },
  };
  vi.stubGlobal('localStorage', store);
  return store;
}

/** A localStorage that throws on every call (private mode, blocked storage). */
export function stubThrowingStorage() {
  const boom = () => {
    throw new Error('SecurityError');
  };
  vi.stubGlobal('localStorage', { getItem: boom, setItem: boom, removeItem: boom, clear: boom });
}
