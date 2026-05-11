import React, { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Plus, Search, BookOpen } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const AdminCoursesPage = () => {
  const navigate = useNavigate();
  const [courses, setCourses] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);

  const fetchCourses = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.getAllCourses(1, 100);
      setCourses(result.data || []);
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
      const created = await api.createCourse({
        name: formData.get('name'),
        course_code: formData.get('course_code'),
      });
      setShowCreate(false);
      if (created?.id) {
        navigate(`/courses/${created.id}`);
      } else {
        fetchCourses();
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const filtered = courses.filter((c) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      (c.name || '').toLowerCase().includes(q) ||
      (c.course_code || '').toLowerCase().includes(q)
    );
  });

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-text-primary">Courses</h1>
            <p className="text-sm text-text-secondary mt-1">
              {loading ? '—' : `${filtered.length} of ${courses.length} course${courses.length === 1 ? '' : 's'}`}
            </p>
          </div>
          <button
            onClick={() => setShowCreate((v) => !v)}
            className="inline-flex items-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
          >
            <Plus className="w-4 h-4" />
            New Course
          </button>
        </div>

        {showCreate && (
          <div className="mb-6 rounded-lg border border-border-default bg-surface-0 p-5">
            <h2 className="text-sm font-semibold text-text-primary mb-3">Create new course</h2>
            <form onSubmit={handleCreate} className="flex flex-col gap-3 md:flex-row md:items-end md:gap-4">
              <div className="flex-1">
                <label htmlFor="course-name" className="block text-xs font-medium text-text-secondary mb-1">Course name</label>
                <input id="course-name" name="name" required className="w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm" placeholder="Introduction to Mathematics" />
              </div>
              <div className="md:w-44">
                <label htmlFor="course-code" className="block text-xs font-medium text-text-secondary mb-1">Course code</label>
                <input id="course-code" name="course_code" required className="w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm" placeholder="MATH101" />
              </div>
              <button type="submit" disabled={creating} className="rounded-md bg-accent-success px-4 py-2 text-sm font-medium text-white hover:bg-accent-success/90 disabled:opacity-50">
                {creating ? 'Creating…' : 'Create'}
              </button>
              <button type="button" onClick={() => setShowCreate(false)} className="px-3 py-2 text-sm text-text-tertiary hover:text-text-secondary">
                Cancel
              </button>
            </form>
          </div>
        )}

        <div className="mb-4 relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search courses by name or code"
            className="w-full max-w-md rounded-md border border-border-strong bg-surface-0 pl-9 pr-3 py-2 text-sm"
          />
        </div>

        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        ) : error ? (
          <div className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-4 text-center">
            <p className="text-sm text-accent-danger mb-2">{error}</p>
            <button onClick={fetchCourses} className="text-sm font-medium text-brand-600 hover:text-brand-800">Try Again</button>
          </div>
        ) : filtered.length === 0 ? (
          <div className="rounded-lg border border-border-default bg-surface-0 p-10 text-center">
            <BookOpen className="mx-auto w-8 h-8 text-text-tertiary mb-2" />
            <p className="text-sm text-text-secondary">
              {search ? 'No courses match your search.' : 'No courses yet. Click “New Course” to create the first one.'}
            </p>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border-default bg-surface-0">
            <table className="w-full text-sm">
              <thead className="bg-surface-1 text-text-secondary">
                <tr>
                  <th className="px-4 py-2 text-left font-medium">Name</th>
                  <th className="px-4 py-2 text-left font-medium">Code</th>
                  <th className="px-4 py-2 text-left font-medium">State</th>
                  <th className="px-4 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((c) => (
                  <tr key={c.id} className="border-t border-border-default hover:bg-surface-1">
                    <td className="px-4 py-2 font-medium text-text-primary">
                      <Link to={`/courses/${c.id}`} className="hover:text-brand-700">{c.name || '(unnamed)'}</Link>
                    </td>
                    <td className="px-4 py-2 text-text-secondary">{c.course_code || '—'}</td>
                    <td className="px-4 py-2 text-text-secondary">{c.workflow_state || '—'}</td>
                    <td className="px-4 py-2 text-right">
                      <Link to={`/courses/${c.id}/settings`} className="text-xs text-brand-600 hover:text-brand-800">Settings</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default AdminCoursesPage;
