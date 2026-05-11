import React, { useMemo, useEffect, useRef, useState, useCallback, useContext } from 'react';
import DOMPurify from 'dompurify';
import katex from 'katex';
import { Volume2, Square } from 'lucide-react';
import { ReadingPrefsContext } from '../contexts/ReadingPrefsContext';

/* --- DOMPurify Configuration ---------------------------------------------- */

// Course content imported from Canvas / IMSCC commonly uses tags that a tight
// allowlist would silently drop (collapsible <details>/<summary>, semantic
// landmarks, embedded media, definition lists, Canvas equation images with
// data-equation-content, etc.). Our philosophy is to keep all the rich HTML
// authors put in — only block what's actively dangerous.
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
    // Lists
    'ul', 'ol', 'li', 'dl', 'dt', 'dd',
    // Tables
    'table', 'caption', 'colgroup', 'col', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td',
    // Media / embeds
    'img', 'picture', 'source', 'video', 'audio', 'track', 'iframe', 'embed', 'object', 'param',
    // Forms (instructor-authored quizzes, callouts etc. occasionally use these)
    'form', 'fieldset', 'legend', 'label', 'input', 'button', 'select', 'option',
    'optgroup', 'textarea', 'output', 'progress', 'meter', 'datalist',
    // Inline styling — Canvas exports include scoped <style> blocks for
    // collapsible/tabbed designs. Trust level for course content imported by
    // an instructor is higher than for general comment input.
    'style',
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
    // Tables
    'colspan', 'rowspan', 'scope', 'headers', 'span',
    // Anchors / links
    'download', 'hreflang', 'type', 'referrerpolicy',
    // Images / picture / video / audio / iframe
    'srcset', 'sizes', 'loading', 'decoding', 'crossorigin', 'usemap',
    'controls', 'autoplay', 'loop', 'muted', 'playsinline', 'preload', 'poster',
    'allowfullscreen', 'allow', 'frameborder', 'sandbox', 'longdesc',
    // Form elements (kept permissive but on* handlers are still blocked)
    'value', 'placeholder', 'min', 'max', 'step', 'pattern', 'required',
    'readonly', 'disabled', 'checked', 'selected', 'multiple', 'accept',
    'autocomplete', 'autofocus', 'form', 'formaction', 'formmethod',
    'formnovalidate', 'formtarget', 'list', 'maxlength', 'minlength', 'size',
    'for', 'open',
    // Media tracks
    'kind', 'srclang', 'default',
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
  FORBID_TAGS: ['script', 'base', 'meta', 'link'],
  FORBID_ATTR: ['xmlns:xlink'],
  // Allow known-safe URI schemes + relative URLs.
  ALLOWED_URI_REGEXP: /^(?:(?:https?|mailto|tel|ftp|sftp|data|blob):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i,
};

/**
 * Sanitize an HTML string using DOMPurify.
 * Exported for use in other components that need to sanitize HTML
 * before passing it to dangerouslySetInnerHTML.
 */
export function sanitizeHTML(dirty) {
  if (!dirty) return '';
  return DOMPurify.sanitize(dirty, purifyConfig);
}

/* --- Rich Content Viewer ------------------------------------------------ */

/**
 * Render all math-tex spans inside a container element using KaTeX.
 * Looks for elements with class "math-tex" and renders their LaTeX content.
 * Also detects inline LaTeX delimiters: \( ... \) and $$ ... $$
 */
function renderMathInElement(container) {
  if (!container) return;

  // Render explicit math-tex spans (from our RCE)
  const mathSpans = container.querySelectorAll('.math-tex');
  mathSpans.forEach((span) => {
    const latex = span.getAttribute('data-latex') || span.textContent;
    if (!latex) return;
    try {
      katex.render(latex, span, { throwOnError: false, displayMode: false });
    } catch {
      // Leave as-is if rendering fails
    }
  });

  // Also process LaTeX delimiters \( ... \), \[ ... \], and $$ ... $$ in text nodes
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, null);
  const textNodes = [];
  let node;
  while ((node = walker.nextNode())) {
    if (/\\\(.*?\\\)|\\\[.*?\\\]|\$\$.*?\$\$/s.test(node.textContent)) {
      textNodes.push(node);
    }
  }

  textNodes.forEach((textNode) => {
    const text = textNode.textContent;
    const parts = text.split(/(\\\(.*?\\\)|\\\[.*?\\\]|\$\$.*?\$\$)/s);
    if (parts.length <= 1) return;

    const fragment = document.createDocumentFragment();
    parts.forEach((part) => {
      let match;
      if ((match = part.match(/^\\\((.*?)\\\)$/s))) {
        const span = document.createElement('span');
        try {
          katex.render(match[1], span, { throwOnError: false, displayMode: false });
        } catch {
          span.textContent = part;
        }
        fragment.appendChild(span);
      } else if ((match = part.match(/^\\\[(.*?)\\\]$/s))) {
        const div = document.createElement('div');
        div.style.textAlign = 'center';
        div.style.margin = '0.5em 0';
        try {
          katex.render(match[1], div, { throwOnError: false, displayMode: true });
        } catch {
          div.textContent = part;
        }
        fragment.appendChild(div);
      } else if ((match = part.match(/^\$\$(.*?)\$\$$/s))) {
        const div = document.createElement('div');
        try {
          katex.render(match[1], div, { throwOnError: false, displayMode: true });
        } catch {
          div.textContent = part;
        }
        fragment.appendChild(div);
      } else if (part) {
        fragment.appendChild(document.createTextNode(part));
      }
    });
    textNode.parentNode.replaceChild(fragment, textNode);
  });
}

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

/** Strip HTML to plain text for TTS. */
function htmlToPlainText(html) {
  if (!html) return '';
  if (typeof document === 'undefined') return html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  const tmp = document.createElement('div');
  tmp.innerHTML = html;
  return (tmp.textContent || tmp.innerText || '').replace(/\s+/g, ' ').trim();
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
      renderMathInElement(containerRef.current);
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
        ].filter(Boolean).join(' ')}
        dangerouslySetInnerHTML={{ __html: sanitized }}
        style={{
          /* Embedded video responsive wrappers */
          '--tw-prose-body': '#374151',
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
