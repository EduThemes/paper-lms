// Deep-link return after login (G160).
//
// When a signed-out visitor opens a protected link (a notification email,
// a bookmark) or their session expires mid-task, we remember where they
// were and send them back there after they sign in — including after an
// SSO round trip, which leaves the SPA, so the target lives in
// sessionStorage rather than only in router state.

const KEY = 'paperlms_return_to';
const EXPIRED_KEY = 'paperlms_session_expired';

const storage = () => {
  try {
    return typeof sessionStorage !== 'undefined' ? sessionStorage : null;
  } catch {
    return null;
  }
};

// safeReturnPath accepts only same-origin, in-app paths: it must start
// with a single "/", must not be protocol-relative ("//evil") or contain a
// backslash, and must not point back at the auth pages.
export function safeReturnPath(path) {
  if (typeof path !== 'string') return null;
  const p = path.trim();
  if (!p.startsWith('/') || p.startsWith('//') || p.includes('\\')) return null;
  for (let i = 0; i < p.length; i += 1) {
    if (p.charCodeAt(i) < 0x20) return null; // control characters
  }
  const pathname = p.split(/[?#]/)[0];
  if (pathname === '/' || pathname === '') return null;
  if (/^\/(login|logout|mfa\/verify|auth\/password-set)(\/|$)/.test(pathname)) return null;
  if (p.length > 2000) return null;
  return p;
}

export function locationToPath(loc) {
  if (!loc) return null;
  if (typeof loc === 'string') return loc;
  return `${loc.pathname || ''}${loc.search || ''}${loc.hash || ''}`;
}

export function rememberReturnTo(loc) {
  const path = safeReturnPath(locationToPath(loc));
  const s = storage();
  if (!s) return;
  try {
    if (path) s.setItem(KEY, path);
  } catch {
    /* storage full / blocked */
  }
}

export function peekReturnTo() {
  const s = storage();
  if (!s) return null;
  try {
    return safeReturnPath(s.getItem(KEY));
  } catch {
    return null;
  }
}

// consumeReturnTo returns the remembered path once and forgets it.
export function consumeReturnTo() {
  const path = peekReturnTo();
  const s = storage();
  try {
    s?.removeItem(KEY);
  } catch {
    /* ignore */
  }
  return path;
}

export function markSessionExpired() {
  try {
    storage()?.setItem(EXPIRED_KEY, '1');
  } catch {
    /* ignore */
  }
}

// consumeSessionExpired reports (once) whether the last sign-out was a
// session expiry rather than a deliberate logout.
export function consumeSessionExpired() {
  const s = storage();
  try {
    const v = s?.getItem(EXPIRED_KEY) === '1';
    s?.removeItem(EXPIRED_KEY);
    return v;
  } catch {
    return false;
  }
}
