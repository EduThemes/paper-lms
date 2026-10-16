import React, { useMemo, useEffect, useRef, useState, useCallback, useContext } from 'react';
import DOMPurify from 'dompurify';
import { TRUSTED_IFRAME_SRC, IFRAME_ALLOW_RE } from '../utils/iframePolicy';
import { renderMathInElement } from '../utils/mathRender';
import { enhanceUserContent } from '../utils/enhanceUserContent';
import { Volume2, Square } from 'lucide-react';
import { ReadingPrefsContext } from '../contexts/ReadingPrefsContext';
import { htmlToText } from '../utils/html';
import { CONTENT_CONTAINMENT, CONTAINED_CLASS } from './contentContainment';

/* --- DOMPurify Configuration ---------------------------------------------- */

// Course content imported from Canvas / IMSCC commonly uses tags that a tight
// allowlist would silently drop (collapsible <details>/<summary>, semantic
// landmarks, embedded media, definition lists, Canvas equation images with
// data-equation-content, etc.). Our philosophy is to keep the rich HTML
// authors put in and block what's dangerous.
//
// This sanitizer also renders student-authored HTML (submission bodies,
// discussion replies) in teacher views, so it must not allow anything that
// can restyle or overlay the app or post data elsewhere (B002): no <style>
// elements, no <form> / form* attributes, no plugin <object>/<embed>, iframes
// only from the trusted media hosts the server allows, no fixed/sticky
// positioning, and data: URIs only for images.
//
// FORBID_TAGS removes proven XSS vectors. Inline event handlers (on*) are
// stripped by DOMPurify automatically.
const purifyConfig = {
  ALLOWED_TAGS: [
    // Sectioning + text content
    'p', 'br', 'hr', 'div', 'span', 'pre', 'blockquote',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'header', 'footer', 'main', 'nav', 'section', 'article', 'aside',
    'address', 'figure', 'figcaption', 'details', 'summary', 'dialog',
    // Inline text semantics
    'a', 'b', 'i', 'u', 's', 'em', 'strong', 'mark', 'small', 'q', 'cite',
    'abbr', 'time', 'kbd', 'samp', 'var', 'code', 'sub', 'sup', 'del', 'ins',
    'bdi', 'bdo', 'wbr', 'ruby', 'rt', 'rp', 'dfn',
    // Legacy Canvas/TinyMCE colour + size markup (the server allows it too)
    'font',
    // Lists
    'ul', 'ol', 'li', 'dl', 'dt', 'dd',
    // Tables
    'table', 'caption', 'colgroup', 'col', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td',
    // Media / embeds (iframes are further restricted to TRUSTED_IFRAME_SRC)
    'img', 'picture', 'source', 'video', 'audio', 'track', 'iframe',
    // Form controls without <form> (task-list checkboxes, read-only widgets).
    // With no <form> element and no form* attributes they cannot submit
    // anything anywhere.
    'fieldset', 'legend', 'label', 'input', 'button', 'select', 'option',
    'optgroup', 'textarea', 'output', 'progress', 'meter', 'datalist',
    // SVG (used heavily in math content)
    'svg', 'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polyline', 'polygon',
    'text', 'tspan', 'defs', 'use', 'symbol', 'pattern', 'mask', 'clipPath',
    'linearGradient', 'radialGradient', 'stop', 'filter', 'feGaussianBlur',
    'feOffset', 'feMerge', 'feMergeNode', 'foreignObject', 'marker', 'image',
    'title', 'desc',
    // MathML (some Canvas content uses native MathML)
    'math', 'mrow', 'mi', 'mn', 'mo', 'msup', 'msub', 'msubsup', 'mfrac',
    'mroot', 'msqrt', 'mtext', 'mspace', 'mtable', 'mtr', 'mtd', 'mfenced',
    'mover', 'munder', 'munderover', 'mstyle',
  ],
  // ALLOW_DATA_ATTR + ADD_ATTR (below) cover all data-* attributes including
  // Canvas-specific ones like data-equation-content / data-api-endpoint.
  ALLOWED_ATTR: [
    // Common
    'href', 'src', 'alt', 'title', 'class', 'id', 'name', 'lang', 'dir',
    'target', 'rel', 'style', 'width', 'height', 'role', 'aria-label',
    'aria-hidden', 'aria-describedby', 'aria-labelledby', 'tabindex',
    'contenteditable',
    // Tables (colwidth carries resized column widths for the editor)
    'colspan', 'rowspan', 'scope', 'headers', 'span', 'colwidth',
    // Ordered lists, legacy <font>
    'start', 'reversed', 'color', 'face',
    // Anchors / links
    'download', 'hreflang', 'type', 'referrerpolicy',
    // Images / picture / video / audio / iframe
    'srcset', 'sizes', 'loading', 'decoding', 'fetchpriority', 'crossorigin', 'usemap',
    // <picture><source media=… srcset=…> art direction (designed pages)
    'media',
    'controls', 'autoplay', 'loop', 'muted', 'playsinline', 'preload', 'poster',
    'allowfullscreen', 'allow', 'frameborder', 'sandbox', 'longdesc',
    // Form-control attributes (no form/formaction/formmethod/formtarget:
    // controls must never be able to submit to another origin)
    'value', 'placeholder', 'min', 'max', 'step', 'pattern', 'required',
    'readonly', 'disabled', 'checked', 'selected', 'multiple', 'accept',
    'list', 'maxlength', 'minlength', 'size',
    'for', 'open',
    // Media tracks
    'kind', 'srclang', 'default', 'label',
    // SVG / MathML
    'viewBox', 'xmlns', 'fill', 'stroke', 'stroke-width', 'stroke-linecap',
    'stroke-linejoin', 'stroke-dasharray', 'stroke-opacity', 'fill-opacity',
    'opacity', 'd', 'cx', 'cy', 'r', 'rx', 'ry', 'x', 'y', 'x1', 'y1', 'x2',
    'y2', 'points', 'transform', 'gradientUnits', 'gradientTransform',
    'offset', 'stop-color', 'stop-opacity', 'preserveAspectRatio', 'mathvariant',
    'mathsize', 'mathcolor', 'displaystyle', 'scriptlevel',
  ],
  ALLOW_DATA_ATTR: true,
  // Hard block: known XSS vectors and tags that don't belong in user content.
  FORBID_TAGS: ['script', 'base', 'meta', 'link', 'style', 'form', 'object', 'embed', 'param'],
  FORBID_ATTR: ['xmlns:xlink', 'form', 'formaction', 'formmethod', 'formenctype', 'formtarget', 'formnovalidate', 'srcdoc'],
  // Allow known-safe URI schemes + relative URLs. data: is limited to raster
  // and SVG images (an SVG loaded through <img> cannot run script).
  ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel|ftp|sftp|blob):|data:image\/(?:png|gif|jpe?g|webp|bmp|svg\+xml)[;,]|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i,
};

// The iframe allowlist mirrors the server sanitizer (G001); see
// utils/iframePolicy.js. Re-exported for existing importers.
export { TRUSTED_IFRAME_SRC };

// A private DOMPurify instance so these hooks never affect other callers.
let purifier = null;
function getPurifier() {
  if (purifier) return purifier;
  purifier = DOMPurify(typeof window !== 'undefined' ? window : undefined);
  purifier.addHook('uponSanitizeElement', (node, data) => {
    if (data.tagName === 'iframe') {
      const src = node.getAttribute && node.getAttribute('src');
      if (!src || !TRUSTED_IFRAME_SRC.test(src.trim())) {
        node.parentNode?.removeChild(node);
      }
    }
  });
  purifier.addHook('afterSanitizeAttributes', (node) => {
    // An iframe's allow= list must be one the server keeps (no camera,
    // microphone, screen capture, clipboard-read or midi; no origins).
    if (node.nodeName === 'IFRAME' && node.hasAttribute('allow')
      && !IFRAME_ALLOW_RE.test(node.getAttribute('allow'))) {
      node.removeAttribute('allow');
    }
    // Inline styles may not pin content over the application chrome: only
    // static / relative / absolute positioning survives. An allowlist, not a
    // fixed|sticky blocklist, so -webkit-sticky, var(--p) (which could resolve
    // to fixed) and other spellings go too. Absolute boxes stay inside the
    // content root, which is a containment box (contentContainment.js).
    if (node.style && node.style.position
      && !/^(static|relative|absolute)$/i.test(node.style.position.trim())) {
      node.style.removeProperty('position');
      if (!node.getAttribute('style')) node.removeAttribute('style');
    }
    // Form controls in content are display-only (task-list checkboxes etc.),
    // so content can't present a working credential prompt.
    if (/^(INPUT|BUTTON|SELECT|TEXTAREA)$/.test(node.nodeName)) {
      node.setAttribute('disabled', '');
    }
  });
  return purifier;
}

/**
 * Sanitize an HTML string using DOMPurify.
 * Exported for use in other components that need to sanitize HTML
 * before passing it to dangerouslySetInnerHTML.
 */
export function sanitizeHTML(dirty) {
  if (!dirty) return '';
  return getPurifier().sanitize(String(dirty), purifyConfig);
}

/* --- Rich Content Viewer ------------------------------------------------ */

/* --- Read-aloud (browser SpeechSynthesis) --------------------------------- */

function ReadAloudButton({ text }) {
  const [speaking, setSpeaking] = useState(false);
  const supported = typeof window !== 'undefined' && 'speechSynthesis' in window;

  const stop = useCallback(() => {
    if (!supported) return;
    window.speechSynthesis.cancel();
    setSpeaking(false);
  }, [supported]);

  useEffect(() => () => stop(), [stop]);

  const speak = useCallback(() => {
    if (!supported || !text) return;
    window.speechSynthesis.cancel();
    const utter = new window.SpeechSynthesisUtterance(text);
    utter.rate = 0.95;
    utter.pitch = 1;
    utter.onend = () => setSpeaking(false);
    utter.onerror = () => setSpeaking(false);
    window.speechSynthesis.speak(utter);
    setSpeaking(true);
  }, [supported, text]);

  if (!supported || !text) return null;

  const Icon = speaking ? Square : Volume2;
  return (
    <button
      type="button"
      onClick={speaking ? stop : speak}
      aria-label={speaking ? 'Stop reading aloud' : 'Read aloud'}
      className="inline-flex items-center gap-1.5 rounded-md border border-input bg-background px-2.5 py-1.5 text-xs font-medium text-foreground shadow-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
    >
      <Icon className="h-3.5 w-3.5" />
      {speaking ? 'Stop' : 'Read aloud'}
    </button>
  );
}

/** Strip HTML to plain text for TTS (inert parse — see utils/html.js). */
function htmlToPlainText(html) {
  return htmlToText(html);
}

export default function RichContentViewer({ content, className }) {
  const sanitized = useMemo(() => sanitizeHTML(content), [content]);
  const plainText = useMemo(() => htmlToPlainText(sanitized), [sanitized]);
  const containerRef = useRef(null);
  // Tolerate use outside <ReadingPrefsProvider> (e.g. tests) — useContext returns null safely.
  const readingCtx = useContext(ReadingPrefsContext);
  const prefs = readingCtx?.prefs ?? null;

  useEffect(() => {
    if (containerRef.current && sanitized) {
      enhanceUserContent(containerRef.current);
    }
  }, [sanitized]);

  if (!sanitized) return null;

  const showTTS = prefs?.ttsEnabled === true;

  return (
    <div className="reading-surface">
      {showTTS && (
        <div className="mb-2 flex justify-end">
          <ReadAloudButton text={plainText} />
        </div>
      )}
      <div
        ref={containerRef}
        className={[
          // Tailwind prose for beautiful typography
          'prose prose-sm sm:prose-base max-w-none',
          // Headings
          'prose-headings:font-semibold prose-headings:text-text-primary',
          // Links
          'prose-a:text-brand-600 prose-a:underline hover:prose-a:text-brand-800',
          // Images
          'prose-img:rounded-lg prose-img:shadow-sm',
          // Tables
          'prose-table:border-collapse',
          'prose-th:border prose-th:border-border-strong prose-th:bg-surface-1 prose-th:px-3 prose-th:py-2 prose-th:text-left prose-th:text-sm prose-th:font-medium prose-th:text-text-secondary',
          'prose-td:border prose-td:border-border-strong prose-td:px-3 prose-td:py-2 prose-td:text-sm',
          // Blockquotes
          'prose-blockquote:border-l-4 prose-blockquote:border-blue-300 prose-blockquote:bg-brand-50 prose-blockquote:py-1 prose-blockquote:pl-4 prose-blockquote:not-italic',
          // Code
          'prose-code:bg-surface-2 prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-code:text-sm prose-code:font-mono',
          // HR
          'prose-hr:border-border-strong',
          // Custom class
          className,
          // Containment box: nothing in the content can draw over Paper's UI.
          CONTAINED_CLASS,
        ].filter(Boolean).join(' ')}
        dangerouslySetInnerHTML={{ __html: sanitized }}
        style={{
          /* Embedded video responsive wrappers */
          '--tw-prose-body': '#374151',
          ...CONTENT_CONTAINMENT,
        }}
      />
    </div>
  );
}

export { ReadAloudButton, htmlToPlainText };

export { renderMathInElement };

/**
 * Legacy alias for backward compatibility.
 * New code should use sanitizeHTML instead.
 */
export const sanitizeHtml = sanitizeHTML;
