// Reference migration #1 — "list a resource" shape.
//
// Before: ~30 lines of useState/useEffect/try/catch/finally boilerplate
// to load courses, plus a manual `fetchCourses` re-call after create.
//
// After: `useCoursesAll()` gives us loading / error / data + automatic
// dedup + 30s freshness window from the QueryClient default. The new-
// course button uses `useCreateCourse()` which invalidates the courses
// list on success — the new row appears with no manual refetch.
//
// The `<Page>` wrapper owns the loading skeleton, the error card +
// "Try again" button, and the empty-state message. Everything below
// the render-prop boundary executes ONLY when `data` is defined.

import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { Plus, BookOpen, Users } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { useCoursesAll, useCreateCourse } from '../services/apiQueries';

const CoursesPage = () => {
  const { user } = useAuth();
  const query = useCoursesAll(1, 100);
  const createCourse = useCreateCourse();
  const [showCreate, setShowCreate] = useState(false);

  const canCreate = user?.role === 'admin' || user?.role === 'teacher';

  const handleCreate = async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    try {
      await createCourse.mutateAsync({
        name: formData.get('name'),
        course_code: formData.get('course_code'),
      });
      setShowCreate(false);
    } catch {
      // Error surfaces below via createCourse.error — no setError needed.
    }
  };

  // Note: we DON'T use the <Page empty=...> prop here because the
  // original empty-state shows a "Create your first course" CTA that
  // would be lost if Page short-circuited. List pages without an
  // empty-state CTA can use `empty` (see AdminPeoplePage).
  return (
    <Page query={query} title="All Courses">
      {(result) => {
        const courses = result?.data || [];
        return (
          <Layout>
            <div className="flex justify-between items-center mb-6">
              <div>
                <h2 className="text-2xl font-bold text-text-primary">All Courses</h2>
                <p className="text-text-secondary mt-1">
                  {courses.length} course{courses.length !== 1 ? 's' : ''}
                </p>
              </div>
              {canCreate && (
                <button
                  onClick={() => setShowCreate((v) => !v)}
                  className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
                >
                  <Plus className="w-4 h-4" />
                  New Course
                </button>
              )}
            </div>

            {showCreate && (
              <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
                <h3 className="font-semibold mb-4">Create New Course</h3>
                <form onSubmit={handleCreate} className="flex flex-col gap-3 md:flex-row md:items-end md:gap-4">
                  <div className="flex-1">
                    <label htmlFor="course-name" className="block text-sm font-medium text-text-secondary mb-1">Course Name</label>
                    <input id="course-name" name="name" required className="w-full rounded-md border border-border-strong px-3 py-2 text-sm" placeholder="Introduction to Mathematics" />
                  </div>
                  <div className="md:w-40">
                    <label htmlFor="course-code" className="block text-sm font-medium text-text-secondary mb-1">Course Code</label>
                    <input id="course-code" name="course_code" required className="w-full rounded-md border border-border-strong px-3 py-2 text-sm" placeholder="MATH101" />
                  </div>
                  <button type="submit" disabled={createCourse.isPending} className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50">
                    {createCourse.isPending ? 'Creating...' : 'Create'}
                  </button>
                  <button type="button" onClick={() => setShowCreate(false)} className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm">
                    Cancel
                  </button>
                </form>
                {createCourse.isError && (
                  <p className="mt-3 text-sm text-accent-danger" role="alert">
                    {createCourse.error?.message || 'Could not create course.'}
                  </p>
                )}
              </div>
            )}

            {courses.length === 0 ? (
              <div className="bg-surface-0 rounded-lg shadow mx-auto max-w-xl">
                <div className="space-y-4 px-6 py-12 text-center">
                  <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-brand-50 text-brand-600">
                    <BookOpen className="h-6 w-6" aria-hidden="true" />
                  </div>
                  <div>
                    <h2 className="text-lg font-semibold text-text-primary">No courses yet</h2>
                    <p className="mt-1 text-sm text-text-secondary">
                      {canCreate
                        ? 'Get started by creating your first course for students to enroll in.'
                        : 'You are not enrolled in any courses yet. Check back later or contact your teacher.'}
                    </p>
                  </div>
                  {canCreate && (
                    <button
                      onClick={() => setShowCreate(true)}
                      className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
                    >
                      <Plus className="w-4 h-4" />
                      Create your first course
                    </button>
                  )}
                </div>
              </div>
            ) : (
              <div className="bg-surface-0 rounded-lg shadow divide-y">
                {courses.map((course) => (
                <Link
                  key={course.id}
                  to={`/courses/${course.id}`}
                  className="flex items-center justify-between p-4 hover:bg-surface-1"
                >
                  <div className="flex items-center gap-3">
                    <BookOpen className="w-5 h-5 text-brand-500" />
                    <div>
                      <h3 className="font-medium">{course.name}</h3>
                      <div className="flex items-center gap-3 text-text-tertiary text-sm">
                        <span>{course.course_code}</span>
                        {course.total_students != null && (
                          <span className="flex items-center gap-1">
                            <Users className="w-3 h-3" />
                            {course.total_students} student{course.total_students !== 1 ? 's' : ''}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded-full ${
                    course.workflow_state === 'available' ? 'bg-accent-success/20 text-accent-success' :
                    course.workflow_state === 'unpublished' ? 'bg-accent-warning/20 text-accent-warning' :
                    'bg-surface-2 text-text-primary'
                  }`}>
                    {course.workflow_state}
                  </span>
                </Link>
                ))}
              </div>
            )}
          </Layout>
        );
      }}
    </Page>
  );
};

export default CoursesPage;
