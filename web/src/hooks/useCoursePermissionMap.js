import { useEffect, useState } from 'react';
import { adminAccountsApi } from '../services/api_admin_accounts';

// Per-course cache so the navigation doesn't refetch on every page.
const cache = new Map();

/**
 * The caller's custom-role permission map in a course (G102), from
 * GET /courses/:id/permissions. Returns null until loaded, when disabled,
 * or when the enrollment has no custom role (base roles keep their
 * type-based UI). Otherwise { custom_role: true, permission_map }.
 *
 * Effect-based (not react-query) because CourseNav renders in many page
 * tests without a query client.
 */
export default function useCoursePermissionMap(courseId, enabled) {
  const key = courseId ? String(courseId) : '';
  const [value, setValue] = useState(() => (key && cache.has(key) ? cache.get(key) : null));

  useEffect(() => {
    if (!enabled || !key) {
      setValue(null);
      return undefined;
    }
    if (cache.has(key)) {
      setValue(cache.get(key));
      return undefined;
    }
    let cancelled = false;
    Promise.resolve()
      .then(() => adminAccountsApi.getCoursePermissions(key))
      .then((res) => {
        const next = res?.custom_role ? { custom_role: true, permission_map: res.permission_map || {} } : null;
        cache.set(key, next);
        if (!cancelled) setValue(next);
      })
      .catch(() => {
        if (!cancelled) setValue(null);
      });
    return () => { cancelled = true; };
  }, [key, enabled]);

  return value;
}

// Test hook: forget cached permissions.
export function resetCoursePermissionCache() {
  cache.clear();
}
