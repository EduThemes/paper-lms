import React from 'react';
import { Link, useLocation, useParams } from 'react-router-dom';
import { EyeOff } from 'lucide-react';
import { useCourseUI } from '../contexts/CourseUIContext';
import { courseTabForPath, isCourseTabHidden } from '../utils/courseTabs';
import Layout from './Layout';
import CourseNav from './CourseNav';

// CourseTabGate wraps the course routes (G078). When a learner opens the
// index of a tab the course hides (typing /courses/7/files, say), it shows
// Canvas's "disabled" notice instead of the page. Staff and admins see
// hidden tabs as usual.
export default function CourseTabGate({ children }) {
  const { courseId } = useParams();
  const { pathname } = useLocation();
  const { course, loading, isStaff } = useCourseUI();

  const tabId = courseTabForPath(pathname, courseId);
  if (!tabId) return children;
  // Hold a gated index until the course (and its tab settings) is known,
  // so a hidden page never flashes up first.
  if (loading) return null;
  if (isStaff || !course || !isCourseTabHidden(course.navigation_tabs, tabId)) return children;

  return (
    <Layout>
      <CourseNav />
      <div role="status" className="max-w-xl mx-auto text-center py-16">
        <EyeOff className="w-10 h-10 mx-auto text-text-tertiary mb-4" aria-hidden="true" />
        <h1 className="text-lg font-semibold text-text-primary mb-2">That page has been disabled for this course</h1>
        <p className="text-sm text-text-secondary mb-6">Your teacher has hidden it from the course navigation.</p>
        <Link
          to={`/courses/${courseId}`}
          className="inline-flex px-4 py-2 rounded-md text-sm font-medium bg-brand-600 text-on-brand hover:bg-brand-700"
        >
          Go to the course home
        </Link>
      </div>
    </Layout>
  );
}
