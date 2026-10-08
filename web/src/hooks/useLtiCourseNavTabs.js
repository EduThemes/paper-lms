import { useEffect, useState } from 'react';
import { ltiApi } from '../services/api_lti';

// course_navigation LTI tools per course, shared across CourseNav mounts.
const cache = new Map();

// toCourseNavTabs turns placement tools into CourseNav tab entries.
export function toCourseNavTabs(tools) {
  return (Array.isArray(tools) ? tools : []).map((t) => ({
    path: `/external_tools/${t.id}/launch/course_navigation`,
    label: t.text || t.name || 'External tool',
  }));
}

/**
 * The course's course_navigation LTI tools as CourseNav tabs (G131).
 * Plain fetch + module cache rather than react-query: CourseNav renders on
 * pages and tests that have no QueryClientProvider.
 */
export default function useLtiCourseNavTabs(courseId) {
  const [tabs, setTabs] = useState(() => cache.get(String(courseId))?.tabs || []);

  useEffect(() => {
    if (!courseId) return undefined;
    const key = String(courseId);
    let live = true;
    let entry = cache.get(key);
    if (!entry) {
      entry = { promise: ltiApi.courseTools(courseId, 'course_navigation').then(toCourseNavTabs).catch(() => []) };
      cache.set(key, entry);
    }
    entry.promise.then((t) => {
      entry.tabs = t;
      if (live) setTabs(t);
    });
    return () => {
      live = false;
    };
  }, [courseId]);

  return tabs;
}
