// Iframe embed policy for rich content (G001).
//
// The host list is shared with the server sanitizer
// (internal/security/htmlsanitize/iframe_policy.go) through
// trustedIframeHosts.json; a Go test fails when the two drift. The server
// strips every other iframe src on write; the viewer and the editor apply the
// same list so content stored before the policy existed is held to it too.
import policy from './trustedIframeHosts.json';

export const TRUSTED_IFRAME_HOSTS = policy.hosts;

export const TRUSTED_IFRAME_SRC = new RegExp(
  `^(https://(${policy.hosts.join('|')})|${policy.sameOriginPath})`,
  'i',
);

/** True when an iframe with this src is allowed in rich content. */
export function isTrustedIframeSrc(src) {
  return typeof src === 'string' && TRUSTED_IFRAME_SRC.test(src.trim());
}

// Mirrors iframeAllowFeatures: no camera, microphone, display-capture,
// clipboard-read or midi for author-chosen embeds.
const ALLOW_FEATURES = '(accelerometer|autoplay|clipboard-write|encrypted-media|fullscreen|gyroscope|picture-in-picture|screen-wake-lock|web-share|xr-spatial-tracking)';
const ALLOW_ENTRY = `${ALLOW_FEATURES}( +('self'|'src'|'none'|\\*))?`;

/** Mirrors IframeAllowAttr: the allow="..." values the server keeps. */
export const IFRAME_ALLOW_RE = new RegExp(`^ *${ALLOW_ENTRY}( *; *${ALLOW_ENTRY})* *;? *$`);

/** Default permissions for an embed the editor builds itself. */
export const DEFAULT_IFRAME_ALLOW = 'autoplay; fullscreen; encrypted-media; picture-in-picture; clipboard-write';

function safeURL(raw) {
  try {
    const u = new URL(String(raw).trim());
    return u.protocol === 'https:' || u.protocol === 'http:' ? u : null;
  } catch {
    return null;
  }
}

/**
 * Turn what a teacher pastes (a share/watch URL of a known provider, an
 * embed URL, or a whole <iframe ...> embed code) into iframe attributes the
 * sanitizer will keep. Returns null when the source is not on the
 * allowlist.
 *
 * @param {string} input
 * @returns {{src: string, title?: string, width?: string, height?: string, allow?: string} | null}
 */
export function toEmbedAttrs(input) {
  const raw = String(input || '').trim();
  if (!raw) return null;

  // Pasted embed code: take its src and the attributes the policy keeps.
  if (/^<iframe[\s>]/i.test(raw)) {
    const doc = new DOMParser().parseFromString(raw, 'text/html');
    const frame = doc.querySelector('iframe');
    const src = frame?.getAttribute('src');
    if (!src) return null;
    const attrs = toEmbedAttrs(src);
    if (!attrs) return null;
    const title = frame.getAttribute('title');
    const width = frame.getAttribute('width');
    const height = frame.getAttribute('height');
    const allow = frame.getAttribute('allow');
    if (title) attrs.title = title;
    if (width && /^[0-9]+%?$/.test(width)) attrs.width = width;
    if (height && /^[0-9]+%?$/.test(height)) attrs.height = height;
    if (allow && IFRAME_ALLOW_RE.test(allow)) attrs.allow = allow;
    return attrs;
  }

  const u = safeURL(raw);
  if (!u) return null;
  const host = u.hostname.toLowerCase();
  const path = u.pathname;
  let src = null;

  if (host === 'youtu.be') {
    const id = path.replace(/^\//, '');
    if (/^[\w-]{6,}$/.test(id)) src = `https://www.youtube.com/embed/${id}`;
  } else if (/(^|\.)youtube\.com$/.test(host)) {
    if (path === '/watch') {
      const id = u.searchParams.get('v');
      if (id && /^[\w-]{6,}$/.test(id)) src = `https://www.youtube.com/embed/${id}`;
    } else if (path.startsWith('/shorts/')) {
      const id = path.split('/')[2];
      if (id && /^[\w-]{6,}$/.test(id)) src = `https://www.youtube.com/embed/${id}`;
    } else if (path.startsWith('/embed/')) {
      src = `https://www.youtube.com${path}`;
    }
  } else if (host === 'vimeo.com' || host === 'www.vimeo.com') {
    const id = path.split('/').filter(Boolean)[0];
    if (/^\d+$/.test(id || '')) src = `https://player.vimeo.com/video/${id}`;
  } else if (host === 'www.loom.com' || host === 'loom.com') {
    const m = path.match(/^\/(share|embed)\/([\w-]+)/);
    if (m) src = `https://www.loom.com/embed/${m[2]}`;
  } else if (host === 'docs.google.com') {
    // Docs/Sheets/Slides "edit"/"view" links become their embeddable form.
    const m = path.match(/^\/(document|spreadsheets|presentation|drawings)\/d\/([\w-]+)/);
    if (m && !/\/(pub|embed|preview|pubhtml)$/.test(path)) {
      src = m[1] === 'presentation'
        ? `https://docs.google.com/presentation/d/${m[2]}/embed`
        : `https://docs.google.com/${m[1]}/d/${m[2]}/preview`;
    } else if (/^\/forms\/d\/(e\/)?[\w-]+\/viewform/.test(path)) {
      u.searchParams.set('embedded', 'true');
      src = u.toString();
    }
  } else if (host === 'drive.google.com') {
    const m = path.match(/^\/file\/d\/([\w-]+)/);
    if (m) src = `https://drive.google.com/file/d/${m[1]}/preview`;
  }

  if (!src && u.protocol === 'https:') src = u.toString();
  return src && isTrustedIframeSrc(src) ? { src } : null;
}
