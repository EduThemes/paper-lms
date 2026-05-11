import { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

/**
 * Hook to detect if the current user is a teacher/TA/admin in a course.
 * Returns null while loading, true/false once resolved.
 *
 * @param {string|number} courseId
 * @returns {boolean|null}
 */
export default function useIsTeacher(courseId) {
  const { user } = useAuth();
  const [isTeacher, setIsTeacher] = useState(null);

  useEffect(() => {
    if (!courseId || !user) return;
    api.getEnrollments(courseId, 1, 200)
      .then(result => {
        const enrollments = result.data || [];
        const myEnrollment = enrollments.find(e =>
          e.user_id === user.id || e.user?.id === user.id
        );
        setIsTeacher(
          user.role === 'admin' ||
          myEnrollment?.type === 'TeacherEnrollment' ||
          myEnrollment?.type === 'TaEnrollment' ||
          myEnrollment?.role === 'TeacherEnrollment' ||
          myEnrollment?.role === 'TaEnrollment'
        );
      })
      .catch(() => setIsTeacher(false));
  }, [courseId, user]);

  return isTeacher;
}
