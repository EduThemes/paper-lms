/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Preview iframes. Both are sandboxed without allow-scripts (nothing in a page can run) but same-origin, so
 * the designer can update the document in place, measure it, and map clicks back to sections. Each frame
 * loads Paper's own stylesheets and shows the page in the same containment box Paper uses (lib/previewDoc).
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { DEVICE_WIDTHS, appStylesHtml, previewShell, sectionIdOf, sectionSelector } from '../lib/previewDoc';
import { PREVIEW_ROOT, scopeCss } from '../lib/previewCss';
import { patchMissingImages } from '../lib/placeholder';
import { useElementWidth, useInView } from './useElementWidth';

const renderCache = new Map();
const CACHE_MAX = 300;

/** Renders once per key (themes × colourways × kinds stays small). */
function cached(key, render) {
  let out = renderCache.get(key);
  if (out === undefined) {
    out = render();
    if (renderCache.size >= CACHE_MAX) renderCache.delete(renderCache.keys().next().value);
    renderCache.set(key, out);
  }
  return out;
}

let stylesMemo = null;
const appStyles = () => (stylesMemo ??= appStylesHtml());

/**
 * A scaled-down, non-interactive render for galleries. Rendered only when the card comes near the
 * viewport, and dropped again when it scrolls far away, so a gallery of 100+ live previews stays light.
 * Missing theme art shows as a tinted block.
 */
export function ThumbFrame({ cacheKey, render, tint, title, dark = false, width = 1000, lang = 'en' }) {
  const [boxRef, boxWidth] = useElementWidth();
  const [viewRef, near] = useInView('600px');
  const scale = boxWidth ? boxWidth / width : 0.26;
  const srcDoc = useMemo(() => {
    if (!near) return '';
    const { html, css } = cached(cacheKey, render);
    return previewShell({ html, css, dark, lang, appStyles: appStyles() });
  }, [near, cacheKey, dark, lang]); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <div
      ref={(el) => {
        boxRef.current = el;
        viewRef.current = el;
      }}
      className="absolute inset-0 overflow-hidden"
      style={{ background: tint.bg }}
      aria-hidden="true"
    >
      {near && (
        <iframe
          title={title}
          tabIndex={-1}
          sandbox="allow-same-origin"
          srcDoc={srcDoc}
          loading="lazy"
          onLoad={(e) => {
            const d = e.currentTarget.contentDocument;
            if (d) patchMissingImages(d, tint);
          }}
          className="pointer-events-none origin-top-left border-0"
          style={{ width, height: `${100 / scale}%`, transform: `scale(${scale})` }}
        />
      )}
    </div>
  );
}

/**
 * The live preview. The body is updated in place on every change (no reload, scroll is kept), the frame
 * grows to its content's height, and a click on a section selects it for editing.
 */
export function LivePreview({ html, css, device, dark, selectedId, onSelect, tint, title, lang = 'en' }) {
  const [wrapRef, avail] = useElementWidth();
  const frameRef = useRef(null);
  const [loaded, setLoaded] = useState(false);
  const [contentH, setContentH] = useState(800);
  const clickedRef = useRef(false);
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;

  const bezel = device === 'phone' ? 20 : 0;
  const frameW = DEVICE_WIDTHS[device] ?? DEVICE_WIDTHS.desktop;
  const scale = avail ? Math.min(1, (avail - bezel) / frameW) : 1;
  // The shell is built once; content, CSS and mode are then set in place.
  const shell = useMemo(() => previewShell({ bodyClass: 'interactive', lang, appStyles: appStyles() }), [lang]);

  const docOf = () => frameRef.current?.contentDocument ?? null;
  const measure = () => {
    const d = docOf();
    if (d?.documentElement) setContentH(Math.max(400, Math.ceil(d.documentElement.scrollHeight)));
  };

  function onLoad() {
    const d = docOf();
    if (!d) return;
    d.addEventListener(
      'click',
      (e) => {
        const target = e.target;
        if (target?.closest?.('a')) e.preventDefault(); // links don't navigate inside the preview
        const id = sectionIdOf(target);
        if (id) {
          clickedRef.current = true;
          onSelectRef.current?.(id);
        }
      },
      true,
    );
    if (typeof ResizeObserver !== 'undefined' && d.body) new ResizeObserver(() => measure()).observe(d.body);
    setLoaded(true);
  }

  useEffect(() => {
    const d = docOf();
    const root = d?.querySelector(PREVIEW_ROOT);
    if (!loaded || !root) return;
    // `html` is the engine's render (every field escaped, rich text through sanitizeRich, from a sanitizeDoc'd
    // doc), written into a frame sandboxed without allow-scripts, never into Paper's own document.
    root.innerHTML = html;
    const style = d.getElementById('pd-page-css');
    if (style) style.textContent = scopeCss(css);
    patchMissingImages(root, tint);
    root.querySelectorAll('img').forEach((img) => img.addEventListener('load', measure, { once: true }));
    measure();
  }, [html, css, loaded, tint]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const d = docOf();
    if (!loaded || !d) return;
    d.documentElement.classList.toggle('dark', !!dark);
    d.body.className = ['interactive', device === 'phone' ? 'phone' : ''].filter(Boolean).join(' ');
    measure();
  }, [dark, device, loaded]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const d = docOf();
    if (!loaded || !d) return;
    const style = d.getElementById('pd-sel');
    if (style) {
      style.textContent = selectedId
        ? `${sectionSelector(selectedId)} { outline: 3px solid #2b7abc !important; outline-offset: 3px; }`
        : '';
    }
    if (!selectedId) return;
    if (clickedRef.current) {
      clickedRef.current = false;
      return;
    }
    const el = d.querySelector(sectionSelector(selectedId));
    const frame = frameRef.current;
    if (!el || !frame || typeof window.scrollTo !== 'function') return;
    const top = frame.getBoundingClientRect().top + el.getBoundingClientRect().top * scale;
    if (top < 120 || top > window.innerHeight - 80) {
      const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches;
      window.scrollTo({ top: Math.max(0, window.scrollY + top - 140), behavior: reduce ? 'auto' : 'smooth' });
    }
  }, [selectedId, loaded, html, scale]);

  return (
    <div ref={wrapRef} className="w-full">
      <div
        className={
          device === 'phone'
            ? 'mx-auto overflow-hidden rounded-[28px] border-[10px] border-zinc-800 bg-zinc-800 shadow-md'
            : 'mx-auto overflow-hidden rounded-card border border-border bg-surface-0 shadow-sm'
        }
        style={{ width: frameW * scale + bezel, height: contentH * scale + bezel }}
      >
        <div className="overflow-hidden" style={{ width: frameW * scale, height: contentH * scale }}>
          <iframe
            ref={frameRef}
            title={title}
            sandbox="allow-same-origin"
            srcDoc={shell}
            onLoad={onLoad}
            className="origin-top-left border-0"
            style={{ width: frameW, height: contentH, transform: `scale(${scale})` }}
          />
        </div>
      </div>
    </div>
  );
}
