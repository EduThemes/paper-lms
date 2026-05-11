import { useEffect } from 'react';
import { useParams, useLocation } from 'react-router-dom';
import { api } from '../services/api';

const useCourseVisitTracker = (title) => {
  const { courseId } = useParams();
  const location = useLocation();

  useEffect(() => {
    if (!courseId || !title) return;
    // Skip the home page itself
    const isHomePage = location.pathname === `/courses/${courseId}` || location.pathname === `/courses/${courseId}/`;
    if (isHomePage) return;

    api.recordCourseVisit(courseId, {
      url: location.pathname,
      title: title,
    }).catch(() => {});  // fire and forget
  }, [courseId, location.pathname, title]);
};

export default useCourseVisitTracker;
