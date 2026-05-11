import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Users, UserPlus, Search, X, Mail } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { Skeleton } from '@/components/ui/skeleton';

const ROLE_LABELS = {
  TeacherEnrollment: 'Teacher',
  TaEnrollment: 'TA',
  StudentEnrollment: 'Student',
  ObserverEnrollment: 'Observer',
  DesignerEnrollment: 'Designer',
};

const ROLE_COLORS = {
  TeacherEnrollment: 'bg-brand-100 text-brand-800',
  TaEnrollment: 'bg-purple-100 text-purple-800',
  StudentEnrollment: 'bg-accent-success/20 text-accent-success',
  ObserverEnrollment: 'bg-surface-2 text-text-primary',
  DesignerEnrollment: 'bg-orange-100 text-orange-800',
};

const PeoplePage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [enrollments, setEnrollments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [search, setSearch] = useState('');
  const [showAddForm, setShowAddForm] = useState(false);
  const [addForm, setAddForm] = useState({ user_id: '', type: 'StudentEnrollment' });
  const [adding, setAdding] = useState(false);
  const [filter, setFilter] = useState('all');
  const [userSearch, setUserSearch] = useState('');
  const [userResults, setUserResults] = useState([]);
  const [userSearching, setUserSearching] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const [showUserDropdown, setShowUserDropdown] = useState(false);

  const isTeacher = useIsTeacher(courseId);

  const fetchEnrollments = async () => {
    setError(null);
    setLoading(true);
    try {
      const result = await api.getEnrollments(courseId, 1, 200);
      setEnrollments(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchEnrollments();
  }, [courseId]);

  // Debounced user search
  useEffect(() => {
    if (userSearch.length < 2) {
      setUserResults([]);
      setShowUserDropdown(false);
      return;
    }
    const timer = setTimeout(async () => {
      setUserSearching(true);
      try {
        const result = await api.searchUsers(userSearch);
        const users = result.data || result || [];
        // Filter out users already enrolled
        const enrolledIds = new Set(enrollments.map((e) => e.user_id || e.user?.id));
        setUserResults(users.filter((u) => !enrolledIds.has(u.id)));
        setShowUserDropdown(true);
      } catch {
        setUserResults([]);
      } finally {
        setUserSearching(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [userSearch, enrollments]);

  const selectUser = (u) => {
    setSelectedUser(u);
    setAddForm((prev) => ({ ...prev, user_id: String(u.id) }));
    setUserSearch('');
    setShowUserDropdown(false);
    setUserResults([]);
  };

  const clearSelectedUser = () => {
    setSelectedUser(null);
    setAddForm((prev) => ({ ...prev, user_id: '' }));
    setUserSearch('');
  };

  const handleAddEnrollment = async (e) => {
    e.preventDefault();
    if (!addForm.user_id) return;
    setAdding(true);
    setError(null);
    try {
      await api.createEnrollment(courseId, {
        user_id: parseInt(addForm.user_id, 10),
        type: addForm.type,
        enrollment_state: 'active',
      });
      setShowAddForm(false);
      setAddForm({ user_id: '', type: 'StudentEnrollment' });
      setSelectedUser(null);
      setUserSearch('');
      // Refresh
      const result = await api.getEnrollments(courseId, 1, 200);
      setEnrollments(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setAdding(false);
    }
  };

  // Group enrollments by role
  const filtered = enrollments.filter((e) => {
    if (e.enrollment_state === 'deleted') return false;
    if (filter !== 'all' && e.type !== filter) return false;
    if (search) {
      const term = search.toLowerCase();
      const name = (e.user?.name || e.user?.login_id || '').toLowerCase();
      return name.includes(term);
    }
    return true;
  });

  // Sort: teachers first, then TAs, then students
  const rolePriority = { TeacherEnrollment: 0, TaEnrollment: 1, StudentEnrollment: 2, ObserverEnrollment: 3, DesignerEnrollment: 4 };
  filtered.sort((a, b) => (rolePriority[a.type] ?? 9) - (rolePriority[b.type] ?? 9));

  const roleCounts = {};
  enrollments.forEach((e) => {
    if (e.enrollment_state !== 'deleted') {
      roleCounts[e.type] = (roleCounts[e.type] || 0) + 1;
    }
  });

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }

  if (error && enrollments.length === 0) {
    return (
      <Layout>
        <CourseNav />
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button onClick={fetchEnrollments} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Users className="w-6 h-6" />
            People
          </h2>
          {isTeacher && (
            <button
              onClick={() => setShowAddForm(!showAddForm)}
              className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              {showAddForm ? <X className="w-4 h-4" /> : <UserPlus className="w-4 h-4" />}
              {showAddForm ? 'Cancel' : 'Add People'}
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm">
          {error}
          <button onClick={() => setError(null)} className="ms-2 text-accent-danger hover:text-accent-danger font-bold">&times;</button>
        </div>
      )}

      {/* Add Enrollment Form */}
      {showAddForm && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <form onSubmit={handleAddEnrollment} className="flex items-end gap-3">
            <div className="flex-1 relative">
              <label htmlFor="add-person-search" className="block text-sm font-medium text-text-secondary mb-1">Person</label>
              {selectedUser ? (
                <div className="flex items-center gap-2 border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-1">
                  <div className="w-6 h-6 bg-brand-100 text-brand-700 rounded-full flex items-center justify-center text-xs font-medium flex-shrink-0">
                    {(selectedUser.name || '?')[0].toUpperCase()}
                  </div>
                  <span className="text-text-primary font-medium truncate">{selectedUser.name}</span>
                  {selectedUser.email && <span className="text-text-disabled text-xs truncate">{selectedUser.email}</span>}
                  <button type="button" onClick={clearSelectedUser} className="ml-auto text-text-disabled hover:text-text-secondary flex-shrink-0">
                    <X className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                <>
                  <div className="relative">
                    <Search className="w-4 h-4 absolute start-3 top-1/2 -translate-y-1/2 text-text-disabled" />
                    <input
                      id="add-person-search"
                      type="text"
                      value={userSearch}
                      onChange={(e) => setUserSearch(e.target.value)}
                      onFocus={() => userResults.length > 0 && setShowUserDropdown(true)}
                      onBlur={() => setTimeout(() => setShowUserDropdown(false), 200)}
                      placeholder="Search by name or email..."
                      className="w-full ps-9 pe-3 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    />
                  </div>
                  {showUserDropdown && (
                    <div className="absolute z-20 top-full start-0 end-0 mt-1 bg-surface-0 border border-border-default rounded-md shadow-lg max-h-48 overflow-y-auto">
                      {userSearching ? (
                        <div className="px-3 py-2 text-sm text-text-tertiary">Searching...</div>
                      ) : userResults.length === 0 ? (
                        <div className="px-3 py-2 text-sm text-text-tertiary">No users found</div>
                      ) : (
                        userResults.map((u) => (
                          <button
                            key={u.id}
                            type="button"
                            onMouseDown={(e) => { e.preventDefault(); selectUser(u); }}
                            className="w-full flex items-center gap-2 px-3 py-2 hover:bg-brand-50 text-start"
                          >
                            <div className="w-7 h-7 bg-border-default rounded-full flex items-center justify-center text-xs font-medium text-text-secondary flex-shrink-0">
                              {(u.name || '?')[0].toUpperCase()}
                            </div>
                            <div className="min-w-0">
                              <div className="text-sm font-medium text-text-primary truncate">{u.name || `User ${u.id}`}</div>
                              {u.email && <div className="text-xs text-text-tertiary truncate">{u.email}</div>}
                            </div>
                          </button>
                        ))
                      )}
                    </div>
                  )}
                </>
              )}
            </div>
            <div>
              <label htmlFor="add-person-role" className="block text-sm font-medium text-text-secondary mb-1">Role</label>
              <select
                id="add-person-role"
                value={addForm.type}
                onChange={(e) => setAddForm({ ...addForm, type: e.target.value })}
                className="border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="StudentEnrollment">Student</option>
                <option value="TeacherEnrollment">Teacher</option>
                <option value="TaEnrollment">TA</option>
                <option value="ObserverEnrollment">Observer</option>
              </select>
            </div>
            <button
              type="submit"
              disabled={adding || !addForm.user_id}
              className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              {adding ? 'Adding...' : 'Add'}
            </button>
          </form>
        </div>
      )}

      {/* Search and Filter */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1 max-w-xs">
          <Search className="w-4 h-4 absolute start-3 top-1/2 -translate-y-1/2 text-text-disabled" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search people..."
            aria-label="Search people"
            className="w-full ps-9 pe-3 py-2 border border-border-strong rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setFilter('all')}
            className={`px-3 py-1.5 text-xs font-medium rounded-full ${filter === 'all' ? 'bg-brand-100 text-brand-800' : 'bg-surface-2 text-text-secondary hover:bg-border-default'}`}
          >
            All ({enrollments.filter(e => e.enrollment_state !== 'deleted').length})
          </button>
          {Object.entries(roleCounts).map(([role, count]) => (
            <button
              key={role}
              onClick={() => setFilter(role)}
              className={`px-3 py-1.5 text-xs font-medium rounded-full ${filter === role ? ROLE_COLORS[role] || 'bg-brand-100 text-brand-800' : 'bg-surface-2 text-text-secondary hover:bg-border-default'}`}
            >
              {ROLE_LABELS[role] || role} ({count})
            </button>
          ))}
        </div>
      </div>

      {/* People List */}
      <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
        {filtered.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">
            {search ? 'No people match your search.' : 'No people enrolled.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-surface-1 border-b">
                <tr>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Name</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Login ID</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Role</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Section</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {filtered.map((enrollment) => (
                  <tr key={enrollment.id} className="hover:bg-surface-1">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 bg-border-default rounded-full flex items-center justify-center text-text-tertiary text-sm font-medium flex-shrink-0">
                          {(enrollment.user?.name || '?')[0].toUpperCase()}
                        </div>
                        <div>
                          <div className="text-sm font-medium text-text-primary">
                            {enrollment.user?.name || `User ${enrollment.user_id}`}
                          </div>
                          {enrollment.user?.email && (
                            <div className="text-xs text-text-tertiary">{enrollment.user.email}</div>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-sm text-text-secondary">
                      {enrollment.user?.login_id || '-'}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${ROLE_COLORS[enrollment.type] || 'bg-surface-2 text-text-primary'}`}>
                        {ROLE_LABELS[enrollment.type] || enrollment.type}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-sm text-text-secondary">
                      {enrollment.course_section_id || '-'}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-0.5 rounded-full ${
                        enrollment.enrollment_state === 'active' ? 'bg-accent-success/20 text-accent-success' :
                        enrollment.enrollment_state === 'invited' ? 'bg-accent-warning/20 text-accent-warning' :
                        'bg-surface-2 text-text-secondary'
                      }`}>
                        {enrollment.enrollment_state || 'active'}
                      </span>
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

export default PeoplePage;
