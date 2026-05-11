import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { ArrowLeft, Plus, Edit3, Trash2, Search, X, Shield, Clock, CalendarDays, FileText, CheckCircle, AlertTriangle } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

/* ─── Constants ─── */

const ACCOMMODATION_TYPES = [
  { value: 'extended_time', label: 'Extended Time', description: 'Extra time on timed assessments' },
  { value: 'modified_due_dates', label: 'Modified Due Dates', description: 'Additional days for assignment due dates' },
  { value: 'alternative_format', label: 'Alternative Format', description: 'Alternate submission formats accepted' },
  { value: 'reduced_assignments', label: 'Reduced Assignments', description: 'Modified assignment requirements' },
  { value: 'assistive_tech', label: 'Assistive Technology', description: 'Screen reader, text-to-speech, etc.' },
  { value: 'other', label: 'Other', description: 'Custom accommodation' },
];

const PLAN_TYPES = [
  { value: 'iep', label: 'IEP', color: 'bg-purple-100 text-purple-700 border-purple-200' },
  { value: '504', label: '504', color: 'bg-brand-100 text-brand-700 border-blue-200' },
  { value: 'ell', label: 'ELL', color: 'bg-accent-success/20 text-accent-success border-accent-success/30' },
  { value: 'gifted', label: 'Gifted', color: 'bg-accent-warning/20 text-accent-warning border-accent-warning/30' },
  { value: 'informal', label: 'Informal', color: 'bg-surface-2 text-text-secondary border-border-default' },
];

const PLAN_MAP = Object.fromEntries(PLAN_TYPES.map((p) => [p.value, p]));
const TYPE_MAP = Object.fromEntries(ACCOMMODATION_TYPES.map((t) => [t.value, t]));

const EMPTY_FORM = {
  accommodation_type: 'extended_time',
  plan_type: 'iep',
  time_multiplier: '1.5',
  extra_days: '1',
  effective_start: '',
  effective_end: '',
  notes: '',
};

/* ─── API Helpers ─── */

async function apiRequest(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.errors?.[0]?.message || `Request failed: ${response.status}`);
  }
  return response.json();
}

/* ─── Plan Badge ─── */

const PlanBadge = ({ planType }) => {
  const plan = PLAN_MAP[planType];
  if (!plan) return null;
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold border ${plan.color}`}>
      {plan.label}
    </span>
  );
};

/* ─── Accommodation Card ─── */

const AccommodationCard = ({ accommodation, onEdit, onDeactivate }) => {
  const type = TYPE_MAP[accommodation.accommodation_type] || { label: accommodation.accommodation_type, description: '' };
  const isActive = accommodation.active !== false && accommodation.workflow_state !== 'deleted';

  return (
    <div className={`bg-surface-0 border rounded-lg p-4 ${isActive ? 'border-border-default' : 'border-border-subtle opacity-60'}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap mb-1">
            <h3 className="font-semibold text-text-primary">{type.label}</h3>
            <PlanBadge planType={accommodation.plan_type} />
            {isActive && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-accent-success/10 text-accent-success border border-accent-success/30">
                <CheckCircle className="w-3 h-3" aria-hidden="true" />
                Active
              </span>
            )}
          </div>
          <p className="text-sm text-text-tertiary mb-2">{type.description}</p>

          {/* Accommodation Details */}
          <div className="flex flex-wrap gap-3 text-xs text-text-secondary">
            {accommodation.accommodation_type === 'extended_time' && accommodation.time_multiplier && (
              <span className="flex items-center gap-1">
                <Clock className="w-3.5 h-3.5" aria-hidden="true" />
                {accommodation.time_multiplier}x time
              </span>
            )}
            {accommodation.accommodation_type === 'modified_due_dates' && accommodation.extra_days && (
              <span className="flex items-center gap-1">
                <CalendarDays className="w-3.5 h-3.5" aria-hidden="true" />
                +{accommodation.extra_days} day{accommodation.extra_days !== 1 ? 's' : ''}
              </span>
            )}
            {accommodation.effective_start && (
              <span className="flex items-center gap-1">
                <CalendarDays className="w-3.5 h-3.5" aria-hidden="true" />
                From {new Date(accommodation.effective_start).toLocaleDateString()}
              </span>
            )}
            {accommodation.effective_end && (
              <span>to {new Date(accommodation.effective_end).toLocaleDateString()}</span>
            )}
          </div>

          {/* Auto-apply badge */}
          {isActive && (accommodation.accommodation_type === 'extended_time' || accommodation.accommodation_type === 'modified_due_dates') && (
            <div className="mt-2">
              <span className="inline-flex items-center gap-1 text-xs text-accent-success bg-accent-success/10 px-2 py-1 rounded">
                <CheckCircle className="w-3 h-3" aria-hidden="true" />
                Auto-applies to all quizzes and assignments
              </span>
            </div>
          )}

          {/* Notes */}
          {accommodation.notes && (
            <p className="mt-2 text-xs text-text-tertiary italic">{accommodation.notes}</p>
          )}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-1 shrink-0">
          <button
            type="button"
            onClick={() => onEdit(accommodation)}
            className="p-1.5 text-text-disabled hover:text-brand-600 rounded-lg hover:bg-brand-50 focus:outline-none focus:ring-2 focus:ring-brand-500"
            aria-label={`Edit ${type.label} accommodation`}
          >
            <Edit3 className="w-4 h-4" aria-hidden="true" />
          </button>
          {isActive && (
            <button
              type="button"
              onClick={() => onDeactivate(accommodation.id)}
              className="p-1.5 text-text-disabled hover:text-accent-danger rounded-lg hover:bg-accent-danger/10 focus:outline-none focus:ring-2 focus:ring-red-500"
              aria-label={`Deactivate ${type.label} accommodation`}
            >
              <Trash2 className="w-4 h-4" aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
};

/* ─── Application Log ─── */

const ApplicationLog = ({ userId }) => {
  const [logs, setLogs] = useState([]);
  const [logLoading, setLogLoading] = useState(true);

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const data = await apiRequest(`/users/${userId}/accommodations/log`);
        setLogs(data.data || data || []);
      } catch {
        // silently fail
      } finally {
        setLogLoading(false);
      }
    };
    if (userId) fetchLogs();
  }, [userId]);

  if (logLoading) {
    return (
      <div className="flex justify-center py-6">
        <div className="h-5 w-5 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" role="status" aria-label="Loading application log" />
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <p className="text-sm text-text-tertiary text-center py-4">No accommodation applications logged yet.</p>
    );
  }

  return (
    <div className="divide-y divide-gray-100">
      {logs.map((log, i) => (
        <div key={i} className="py-3 flex items-start gap-3">
          <div className="w-1.5 h-1.5 rounded-full bg-blue-400 mt-2 shrink-0" aria-hidden="true" />
          <div className="flex-1 min-w-0">
            <p className="text-sm text-text-primary">
              <span className="font-medium">{TYPE_MAP[log.accommodation_type]?.label || log.accommodation_type}</span>
              {' applied to '}
              <span className="font-medium">{log.assignment_name || log.quiz_name || 'item'}</span>
            </p>
            <div className="flex flex-wrap gap-3 mt-1 text-xs text-text-tertiary">
              {log.original_value !== undefined && log.adjusted_value !== undefined && (
                <span>
                  {log.original_value} &rarr; {log.adjusted_value}
                </span>
              )}
              {log.applied_at && (
                <span>{new Date(log.applied_at).toLocaleDateString()}</span>
              )}
              {log.course_name && (
                <span>{log.course_name}</span>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
};

/* ─── Main Page ─── */

const AccommodationsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [students, setStudents] = useState([]);
  const [selectedStudent, setSelectedStudent] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState([]);
  const [accommodations, setAccommodations] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ ...EMPTY_FORM });
  const [submitting, setSubmitting] = useState(false);
  const [showLog, setShowLog] = useState(false);
  const [courseFilter, setCourseFilter] = useState(courseId || 'all');
  const [courses, setCourses] = useState([]);
  const [deleteConfirm, setDeleteConfirm] = useState(null);

  // Load courses for filter
  useEffect(() => {
    const fetchCourses = async () => {
      try {
        const result = await api.getCourses(1, 100);
        setCourses(result.data || []);
      } catch {
        // silently fail
      }
    };
    fetchCourses();
  }, []);

  // Load students for search/select
  useEffect(() => {
    const fetchStudents = async () => {
      try {
        if (courseFilter && courseFilter !== 'all') {
          const result = await api.getEnrollments(courseFilter, 1, 200);
          const enrollments = result.data || [];
          const studentEnrollments = enrollments.filter(
            (e) => e.type === 'StudentEnrollment' || e.role === 'StudentEnrollment' || e.enrollment_type === 'student'
          );
          const seen = new Set();
          const list = [];
          for (const e of studentEnrollments) {
            const uid = e.user_id || e.user?.id;
            if (uid && !seen.has(uid)) {
              seen.add(uid);
              list.push({
                id: uid,
                name: e.user?.name || e.user?.display_name || `User ${uid}`,
                sortable_name: e.user?.sortable_name || e.user?.name || `User ${uid}`,
              });
            }
          }
          list.sort((a, b) => a.sortable_name.localeCompare(b.sortable_name));
          setStudents(list);
        }
      } catch {
        // silently fail
      }
    };
    fetchStudents();
  }, [courseFilter]);

  // Search filter
  useEffect(() => {
    if (!searchQuery.trim()) {
      setSearchResults([]);
      return;
    }
    const q = searchQuery.toLowerCase();
    const filtered = students.filter((s) => s.name.toLowerCase().includes(q) || s.sortable_name.toLowerCase().includes(q));
    setSearchResults(filtered.slice(0, 10));
  }, [searchQuery, students]);

  // Fetch accommodations for selected student
  const fetchAccommodations = useCallback(async () => {
    if (!selectedStudent) return;
    setLoading(true);
    setError(null);
    try {
      const data = await apiRequest(`/users/${selectedStudent.id}/accommodations`);
      setAccommodations(data.data || data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [selectedStudent]);

  useEffect(() => {
    fetchAccommodations();
  }, [fetchAccommodations]);

  const selectStudent = (student) => {
    setSelectedStudent(student);
    setSearchQuery('');
    setSearchResults([]);
    setShowForm(false);
    setEditingId(null);
    setShowLog(false);
  };

  const clearStudent = () => {
    setSelectedStudent(null);
    setAccommodations([]);
    setShowForm(false);
    setEditingId(null);
    setShowLog(false);
  };

  const resetForm = () => {
    setFormData({ ...EMPTY_FORM });
    setEditingId(null);
    setShowForm(false);
  };

  const handleEdit = (accommodation) => {
    setFormData({
      accommodation_type: accommodation.accommodation_type || 'extended_time',
      plan_type: accommodation.plan_type || 'iep',
      time_multiplier: String(accommodation.time_multiplier || '1.5'),
      extra_days: String(accommodation.extra_days || '1'),
      effective_start: accommodation.effective_start ? accommodation.effective_start.split('T')[0] : '',
      effective_end: accommodation.effective_end ? accommodation.effective_end.split('T')[0] : '',
      notes: accommodation.notes || '',
    });
    setEditingId(accommodation.id);
    setShowForm(true);
  };

  const handleDeactivate = async (id) => {
    setDeleteConfirm(id);
  };

  const confirmDeactivate = async () => {
    if (!deleteConfirm) return;
    try {
      await apiRequest(`/accommodations/${deleteConfirm}`, { method: 'DELETE' });
      setSuccess('Accommodation deactivated.');
      setDeleteConfirm(null);
      fetchAccommodations();
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err.message);
      setDeleteConfirm(null);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const payload = {
      accommodation_type: formData.accommodation_type,
      plan_type: formData.plan_type,
      notes: formData.notes,
      effective_start: formData.effective_start || null,
      effective_end: formData.effective_end || null,
    };

    if (formData.accommodation_type === 'extended_time') {
      payload.time_multiplier = parseFloat(formData.time_multiplier) || 1.5;
    }
    if (formData.accommodation_type === 'modified_due_dates') {
      payload.extra_days = parseInt(formData.extra_days, 10) || 1;
    }

    try {
      if (editingId) {
        await apiRequest(`/accommodations/${editingId}`, {
          method: 'PUT',
          body: JSON.stringify({ accommodation: payload }),
        });
        setSuccess('Accommodation updated.');
      } else {
        await apiRequest(`/users/${selectedStudent.id}/accommodations`, {
          method: 'POST',
          body: JSON.stringify({ accommodation: payload }),
        });
        setSuccess('Accommodation created.');
      }
      resetForm();
      fetchAccommodations();
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  if (courseId && isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (courseId && isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  return (
    <Layout>
      <CourseNav />
      <main id="main-content" className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Header */}
        <div className="mb-6">
          {courseId && (
            <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center gap-1 mb-2">
              <ArrowLeft className="w-4 h-4" aria-hidden="true" />
              Back to Course
            </Link>
          )}
          <div className="flex items-center gap-3">
            <Shield className="w-7 h-7 text-brand-600" aria-hidden="true" />
            <h1 className="text-2xl font-bold text-text-primary">Student Accommodations</h1>
          </div>
          <p className="text-sm text-text-tertiary mt-1">Manage IEP, 504, ELL, and other student accommodations</p>
        </div>

        {/* Course Filter + Student Search */}
        <div className="bg-surface-0 border border-border-default rounded-lg p-4 mb-6">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Course Filter */}
            <div>
              <label htmlFor="course-filter" className="block text-sm font-medium text-text-secondary mb-1">
                Course
              </label>
              <select
                id="course-filter"
                value={courseFilter}
                onChange={(e) => { setCourseFilter(e.target.value); clearStudent(); }}
                className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
              >
                <option value="all">All Courses</option>
                {courses.map((c) => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            </div>

            {/* Student Search */}
            <div className="relative">
              <label htmlFor="student-search" className="block text-sm font-medium text-text-secondary mb-1">
                Student
              </label>
              {selectedStudent ? (
                <div className="flex items-center gap-2 border border-blue-200 bg-brand-50 rounded-lg px-3 py-2">
                  <span className="text-sm font-medium text-brand-800 flex-1">{selectedStudent.name}</span>
                  <button
                    type="button"
                    onClick={clearStudent}
                    className="text-blue-400 hover:text-brand-600 focus:outline-none"
                    aria-label="Clear selected student"
                  >
                    <X className="w-4 h-4" aria-hidden="true" />
                  </button>
                </div>
              ) : (
                <>
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-disabled" aria-hidden="true" />
                    <input
                      id="student-search"
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder={courseFilter === 'all' ? 'Select a course first...' : 'Search students...'}
                      disabled={courseFilter === 'all'}
                      className="w-full border border-border-strong rounded-lg pl-9 pr-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500 disabled:bg-surface-1 disabled:text-text-disabled"
                      role="combobox"
                      aria-expanded={searchResults.length > 0}
                      aria-controls="student-search-results"
                      aria-autocomplete="list"
                    />
                  </div>
                  {searchResults.length > 0 && (
                    <ul
                      id="student-search-results"
                      className="absolute z-10 mt-1 w-full bg-surface-0 border border-border-default rounded-lg shadow-lg max-h-48 overflow-y-auto"
                      role="listbox"
                    >
                      {searchResults.map((s) => (
                        <li key={s.id}>
                          <button
                            type="button"
                            onClick={() => selectStudent(s)}
                            className="w-full text-left px-4 py-2 text-sm hover:bg-brand-50 focus:bg-brand-50 focus:outline-none"
                            role="option"
                            aria-selected={false}
                          >
                            {s.name}
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </>
              )}
            </div>
          </div>
        </div>

        {/* Messages */}
        {error && (
          <div className="mb-4 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg text-accent-danger text-sm" role="alert" aria-live="assertive">
            {error}
          </div>
        )}
        {success && (
          <div className="mb-4 p-3 bg-accent-success/10 border border-accent-success/30 rounded-lg text-accent-success text-sm" role="status" aria-live="polite">
            {success}
          </div>
        )}

        {/* No Student Selected */}
        {!selectedStudent && (
          <div className="text-center py-16">
            <Search className="w-12 h-12 text-gray-300 mx-auto mb-3" aria-hidden="true" />
            <p className="text-text-tertiary">Select a course and search for a student to manage accommodations.</p>
          </div>
        )}

        {/* Student Selected */}
        {selectedStudent && (
          <div className="space-y-6">
            {/* Toolbar */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
              <h2 className="text-lg font-semibold text-text-primary">
                Accommodations for {selectedStudent.name}
              </h2>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setShowLog(!showLog)}
                  className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium rounded-lg border transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 ${
                    showLog ? 'bg-brand-50 text-brand-700 border-blue-200' : 'bg-surface-0 text-text-secondary border-border-default hover:bg-surface-1'
                  }`}
                >
                  <FileText className="w-4 h-4" aria-hidden="true" />
                  Application Log
                </button>
                <button
                  type="button"
                  onClick={() => { setShowForm(true); setEditingId(null); setFormData({ ...EMPTY_FORM }); }}
                  className="flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-brand-600 rounded-lg hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <Plus className="w-4 h-4" aria-hidden="true" />
                  Add Accommodation
                </button>
              </div>
            </div>

            {/* Loading */}
            {loading && (
              <div className="flex justify-center py-8">
                <div className="h-8 w-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" role="status" aria-label="Loading accommodations" />
              </div>
            )}

            {/* Accommodations List */}
            {!loading && accommodations.length === 0 && !showForm && (
              <div className="text-center py-12 bg-surface-0 border border-border-default rounded-lg">
                <Shield className="w-12 h-12 text-gray-300 mx-auto mb-3" aria-hidden="true" />
                <p className="text-text-tertiary mb-2">No accommodations on file for this student.</p>
                <button
                  type="button"
                  onClick={() => { setShowForm(true); setEditingId(null); setFormData({ ...EMPTY_FORM }); }}
                  className="text-brand-600 hover:underline text-sm font-medium focus:outline-none focus:underline"
                >
                  Add the first accommodation
                </button>
              </div>
            )}

            {!loading && accommodations.length > 0 && (
              <div className="space-y-3">
                {accommodations.map((a) => (
                  <AccommodationCard
                    key={a.id}
                    accommodation={a}
                    onEdit={handleEdit}
                    onDeactivate={handleDeactivate}
                  />
                ))}
              </div>
            )}

            {/* Application Log */}
            {showLog && (
              <div className="bg-surface-0 border border-border-default rounded-lg p-4">
                <h3 className="font-semibold text-text-primary mb-3 flex items-center gap-2">
                  <FileText className="w-4 h-4 text-text-tertiary" aria-hidden="true" />
                  Application Log
                </h3>
                <ApplicationLog userId={selectedStudent.id} />
              </div>
            )}

            {/* Add/Edit Form */}
            {showForm && (
              <div className="bg-surface-0 border border-border-default rounded-lg p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-lg font-semibold text-text-primary">
                    {editingId ? 'Edit Accommodation' : 'Add Accommodation'}
                  </h3>
                  <button
                    type="button"
                    onClick={resetForm}
                    className="text-text-disabled hover:text-text-secondary focus:outline-none"
                    aria-label="Close form"
                  >
                    <X className="w-5 h-5" aria-hidden="true" />
                  </button>
                </div>

                <form onSubmit={handleSubmit} className="space-y-4">
                  {/* Type */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label htmlFor="acc-type" className="block text-sm font-medium text-text-secondary mb-1">
                        Accommodation Type
                      </label>
                      <select
                        id="acc-type"
                        value={formData.accommodation_type}
                        onChange={(e) => setFormData({ ...formData, accommodation_type: e.target.value })}
                        className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                        required
                      >
                        {ACCOMMODATION_TYPES.map((t) => (
                          <option key={t.value} value={t.value}>{t.label}</option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label htmlFor="plan-type" className="block text-sm font-medium text-text-secondary mb-1">
                        Plan Type
                      </label>
                      <select
                        id="plan-type"
                        value={formData.plan_type}
                        onChange={(e) => setFormData({ ...formData, plan_type: e.target.value })}
                        className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                        required
                      >
                        {PLAN_TYPES.map((p) => (
                          <option key={p.value} value={p.value}>{p.label}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Extended Time Multiplier */}
                  {formData.accommodation_type === 'extended_time' && (
                    <div>
                      <label htmlFor="time-mult" className="block text-sm font-medium text-text-secondary mb-1">
                        Time Multiplier
                      </label>
                      <div className="flex items-center gap-2">
                        <input
                          id="time-mult"
                          type="number"
                          step="0.1"
                          min="1"
                          max="5"
                          value={formData.time_multiplier}
                          onChange={(e) => setFormData({ ...formData, time_multiplier: e.target.value })}
                          className="w-32 border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                          required
                        />
                        <span className="text-sm text-text-tertiary">x (e.g., 1.5x means 50% more time)</span>
                      </div>
                    </div>
                  )}

                  {/* Extra Days */}
                  {formData.accommodation_type === 'modified_due_dates' && (
                    <div>
                      <label htmlFor="extra-days" className="block text-sm font-medium text-text-secondary mb-1">
                        Extra Days
                      </label>
                      <div className="flex items-center gap-2">
                        <input
                          id="extra-days"
                          type="number"
                          min="1"
                          max="30"
                          value={formData.extra_days}
                          onChange={(e) => setFormData({ ...formData, extra_days: e.target.value })}
                          className="w-32 border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                          required
                        />
                        <span className="text-sm text-text-tertiary">additional day(s) after due date</span>
                      </div>
                    </div>
                  )}

                  {/* Date Range */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div>
                      <label htmlFor="eff-start" className="block text-sm font-medium text-text-secondary mb-1">
                        Effective Start Date
                      </label>
                      <input
                        id="eff-start"
                        type="date"
                        value={formData.effective_start}
                        onChange={(e) => setFormData({ ...formData, effective_start: e.target.value })}
                        className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                      />
                    </div>
                    <div>
                      <label htmlFor="eff-end" className="block text-sm font-medium text-text-secondary mb-1">
                        Effective End Date
                      </label>
                      <input
                        id="eff-end"
                        type="date"
                        value={formData.effective_end}
                        onChange={(e) => setFormData({ ...formData, effective_end: e.target.value })}
                        className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                      />
                    </div>
                  </div>

                  {/* Notes */}
                  <div>
                    <label htmlFor="acc-notes" className="block text-sm font-medium text-text-secondary mb-1">
                      Notes
                    </label>
                    <textarea
                      id="acc-notes"
                      value={formData.notes}
                      onChange={(e) => setFormData({ ...formData, notes: e.target.value })}
                      placeholder="Additional details about this accommodation..."
                      className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500 resize-none"
                      rows={3}
                    />
                  </div>

                  {/* Submit */}
                  <div className="flex items-center gap-3 pt-2">
                    <button
                      type="submit"
                      disabled={submitting}
                      className="px-6 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {submitting ? 'Saving...' : editingId ? 'Update Accommodation' : 'Create Accommodation'}
                    </button>
                    <button
                      type="button"
                      onClick={resetForm}
                      className="px-4 py-2 text-sm font-medium text-text-secondary hover:text-text-primary focus:outline-none focus:underline"
                    >
                      Cancel
                    </button>
                  </div>
                </form>
              </div>
            )}
          </div>
        )}

        {/* Deactivate Confirmation Dialog */}
        {deleteConfirm && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog" aria-modal="true" aria-labelledby="deactivate-title">
            <div className="bg-surface-0 rounded-lg shadow-xl p-6 max-w-sm w-full mx-4">
              <div className="flex items-center gap-3 mb-4">
                <div className="w-10 h-10 rounded-full bg-accent-danger/20 flex items-center justify-center shrink-0">
                  <AlertTriangle className="w-5 h-5 text-accent-danger" aria-hidden="true" />
                </div>
                <div>
                  <h3 id="deactivate-title" className="font-semibold text-text-primary">Deactivate Accommodation</h3>
                  <p className="text-sm text-text-tertiary">This will stop the accommodation from auto-applying.</p>
                </div>
              </div>
              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setDeleteConfirm(null)}
                  className="px-4 py-2 text-sm font-medium text-text-secondary hover:text-text-primary focus:outline-none focus:underline"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={confirmDeactivate}
                  className="px-4 py-2 text-sm font-medium text-white bg-accent-danger rounded-lg hover:bg-accent-danger/90 focus:outline-none focus:ring-2 focus:ring-red-500"
                >
                  Deactivate
                </button>
              </div>
            </div>
          </div>
        )}
      </main>
    </Layout>
  );
};

export default AccommodationsPage;
