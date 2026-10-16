import { Navigate, useParams } from 'react-router-dom';
import PropTypes from 'prop-types';
import useIsTeacher from '../hooks/useIsTeacher';

// TeacherOnlyRoute gates a course sub-route to course staff (teachers, TAs,
// designers, admins), the same rule as the in-page
// `if (isTeacher === false) return <Navigate …/>` pattern, applied at the
// route so a page that forgets it is still covered. Students land on the
// course home.
export default function TeacherOnlyRoute({ children }) {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) {
    return (
      <div role="status" aria-label="Loading" className="flex min-h-[50vh] items-center justify-center text-text-tertiary">
        <svg className="h-6 w-6 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
        </svg>
      </div>
    );
  }
  return children;
}
TeacherOnlyRoute.propTypes = { children: PropTypes.node };
