import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { courseRouteInfo } from '../../utils/courseAnalytics';
import { recordPageView, sendPageViewSeconds } from '../../services/api_learner_outcomes';

/**
 * Records course page views for course analytics (G154). Rendered once
 * inside the /courses/:courseId route. Each course page the user opens is
 * reported (POST /page_views); when they leave it the time the tab was
 * visible on it is reported (PUT /page_views/:id). The server keeps only
 * views of users enrolled in the course, so this is safe for everyone.
 */
export default function CoursePageViewTracker() {
  const { pathname } = useLocation();

  useEffect(() => {
    const info = courseRouteInfo(pathname);
    if (!info) return undefined;

    let pageViewId = null;
    let cancelled = false;
    let visibleMs = 0;
    let visibleSince = typeof document !== 'undefined' && document.visibilityState === 'hidden' ? null : Date.now();
    let reported = false;

    const elapsedSeconds = () => (visibleMs + (visibleSince ? Date.now() - visibleSince : 0)) / 1000;
    const report = () => {
      if (pageViewId && !reported) {
        reported = true;
        sendPageViewSeconds(pageViewId, elapsedSeconds());
      }
    };
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        if (visibleSince) visibleMs += Date.now() - visibleSince;
        visibleSince = null;
        // The tab may never come back: report now, and again on return.
        if (pageViewId) sendPageViewSeconds(pageViewId, elapsedSeconds());
      } else if (!visibleSince) {
        visibleSince = Date.now();
      }
    };
    const onPageHide = () => report();

    Promise.resolve()
      .then(() => recordPageView({ courseId: info.courseId, url: pathname, category: info.category }))
      .then((pv) => {
        if (!cancelled) pageViewId = pv?.id || null;
        else if (pv?.id) sendPageViewSeconds(pv.id, elapsedSeconds());
      })
      .catch(() => { /* analytics must never break the page */ });

    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('pagehide', onPageHide);
    return () => {
      cancelled = true;
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('pagehide', onPageHide);
      report();
    };
  }, [pathname]);

  return null;
}
