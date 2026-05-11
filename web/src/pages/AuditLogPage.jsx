import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { FileText, Download, Filter, ChevronDown, ChevronUp, ArrowRight, ArrowLeft } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const TAB_ACTIVITY = 'activity';
const TAB_GRADES = 'grades';

const EVENT_TYPE_COLORS = {
  grade_change: 'bg-accent-warning/20 text-accent-warning',
  course_update: 'bg-brand-100 text-brand-800',
  enrollment_change: 'bg-accent-success/20 text-accent-success',
  assignment_update: 'bg-purple-100 text-purple-800',
  submission_update: 'bg-indigo-100 text-indigo-800',
  user_update: 'bg-pink-100 text-pink-800',
  account_update: 'bg-surface-2 text-text-primary',
};

const EVENT_TYPE_OPTIONS = [
  { value: '', label: 'All Event Types' },
  { value: 'grade_change', label: 'Grade Change' },
  { value: 'course_update', label: 'Course Update' },
  { value: 'enrollment_change', label: 'Enrollment Change' },
  { value: 'assignment_update', label: 'Assignment Update' },
  { value: 'submission_update', label: 'Submission Update' },
  { value: 'user_update', label: 'User Update' },
  { value: 'account_update', label: 'Account Update' },
];

const GRADING_METHOD_LABELS = {
  manual: 'Manual',
  auto_grade: 'Auto Grade',
  rubric: 'Rubric',
  speedgrader: 'SpeedGrader',
};

const AuditLogPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [activeTab, setActiveTab] = useState(TAB_ACTIVITY);
  const [course, setCourse] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Course Activity state
  const [auditLogs, setAuditLogs] = useState([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditPage, setAuditPage] = useState(1);
  const [auditHasMore, setAuditHasMore] = useState(false);
  const [expandedRows, setExpandedRows] = useState({});

  // Activity filters
  const [eventTypeFilter, setEventTypeFilter] = useState('');
  const [dateFromFilter, setDateFromFilter] = useState('');
  const [dateToFilter, setDateToFilter] = useState('');
  const [userIdFilter, setUserIdFilter] = useState('');
  const [showFilters, setShowFilters] = useState(false);

  // Grade Changes state
  const [gradeChanges, setGradeChanges] = useState([]);
  const [gradeLoading, setGradeLoading] = useState(false);
  const [gradePage, setGradePage] = useState(1);
  const [gradeHasMore, setGradeHasMore] = useState(false);

  // Grade Change filters
  const [studentIdFilter, setStudentIdFilter] = useState('');
  const [graderIdFilter, setGraderIdFilter] = useState('');
  const [assignmentIdFilter, setAssignmentIdFilter] = useState('');
  const [gradeDateFromFilter, setGradeDateFromFilter] = useState('');
  const [gradeDateToFilter, setGradeDateToFilter] = useState('');
  const [showGradeFilters, setShowGradeFilters] = useState(false);

  useEffect(() => {
    const fetchCourse = async () => {
      try {
        const courseData = await api.getCourse(courseId);
        setCourse(courseData);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchCourse();
  }, [courseId]);

  const buildAuditFilterParams = useCallback(() => {
    const params = new URLSearchParams();
    if (eventTypeFilter) params.set('event_type', eventTypeFilter);
    if (dateFromFilter) params.set('date_from', dateFromFilter);
    if (dateToFilter) params.set('date_to', dateToFilter);
    if (userIdFilter) params.set('user_id', userIdFilter);
    return params.toString();
  }, [eventTypeFilter, dateFromFilter, dateToFilter, userIdFilter]);

  const buildGradeFilterParams = useCallback(() => {
    const params = new URLSearchParams();
    if (studentIdFilter) params.set('student_id', studentIdFilter);
    if (graderIdFilter) params.set('grader_id', graderIdFilter);
    if (assignmentIdFilter) params.set('assignment_id', assignmentIdFilter);
    if (gradeDateFromFilter) params.set('date_from', gradeDateFromFilter);
    if (gradeDateToFilter) params.set('date_to', gradeDateToFilter);
    return params.toString();
  }, [studentIdFilter, graderIdFilter, assignmentIdFilter, gradeDateFromFilter, gradeDateToFilter]);

  const fetchAuditLogs = useCallback(async (page = 1) => {
    setAuditLoading(true);
    try {
      const filterParams = buildAuditFilterParams();
      const separator = filterParams ? '&' : '';
      const result = await api.request(`/courses/${courseId}/audit_log?page=${page}&per_page=25${separator}${filterParams}`);
      setAuditLogs(result.data || []);
      setAuditHasMore(!!result.pagination?.next);
      setAuditPage(page);
    } catch (err) {
      setAuditLogs([]);
    } finally {
      setAuditLoading(false);
    }
  }, [courseId, buildAuditFilterParams]);

  const fetchGradeChanges = useCallback(async (page = 1) => {
    setGradeLoading(true);
    try {
      const filterParams = buildGradeFilterParams();
      const separator = filterParams ? '&' : '';
      const result = await api.request(`/courses/${courseId}/grade_change_log?page=${page}&per_page=25${separator}${filterParams}`);
      setGradeChanges(result.data || []);
      setGradeHasMore(!!result.pagination?.next);
      setGradePage(page);
    } catch (err) {
      setGradeChanges([]);
    } finally {
      setGradeLoading(false);
    }
  }, [courseId, buildGradeFilterParams]);

  useEffect(() => {
    if (activeTab === TAB_ACTIVITY) {
      fetchAuditLogs(1);
    }
  }, [activeTab, fetchAuditLogs]);

  useEffect(() => {
    if (activeTab === TAB_GRADES) {
      fetchGradeChanges(1);
    }
  }, [activeTab, fetchGradeChanges]);

  const toggleRow = (id) => {
    setExpandedRows((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const handleExportAuditCSV = async () => {
    const filterParams = buildAuditFilterParams();
    const separator = filterParams ? '?' : '';
    try {
      const res = await fetch(`/api/v1/courses/${courseId}/audit_log.csv${separator}${filterParams}`, { credentials: 'include' });
      if (!res.ok) throw new Error('Export failed');
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit_log_${courseId}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch { /* ignore */ }
  };

  const handleExportGradeCSV = async () => {
    const filterParams = buildGradeFilterParams();
    const separator = filterParams ? '?' : '';
    try {
      const res = await fetch(`/api/v1/courses/${courseId}/grade_change_log.csv${separator}${filterParams}`, { credentials: 'include' });
      if (!res.ok) throw new Error('Export failed');
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `grade_change_log_${courseId}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch { /* ignore */ }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatPayload = (payloadStr) => {
    if (!payloadStr) return '';
    try {
      const parsed = JSON.parse(payloadStr);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return payloadStr;
    }
  };

  const getEventTypeClass = (eventType) => {
    return EVENT_TYPE_COLORS[eventType] || 'bg-surface-2 text-text-primary';
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div>
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="text-center py-12 text-accent-danger">{error}</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      {/* Header */}
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:text-brand-800 text-sm flex items-center mb-2">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to Course
        </Link>
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <FileText className="w-8 h-8 text-text-disabled" />
            <div>
              <h1 className="text-2xl font-bold text-text-primary">Audit Log</h1>
              <p className="text-sm text-text-tertiary">{course?.name || 'Course'}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-border-default mb-6" role="tablist" aria-label="Audit log tabs">
        <nav className="flex space-x-8">
          <button
            role="tab"
            aria-selected={activeTab === TAB_ACTIVITY}
            aria-controls="panel-activity"
            onClick={() => setActiveTab(TAB_ACTIVITY)}
            className={`pb-3 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === TAB_ACTIVITY
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            Course Activity
          </button>
          <button
            role="tab"
            aria-selected={activeTab === TAB_GRADES}
            aria-controls="panel-grades"
            onClick={() => setActiveTab(TAB_GRADES)}
            className={`pb-3 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === TAB_GRADES
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            Grade Changes
          </button>
        </nav>
      </div>

      {/* Course Activity Tab */}
      {activeTab === TAB_ACTIVITY && (
        <div id="panel-activity" role="tabpanel" aria-labelledby="tab-activity">
          {/* Filter Bar */}
          <div className="bg-surface-0 rounded-lg shadow mb-4 p-4">
            <div className="flex items-center justify-between mb-3">
              <button
                onClick={() => setShowFilters(!showFilters)}
                className="flex items-center space-x-2 text-sm font-medium text-text-secondary hover:text-text-primary"
              >
                <Filter className="w-4 h-4" />
                <span>Filters</span>
                {showFilters ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
              </button>
              <button
                onClick={handleExportAuditCSV}
                className="flex items-center space-x-2 px-3 py-1.5 bg-accent-success text-white text-sm rounded hover:bg-accent-success/90 transition-colors"
              >
                <Download className="w-4 h-4" />
                <span>Export CSV</span>
              </button>
            </div>

            {showFilters && (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 pt-3 border-t border-border-subtle">
                <div>
                  <label htmlFor="event-type-filter" className="block text-xs font-medium text-text-tertiary mb-1">Event Type</label>
                  <select
                    id="event-type-filter"
                    value={eventTypeFilter}
                    onChange={(e) => setEventTypeFilter(e.target.value)}
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  >
                    {EVENT_TYPE_OPTIONS.map((opt) => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label htmlFor="user-id-filter" className="block text-xs font-medium text-text-tertiary mb-1">User ID</label>
                  <input
                    id="user-id-filter"
                    type="text"
                    value={userIdFilter}
                    onChange={(e) => setUserIdFilter(e.target.value)}
                    placeholder="Filter by user ID"
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="date-from-filter" className="block text-xs font-medium text-text-tertiary mb-1">Date From</label>
                  <input
                    id="date-from-filter"
                    type="date"
                    value={dateFromFilter}
                    onChange={(e) => setDateFromFilter(e.target.value)}
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="date-to-filter" className="block text-xs font-medium text-text-tertiary mb-1">Date To</label>
                  <input
                    id="date-to-filter"
                    type="date"
                    value={dateToFilter}
                    onChange={(e) => setDateToFilter(e.target.value)}
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Activity Table */}
          {auditLoading ? (
            <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading audit log...
</div>
          ) : auditLogs.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <FileText className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary">No audit log entries found.</p>
              <p className="text-text-disabled text-sm mt-1">Activity will appear here as changes are made to this course.</p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full border-collapse" aria-label="Course audit log">
                  <caption className="sr-only">Course activity audit log showing recent changes and actions</caption>
                  <thead>
                    <tr className="bg-surface-1">
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Date</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">User</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Event Type</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Action</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Context</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Details</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-default">
                    {auditLogs.map((log) => (
                      <React.Fragment key={log.id}>
                        <tr className="hover:bg-surface-1">
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">{formatDate(log.created_at)}</td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-primary">User {log.user_id}</td>
                          <td className="px-4 py-3 whitespace-nowrap">
                            <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${getEventTypeClass(log.event_type)}`}>
                              {log.event_type}
                            </span>
                          </td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">{log.action}</td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">{log.context_type} #{log.context_id}</td>
                          <td className="px-4 py-3 whitespace-nowrap">
                            {log.payload && log.payload !== '{}' && (
                              <button
                                onClick={() => toggleRow(log.id)}
                                className="flex items-center space-x-1 text-sm text-brand-600 hover:text-brand-800"
                                aria-expanded={!!expandedRows[log.id]}
                                aria-controls={`payload-${log.id}`}
                              >
                                <span>View</span>
                                {expandedRows[log.id] ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                              </button>
                            )}
                          </td>
                        </tr>
                        {expandedRows[log.id] && (
                          <tr id={`payload-${log.id}`}>
                            <td colSpan={6} className="px-4 py-3 bg-surface-1">
                              <pre className="text-xs text-text-secondary whitespace-pre-wrap font-mono bg-surface-2 p-3 rounded max-w-full overflow-x-auto">
                                {formatPayload(log.payload)}
                              </pre>
                              {log.ip_address && (
                                <p className="text-xs text-text-disabled mt-2">IP: {log.ip_address} | User-Agent: {log.user_agent}</p>
                              )}
                            </td>
                          </tr>
                        )}
                      </React.Fragment>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="px-4 py-3 bg-surface-1 border-t border-border-default flex items-center justify-between">
                <button
                  onClick={() => fetchAuditLogs(auditPage - 1)}
                  disabled={auditPage <= 1}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-text-secondary">Page {auditPage}</span>
                <button
                  onClick={() => fetchAuditLogs(auditPage + 1)}
                  disabled={!auditHasMore}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Grade Changes Tab */}
      {activeTab === TAB_GRADES && (
        <div id="panel-grades" role="tabpanel" aria-labelledby="tab-grades">
          {/* Filter Bar */}
          <div className="bg-surface-0 rounded-lg shadow mb-4 p-4">
            <div className="flex items-center justify-between mb-3">
              <button
                onClick={() => setShowGradeFilters(!showGradeFilters)}
                className="flex items-center space-x-2 text-sm font-medium text-text-secondary hover:text-text-primary"
              >
                <Filter className="w-4 h-4" />
                <span>Filters</span>
                {showGradeFilters ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
              </button>
              <button
                onClick={handleExportGradeCSV}
                className="flex items-center space-x-2 px-3 py-1.5 bg-accent-success text-white text-sm rounded hover:bg-accent-success/90 transition-colors"
              >
                <Download className="w-4 h-4" />
                <span>Export CSV</span>
              </button>
            </div>

            {showGradeFilters && (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 pt-3 border-t border-border-subtle">
                <div>
                  <label htmlFor="student-id-filter" className="block text-xs font-medium text-text-tertiary mb-1">Student ID</label>
                  <input
                    id="student-id-filter"
                    type="text"
                    value={studentIdFilter}
                    onChange={(e) => setStudentIdFilter(e.target.value)}
                    placeholder="Filter by student"
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="grader-id-filter" className="block text-xs font-medium text-text-tertiary mb-1">Grader ID</label>
                  <input
                    id="grader-id-filter"
                    type="text"
                    value={graderIdFilter}
                    onChange={(e) => setGraderIdFilter(e.target.value)}
                    placeholder="Filter by grader"
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="assignment-id-filter" className="block text-xs font-medium text-text-tertiary mb-1">Assignment ID</label>
                  <input
                    id="assignment-id-filter"
                    type="text"
                    value={assignmentIdFilter}
                    onChange={(e) => setAssignmentIdFilter(e.target.value)}
                    placeholder="Filter by assignment"
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="grade-date-from" className="block text-xs font-medium text-text-tertiary mb-1">Date From</label>
                  <input
                    id="grade-date-from"
                    type="date"
                    value={gradeDateFromFilter}
                    onChange={(e) => setGradeDateFromFilter(e.target.value)}
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
                <div>
                  <label htmlFor="grade-date-to" className="block text-xs font-medium text-text-tertiary mb-1">Date To</label>
                  <input
                    id="grade-date-to"
                    type="date"
                    value={gradeDateToFilter}
                    onChange={(e) => setGradeDateToFilter(e.target.value)}
                    className="w-full border border-border-strong rounded px-3 py-1.5 text-sm focus:ring-brand-500 focus:border-brand-500"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Grade Changes Table */}
          {gradeLoading ? (
            <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading grade changes...
</div>
          ) : gradeChanges.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <FileText className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary">No grade changes found.</p>
              <p className="text-text-disabled text-sm mt-1">Grade changes will appear here when grades are modified.</p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full border-collapse" aria-label="Grade change log">
                  <caption className="sr-only">Grade change log showing all grade modifications for this course</caption>
                  <thead>
                    <tr className="bg-surface-1">
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Date</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Student</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Assignment</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Grader</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Grade</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Score</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">Method</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-default">
                    {gradeChanges.map((gc) => (
                      <tr key={gc.id} className="hover:bg-surface-1">
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">{formatDate(gc.created_at)}</td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-primary">Student {gc.student_id}</td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">Assignment {gc.assignment_id}</td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">User {gc.grader_id}</td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm">
                          <div className="flex items-center space-x-1">
                            <span className={gc.old_grade ? 'text-accent-danger line-through' : 'text-text-disabled'}>{gc.old_grade || 'none'}</span>
                            <ArrowRight className="w-3 h-3 text-text-disabled" />
                            <span className="text-accent-success font-medium">{gc.new_grade || 'none'}</span>
                            {gc.excused && (
                              <span className="ml-1 inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium bg-accent-warning/20 text-accent-warning">EX</span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm">
                          <div className="flex items-center space-x-1">
                            <span className="text-text-tertiary">{gc.old_score != null ? gc.old_score : '-'}</span>
                            <ArrowRight className="w-3 h-3 text-text-disabled" />
                            <span className="text-text-primary font-medium">{gc.new_score != null ? gc.new_score : '-'}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">
                          {GRADING_METHOD_LABELS[gc.grading_method] || gc.grading_method}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="px-4 py-3 bg-surface-1 border-t border-border-default flex items-center justify-between">
                <button
                  onClick={() => fetchGradeChanges(gradePage - 1)}
                  disabled={gradePage <= 1}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-text-secondary">Page {gradePage}</span>
                <button
                  onClick={() => fetchGradeChanges(gradePage + 1)}
                  disabled={!gradeHasMore}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </Layout>
  );
};

export default AuditLogPage;
