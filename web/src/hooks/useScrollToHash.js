import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

/**
 * Scroll to (and focus) the element named by the URL hash once the page's
 * content has rendered. Course content links inserted by the editor point
 * at anchors such as /courses/1/modules#module-5 (G011); the browser's own
 * hash scroll happens before a data-driven page has rendered its list.
 *
 * @param {boolean} ready - true once the anchored elements are in the DOM
 */
export default function useScrollToHash(ready) {
  const { hash } = useLocation();
  useEffect(() => {
    if (!ready || !hash || hash.length < 2) return;
    let id;
    try { id = decodeURIComponent(hash.slice(1)); } catch { return; }
    const el = document.getElementById(id);
    if (!el) return;
    el.scrollIntoView?.({ block: 'start' });
    if (!el.hasAttribute('tabindex')) el.setAttribute('tabindex', '-1');
    el.focus?.({ preventScroll: true });
  }, [ready, hash]);
}
