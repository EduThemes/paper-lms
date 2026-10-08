import { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

const STAFF_TYPES = new Set(['TeacherEnrollment', 'TaEnrollment']);
const ADMIN_ROLES = new Set(['admin', 'super_admin']);

/**
 * Hook to detect if the current user is a teacher/TA/admin in a course.
 * Returns null while loading, true/false once resolved.
 *
 * Reads the caller's own enrollments from
 * GET /courses/:id/enrollments/self (B072). It used to scan the first
 * page of the whole roster (per_page=200, silently capped at 100 by the
 * server), so a teacher whose enrollment was created after 100 students
 * lost every editing control; super_admin was also not recognised.
 *
 * @param {string|number} courseId
 * @returns {boolean|null}
 */
export default function useIsTeacher(courseId) {
  const { user } = useAuth();
  const [isTeacher, setIsTeacher] = useState(null);

  useEffect(() => {
    if (!courseId || !user) return undefined;
    if (ADMIN_ROLES.has(user.role)) {
      setIsTeacher(true);
      return undefined;
    }
    let cancelled = false;
    Promise.resolve()
      .then(() => api.getMyCourseEnrollments(courseId))
      .then((result) => {
        if (cancelled) return;
        const mine = result?.enrollments || [];
        setIsTeacher(
          Boolean(result?.is_admin) ||
          mine.some((e) => (e.enrollment_state ?? 'active') === 'active' &&
            (STAFF_TYPES.has(e.type) || STAFF_TYPES.has(e.role))),
        );
      })
      .catch(() => {
        if (!cancelled) setIsTeacher(false);
      });
    return () => { cancelled = true; };
  }, [courseId, user]);

  return isTeacher;
}
