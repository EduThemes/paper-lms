import useCourseStaffRole from './useCourseStaffRole';

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
 * TAs count as staff here. For the few teacher-only actions use
 * useCourseStaffRole(courseId)?.teacher.
 *
 * @param {string|number} courseId
 * @returns {boolean|null}
 */
export default function useIsTeacher(courseId) {
  const role = useCourseStaffRole(courseId);
  return role == null ? null : role.staff;
}
