import { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

const STAFF_TYPES = new Set(['TeacherEnrollment', 'TaEnrollment']);
const ADMIN_ROLES = new Set(['admin', 'super_admin']);

const isActive = (e) => (e.enrollment_state ?? 'active') === 'active';
const hasType = (e, type) => e.type === type || e.role === type;

/**
 * The caller's staff standing in a course, from
 * GET /courses/:id/enrollments/self (B072).
 *
 * Returns null while loading, then { staff, teacher }:
 *   staff   — teacher, TA or admin: may use the instructor tools;
 *   teacher — teacher or admin only. A few routes exclude TAs (course
 *             audit log, course delete — RequireCourseRole("TeacherEnrollment")),
 *             so UI for them must check this, not staff.
 *
 * @param {string|number} courseId
 * @returns {{staff: boolean, teacher: boolean}|null}
 */
export default function useCourseStaffRole(courseId) {
  const { user } = useAuth();
  const [role, setRole] = useState(null);

  useEffect(() => {
    if (!courseId || !user) return undefined;
    if (ADMIN_ROLES.has(user.role)) {
      setRole({ staff: true, teacher: true });
      return undefined;
    }
    let cancelled = false;
    Promise.resolve()
      .then(() => api.getMyCourseEnrollments(courseId))
      .then((result) => {
        if (cancelled) return;
        const mine = (result?.enrollments || []).filter(isActive);
        const admin = Boolean(result?.is_admin);
        const teacher = admin || mine.some((e) => hasType(e, 'TeacherEnrollment'));
        const staff = teacher || mine.some((e) => STAFF_TYPES.has(e.type) || STAFF_TYPES.has(e.role));
        setRole({ staff, teacher });
      })
      .catch(() => {
        if (!cancelled) setRole({ staff: false, teacher: false });
      });
    return () => { cancelled = true; };
  }, [courseId, user]);

  return role;
}
