/** localStorage that never throws (private windows, blocked storage, full quota). */

function store() {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
}

export function readJson(key) {
  try {
    const raw = store()?.getItem(key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function writeJson(key, value) {
  try {
    const s = store();
    if (!s) return false;
    s.setItem(key, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}

export function removeKey(key) {
  try {
    store()?.removeItem(key);
  } catch {
    /* ignore */
  }
}
