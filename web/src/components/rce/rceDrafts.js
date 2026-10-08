/**
 * Encrypted, user-scoped localStorage persistence for RichContentEditorV2
 * autosave drafts (B103, G012).
 *
 * Canvas keeps RCE drafts in browser storage encrypted with a per-user key
 * and keyed by user; Paper's used to be plaintext and shared by everyone on
 * the browser. Now:
 *
 *  - Each draft lives under `paperlms.rce.u<userId>.<key>`, so another
 *    account on the same browser never sees (or is offered) it.
 *  - Its contents are AES-GCM encrypted with a key the server derives for
 *    the signed-in user (GET /users/self/rce_draft_key). The key is stable
 *    across sessions, so a draft written before a session expired can be
 *    restored after signing back in; without that user's session the stored
 *    value is unreadable.
 *  - Pages call clearRceDraft(key) after a successful save, and logout calls
 *    clearAllRceDrafts(), so drafts never outlive what they protect.
 *
 * When no key is available (signed out, server error, no WebCrypto) drafts
 * are simply not kept: autosave fails closed rather than writing plaintext.
 * Plaintext drafts written by older builds are deleted on first use.
 */
import { fetchRceDraftKey } from '../../services/api_rce_editor';

export const AUTOSAVE_PREFIX = 'paperlms.rce.';
const SCOPED_KEY = /^paperlms\.rce\.u\d+\./;
const FORMAT_VERSION = 2;

// Pending debounced writes, by key, so clearing a draft also cancels a write
// that would otherwise land after the save and resurrect it.
const pendingWrites = new Map();
// Bumped by clearRceDraft so an encryption already in flight is discarded.
const generations = new Map();

// Bumped by clearAllRceDrafts (logout) for the same reason.
let epoch = 0;

let keyProvider = fetchRceDraftKey;
let cipherPromise = null;
// The signed-in user as last reported by the auth context (setRceDraftUser).
// The cipher is cached, so without this a different user signing in on the
// same tab (session expired, then someone else logs in; masquerade start or
// end) kept the previous user's key and namespace: their drafts were offered
// to the new user and the new user's typing was stored as theirs.
let currentUserId = null;

function storage() {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null;
  } catch {
    return null;
  }
}

function subtle() {
  try {
    return globalThis.crypto?.subtle || null;
  } catch {
    return null;
  }
}

function toBase64(bytes) {
  let s = '';
  bytes.forEach((b) => { s += String.fromCharCode(b); });
  return btoa(s);
}

function fromBase64(b64) {
  const s = atob(b64);
  const out = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i += 1) out[i] = s.charCodeAt(i);
  return out;
}

function storageKeys() {
  const ls = storage();
  if (!ls) return [];
  const keys = [];
  try {
    for (let i = 0; i < ls.length; i += 1) {
      const k = ls.key(i);
      if (k && k.startsWith(AUTOSAVE_PREFIX)) keys.push(k);
    }
  } catch { /* ignore */ }
  return keys;
}

function removeKeys(keys) {
  const ls = storage();
  if (!ls) return;
  keys.forEach((k) => {
    try { ls.removeItem(k); } catch { /* ignore */ }
  });
}

/** Delete plaintext, unscoped drafts left by builds before G012. */
function purgeLegacyDrafts() {
  removeKeys(storageKeys().filter((k) => !SCOPED_KEY.test(k)));
}

/**
 * Resolve the signed-in user's draft cipher: { userId, key: CryptoKey }, or
 * null when drafts cannot be encrypted. Cached until clearAllRceDrafts().
 */
export function getDraftCipher() {
  if (!cipherPromise) {
    const p = (async () => {
      purgeLegacyDrafts();
      const s = subtle();
      if (!s) return null;
      const res = await keyProvider();
      if (!res?.key || !res?.user_id) return null;
      // A key for someone other than the signed-in user (a lookup that
      // raced an identity change) is never used.
      if (currentUserId != null && String(res.user_id) !== currentUserId) return null;
      const key = await s.importKey('raw', fromBase64(res.key), { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
      return { userId: String(res.user_id), key };
    })().catch(() => null);
    cipherPromise = p;
    // A failed lookup (e.g. offline) may succeed later; don't pin the null.
    p.then((c) => { if (!c && cipherPromise === p) cipherPromise = null; });
  }
  return cipherPromise;
}

/**
 * Tell the draft store who is signed in (AuthContext calls this whenever
 * its user changes). When the identity changes to a different user, the
 * cached key is dropped and pending writes, which hold the previous user's
 * typing, are cancelled; stored drafts are kept so each user can still
 * restore their own. Signing out (null) changes nothing here: an expired
 * session keeps the drafts so the same author can sign back in and restore,
 * and logout clears everything through clearAllRceDrafts().
 */
export function setRceDraftUser(userId) {
  if (userId == null || userId === '') return;
  const next = String(userId);
  if (currentUserId !== null && currentUserId !== next) {
    for (const key of Array.from(pendingWrites.keys())) cancelRceDraftWrite(key);
    epoch += 1;
    cipherPromise = null;
  }
  currentUserId = next;
}

/** Test hook: replace the key source and drop any cached key. */
export function setRceDraftKeyProvider(provider) {
  keyProvider = provider || fetchRceDraftKey;
  cipherPromise = null;
}

function storageKeyFor(userId, key) {
  return `${AUTOSAVE_PREFIX}u${userId}.${key}`;
}

/** Read a draft for the signed-in user. Resolves { html, savedAt } or null. */
export async function readRceDraft(key) {
  const ls = storage();
  if (!key || !ls) return null;
  const cipher = await getDraftCipher();
  if (!cipher) return null;
  try {
    const raw = ls.getItem(storageKeyFor(cipher.userId, key));
    if (!raw) return null;
    const rec = JSON.parse(raw);
    if (rec?.v !== FORMAT_VERSION || !rec.iv || !rec.ct) return null;
    const plain = await subtle().decrypt({ name: 'AES-GCM', iv: fromBase64(rec.iv) }, cipher.key, fromBase64(rec.ct));
    const draft = JSON.parse(new TextDecoder().decode(plain));
    return draft && typeof draft.html === 'string' ? draft : null;
  } catch {
    // Wrong key (server secret rotated) or corrupt: the draft is unusable.
    return null;
  }
}

/** Encrypt and persist a draft now. Silent on quota / crypto errors. */
export async function writeRceDraft(key, html) {
  if (!key || !storage()) return;
  const generation = generations.get(key) || 0;
  const startEpoch = epoch;
  const cipher = await getDraftCipher();
  if (!cipher) return;
  try {
    const iv = new Uint8Array(12);
    globalThis.crypto.getRandomValues(iv);
    const payload = new TextEncoder().encode(JSON.stringify({ html, savedAt: Date.now() }));
    const ct = new Uint8Array(await subtle().encrypt({ name: 'AES-GCM', iv }, cipher.key, payload));
    // The draft was cleared (content saved) while we were encrypting.
    if ((generations.get(key) || 0) !== generation || epoch !== startEpoch) return;
    storage().setItem(storageKeyFor(cipher.userId, key), JSON.stringify({ v: FORMAT_VERSION, iv: toBase64(iv), ct: toBase64(ct) }));
  } catch {
    /* quota exceeded / crypto failure — ignore */
  }
}

/** Persist a draft after `delayMs`, replacing any pending write for the key. */
export function scheduleRceDraftWrite(key, html, delayMs) {
  if (!key) return;
  cancelRceDraftWrite(key);
  const id = setTimeout(() => {
    pendingWrites.delete(key);
    writeRceDraft(key, html);
  }, delayMs);
  pendingWrites.set(key, id);
}

/** Cancel a pending debounced write for the key, if any. */
export function cancelRceDraftWrite(key) {
  const id = pendingWrites.get(key);
  if (id !== undefined) {
    clearTimeout(id);
    pendingWrites.delete(key);
  }
}

/** Remove a draft (and any pending write for it). Call after a successful save. */
export function clearRceDraft(key) {
  if (!key) return;
  cancelRceDraftWrite(key);
  generations.set(key, (generations.get(key) || 0) + 1);
  const suffix = `.${key}`;
  removeKeys(storageKeys().filter((k) => SCOPED_KEY.test(k) && k.endsWith(suffix) && k.replace(SCOPED_KEY, '') === key));
  removeKeys([AUTOSAVE_PREFIX + key]);
}

/** Remove every editor draft and forget the cached key (logout / account switch). */
export function clearAllRceDrafts() {
  for (const key of Array.from(pendingWrites.keys())) cancelRceDraftWrite(key);
  epoch += 1;
  cipherPromise = null;
  currentUserId = null;
  removeKeys(storageKeys());
}
