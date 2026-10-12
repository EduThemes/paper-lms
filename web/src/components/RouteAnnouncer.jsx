import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { titleForPath, fullTitle } from '../utils/routeTitles';

// RouteAnnouncer handles single-page navigation for assistive technology
// (G159, WCAG 2.4.2 / 2.4.3):
//
//  - every route gets a unique document title (a page with a more
//    specific title overrides it with useDocumentTitle);
//  - after a route change, focus moves to the main content region so
//    keyboard and screen-reader users start at the new page, not on the
//    link they activated in the old one;
//  - the new page title is announced in a polite live region.
//
// The first render is left alone (the browser already announces the page
// load and focus should stay where the user put it). Query-string or hash
// changes on the same path (tabs, filters) neither move focus nor announce.
export default function RouteAnnouncer() {
  const location = useLocation();
  const first = useRef(true);
  const regionRef = useRef(null);

  useEffect(() => {
    document.title = fullTitle(titleForPath(location.pathname));
    if (first.current) {
      first.current = false;
      return undefined;
    }
    // Let the page render and set its own title before announcing it.
    const timer = setTimeout(() => {
      const main = document.getElementById('main-content');
      const heading = main?.querySelector('h1');
      const target = heading || main;
      if (target) {
        if (!target.hasAttribute('tabindex')) target.setAttribute('tabindex', '-1');
        target.focus({ preventScroll: false });
      }
      if (regionRef.current) {
        regionRef.current.textContent = '';
        regionRef.current.textContent = document.title;
      }
    }, 150);
    return () => clearTimeout(timer);
  }, [location.pathname]);

  return (
    <div ref={regionRef} className="sr-only" aria-live="polite" aria-atomic="true" data-testid="route-announcer" />
  );
}
