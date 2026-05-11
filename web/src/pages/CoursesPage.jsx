import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, BookOpen, Users } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const CoursesPage = () => {
  const { user } = useAuth();
  const [courses, setCourses] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);

  const fetchCourses = async () => {
    try {
      const { data } = await api.getAllCourses(1, 100);
      setCourses(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchCourses(); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreating(true);
    const formData = new FormData(e.target);
    try {
      await api.createCourse({
        name: formData.get('name'),
        course_code: formData.get('course_code'),
      });
      setShowCreate(false);
      fetchCourses();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  return (
    <Layout>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-text-primary">All Courses</h2>
          <p className="text-text-secondary mt-1">{courses.length} course{courses.length !== 1 ? 's' : ''}</p>
        </div>
        {(user?.role === 'admin' || user?.role === 'teacher') && (
          <button
            onClick={() => setShowCreate(!showCreate)}
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
            <button type="submit" disabled={creating} className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50">
              {creating ? 'Creating...' : 'Create'}
            </button>
            <button type="button" onClick={() => setShowCreate(false)} className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm">
              Cancel
            </button>
          </form>
        </div>
      )}

      {loading ? (
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      ) : error ? (
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button onClick={() => { setError(null); setLoading(true); fetchCourses(); }} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
        </div>
      ) : courses.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow mx-auto max-w-xl">
          <div className="space-y-4 px-6 py-12 text-center">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-brand-50 text-brand-600">
              <BookOpen className="h-6 w-6" aria-hidden="true" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-text-primary">No courses yet</h2>
              <p className="mt-1 text-sm text-text-secondary">
                {(user?.role === 'admin' || user?.role === 'teacher')
                  ? 'Get started by creating your first course for students to enroll in.'
                  : 'You are not enrolled in any courses yet. Check back later or contact your teacher.'}
              </p>
            </div>
            {(user?.role === 'admin' || user?.role === 'teacher') && (
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
};

export default CoursesPage;
