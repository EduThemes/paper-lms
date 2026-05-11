import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { ArrowLeft, Download, Users, CheckCircle, XCircle, Clock, FileText, Calendar, List } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

/* ─── Constants ─── */

const STATUSES = [
  { key: 'present', label: 'Present', color: 'bg-accent-success', hoverColor: 'hover:bg-accent-success', lightBg: 'bg-accent-success/10', textColor: 'text-accent-success', borderColor: 'border-accent-success/30' },
  { key: 'absent', label: 'Absent', color: 'bg-accent-danger', hoverColor: 'hover:bg-accent-danger', lightBg: 'bg-accent-danger/10', textColor: 'text-accent-danger', borderColor: 'border-accent-danger/30' },
  { key: 'tardy', label: 'Tardy', color: 'bg-accent-warning', hoverColor: 'hover:bg-yellow-600', lightBg: 'bg-accent-warning/10', textColor: 'text-accent-warning', borderColor: 'border-accent-warning/30' },
  { key: 'excused', label: 'Excused', color: 'bg-brand-500', hoverColor: 'hover:bg-brand-600', lightBg: 'bg-brand-50', textColor: 'text-brand-700', borderColor: 'border-blue-200' },
];

const STATUS_MAP = Object.fromEntries(STATUSES.map((s) => [s.key, s]));

function parseLocalDate(dateStr) {
  // Parse "YYYY-MM-DD" as local date (not UTC)
  if (typeof dateStr === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(dateStr)) {
    const [y, m, d] = dateStr.split('-').map(Number);
    return new Date(y, m - 1, d);
  }
  return dateStr instanceof Date ? dateStr : new Date(dateStr);
}

function formatDate(dateStr) {
  if (!dateStr) return '';
  const d = parseLocalDate(dateStr);
  return d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
}

function toISODate(date) {
  const d = date instanceof Date ? date : new Date(date);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function getDaysInMonth(year, month) {
  return new Date(year, month + 1, 0).getDate();
}

function getFirstDayOfMonth(year, month) {
  return new Date(year, month, 1).getDay();
}

/* ─── Summary Bar ─── */

const SummaryBar = ({ records }) => {
  const counts = { present: 0, absent: 0, tardy: 0, excused: 0, unmarked: 0 };
  const total = records.length;
  for (const r of records) {
    if (r.status && counts[r.status] !== undefined) {
      counts[r.status]++;
    } else {
      counts.unmarked++;
    }
  }
  return (
    <div className="flex flex-wrap items-center gap-4 text-sm" role="status" aria-label="Attendance summary">
      <span className="font-medium text-text-secondary">
        <Users className="w-4 h-4 inline-block mr-1" aria-hidden="true" />
        {total} students
      </span>
      <span className="text-accent-success">
        <span className="inline-block w-2.5 h-2.5 rounded-full bg-accent-success mr-1" aria-hidden="true" />
        {counts.present} Present
      </span>
      <span className="text-accent-danger">
        <span className="inline-block w-2.5 h-2.5 rounded-full bg-accent-danger mr-1" aria-hidden="true" />
        {counts.absent} Absent
      </span>
      <span className="text-accent-warning">
        <span className="inline-block w-2.5 h-2.5 rounded-full bg-accent-warning mr-1" aria-hidden="true" />
        {counts.tardy} Tardy
      </span>
      <span className="text-brand-700">
        <span className="inline-block w-2.5 h-2.5 rounded-full bg-brand-500 mr-1" aria-hidden="true" />
        {counts.excused} Excused
      </span>
      {counts.unmarked > 0 && (
        <span className="text-text-tertiary">
          <span className="inline-block w-2.5 h-2.5 rounded-full bg-gray-300 mr-1" aria-hidden="true" />
          {counts.unmarked} Unmarked
        </span>
      )}
    </div>
  );
};

/* ─── Student Attendance Row ─── */

const StudentRow = ({ record, onStatusChange, onNoteChange, saving }) => {
  const [showNotes, setShowNotes] = useState(false);
  const [note, setNote] = useState(record.notes || '');

  useEffect(() => {
    setNote(record.notes || '');
  }, [record.notes]);

  const handleNoteBlur = () => {
    if (note !== (record.notes || '')) {
      onNoteChange(record.user_id, note);
    }
  };

  return (
    <div className="bg-surface-0 border border-border-default rounded-lg p-4">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3">
        {/* Student Name */}
        <div className="flex-1 min-w-0">
          <p className="font-medium text-text-primary truncate">{record.student_name || `Student ${record.user_id}`}</p>
          {record.status && (
            <span className={`inline-block mt-1 text-xs px-2 py-0.5 rounded-full ${STATUS_MAP[record.status]?.lightBg || 'bg-surface-2'} ${STATUS_MAP[record.status]?.textColor || 'text-text-secondary'}`}>
              {STATUS_MAP[record.status]?.label || record.status}
            </span>
          )}
        </div>

        {/* Status Buttons */}
        <div className="flex gap-2" role="radiogroup" aria-label={`Attendance status for ${record.student_name}`}>
          {STATUSES.map((status) => {
            const isActive = record.status === status.key;
            return (
              <button
                key={status.key}
                type="button"
                role="radio"
                aria-checked={isActive}
                aria-label={status.label}
                onClick={() => onStatusChange(record.user_id, status.key)}
                disabled={saving}
                className={`
                  px-3 py-1.5 rounded-lg text-xs font-medium transition-all
                  focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-brand-500
                  ${isActive
                    ? `${status.color} text-white shadow-sm`
                    : `bg-surface-2 text-text-secondary hover:bg-border-default`
                  }
                  disabled:opacity-50 disabled:cursor-not-allowed
                `}
              >
                {status.label}
              </button>
            );
          })}
        </div>

        {/* Notes Toggle */}
        <button
          type="button"
          onClick={() => setShowNotes(!showNotes)}
          className="text-text-disabled hover:text-text-secondary focus:outline-none focus:text-text-secondary p-1"
          aria-label={showNotes ? 'Hide notes' : 'Show notes'}
          aria-expanded={showNotes}
        >
          <FileText className="w-4 h-4" aria-hidden="true" />
        </button>
      </div>

      {/* Notes Field */}
      {showNotes && (
        <div className="mt-3 pt-3 border-t border-border-subtle">
          <label htmlFor={`note-${record.user_id}`} className="sr-only">
            Notes for {record.student_name}
          </label>
          <textarea
            id={`note-${record.user_id}`}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            onBlur={handleNoteBlur}
            placeholder="Add notes (e.g., left early, arrived at 9:15)..."
            className="w-full text-sm border border-border-default rounded-lg px-3 py-2 focus:border-brand-500 focus:ring-1 focus:ring-brand-500 resize-none"
            rows={2}
          />
        </div>
      )}
    </div>
  );
};

/* ─── Calendar Heatmap View ─── */

const CalendarHeatmap = ({ courseId }) => {
  const [calDate, setCalDate] = useState(new Date());
  const [monthData, setMonthData] = useState({});
  const [calLoading, setCalLoading] = useState(false);

  const year = calDate.getFullYear();
  const month = calDate.getMonth();
  const daysInMonth = getDaysInMonth(year, month);
  const firstDay = getFirstDayOfMonth(year, month);

  useEffect(() => {
    const fetchMonth = async () => {
      setCalLoading(true);
      try {
        // Fetch attendance for each day of the month
        const startDate = `${year}-${String(month + 1).padStart(2, '0')}-01`;
        const endDate = `${year}-${String(month + 1).padStart(2, '0')}-${String(daysInMonth).padStart(2, '0')}`;
        const data = await api.getClassAttendance(courseId, startDate);
        const records = Array.isArray(data) ? data : (data?.data || []);
        // Group by date for calendar heatmap
        const grouped = {};
        for (const r of records) {
          const d = r.date?.split('T')?.[0] || startDate;
          if (!grouped[d]) grouped[d] = { present: 0, absent: 0, tardy: 0, excused: 0, total: 0 };
          if (r.status && grouped[d][r.status] !== undefined) grouped[d][r.status]++;
          grouped[d].total++;
        }
        setMonthData(grouped);
      } catch {
        // silently fail
      } finally {
        setCalLoading(false);
      }
    };
    fetchMonth();
  }, [courseId, year, month, daysInMonth]);

  const prevMonth = () => setCalDate(new Date(year, month - 1, 1));
  const nextMonth = () => setCalDate(new Date(year, month + 1, 1));

  const getHeatColor = (day) => {
    const dateKey = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    const dayData = monthData[dateKey];
    if (!dayData) return 'bg-surface-1 text-text-disabled';
    const rate = dayData.total > 0 ? dayData.present / dayData.total : 0;
    if (rate >= 0.95) return 'bg-green-400 text-white';
    if (rate >= 0.85) return 'bg-green-300 text-green-900';
    if (rate >= 0.75) return 'bg-yellow-300 text-yellow-900';
    if (rate >= 0.5) return 'bg-orange-300 text-orange-900';
    return 'bg-red-400 text-white';
  };

  const getTooltip = (day) => {
    const dateKey = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    const dayData = monthData[dateKey];
    if (!dayData) return 'No data';
    return `${dayData.present || 0}/${dayData.total || 0} present`;
  };

  const monthLabel = calDate.toLocaleDateString('en-US', { month: 'long', year: 'numeric' });

  return (
    <div className="bg-surface-0 border border-border-default rounded-lg p-6">
      {/* Month Navigation */}
      <div className="flex items-center justify-between mb-4">
        <button
          type="button"
          onClick={prevMonth}
          className="p-2 rounded-lg hover:bg-surface-2 focus:outline-none focus:ring-2 focus:ring-brand-500"
          aria-label="Previous month"
        >
          <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path fillRule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clipRule="evenodd" /></svg>
        </button>
        <h3 className="text-lg font-semibold text-text-primary">{monthLabel}</h3>
        <button
          type="button"
          onClick={nextMonth}
          className="p-2 rounded-lg hover:bg-surface-2 focus:outline-none focus:ring-2 focus:ring-brand-500"
          aria-label="Next month"
        >
          <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true"><path fillRule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clipRule="evenodd" /></svg>
        </button>
      </div>

      {calLoading && (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" role="status" aria-label="Loading calendar data" />
        </div>
      )}

      {!calLoading && (
        <>
          {/* Day headers */}
          <div className="grid grid-cols-7 gap-1 mb-1">
            {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((d) => (
              <div key={d} className="text-center text-xs font-medium text-text-tertiary py-1">{d}</div>
            ))}
          </div>

          {/* Calendar grid */}
          <div className="grid grid-cols-7 gap-1" role="grid" aria-label={`Attendance heatmap for ${monthLabel}`}>
            {/* Empty cells for offset */}
            {Array.from({ length: firstDay }).map((_, i) => (
              <div key={`empty-${i}`} className="aspect-square" role="gridcell" />
            ))}
            {/* Day cells */}
            {Array.from({ length: daysInMonth }).map((_, i) => {
              const day = i + 1;
              return (
                <div
                  key={day}
                  className={`aspect-square flex items-center justify-center rounded text-xs font-medium ${getHeatColor(day)} cursor-default`}
                  title={getTooltip(day)}
                  role="gridcell"
                  aria-label={`${calDate.toLocaleDateString('en-US', { month: 'long' })} ${day}: ${getTooltip(day)}`}
                >
                  {day}
                </div>
              );
            })}
          </div>

          {/* Legend */}
          <div className="flex items-center justify-center gap-2 mt-4 text-xs text-text-tertiary">
            <span>Low</span>
            <div className="w-4 h-4 rounded bg-red-400" aria-hidden="true" />
            <div className="w-4 h-4 rounded bg-orange-300" aria-hidden="true" />
            <div className="w-4 h-4 rounded bg-yellow-300" aria-hidden="true" />
            <div className="w-4 h-4 rounded bg-green-300" aria-hidden="true" />
            <div className="w-4 h-4 rounded bg-green-400" aria-hidden="true" />
            <span>High</span>
          </div>
        </>
      )}
    </div>
  );
};

/* ─── Student Self-View ─── */

const StudentAttendanceView = ({ courseId, userId }) => {
  const [summary, setSummary] = useState(null);
  const [records, setRecords] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const data = await api.getStudentAttendanceSummary(courseId, userId);
        setSummary(data?.summary || null);
        setRecords(data?.records || []);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, userId]);

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <div className="h-8 w-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" role="status" aria-label="Loading attendance" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-accent-danger/10 border border-accent-danger/30 rounded-lg p-4 text-accent-danger text-sm" role="alert">{error}</div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Summary Stats */}
      {summary && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {[
            { label: 'Present', value: summary.present || 0, color: 'text-accent-success', bg: 'bg-accent-success/10' },
            { label: 'Absent', value: summary.absent || 0, color: 'text-accent-danger', bg: 'bg-accent-danger/10' },
            { label: 'Tardy', value: summary.tardy || 0, color: 'text-accent-warning', bg: 'bg-accent-warning/10' },
            { label: 'Excused', value: summary.excused || 0, color: 'text-brand-600', bg: 'bg-brand-50' },
          ].map((stat) => (
            <div key={stat.label} className={`${stat.bg} rounded-lg p-4 text-center`}>
              <p className={`text-2xl font-bold ${stat.color}`}>{stat.value}</p>
              <p className="text-sm text-text-secondary">{stat.label}</p>
            </div>
          ))}
        </div>
      )}

      {/* Record List */}
      <div className="bg-surface-0 border border-border-default rounded-lg divide-y divide-gray-100">
        {records.length === 0 ? (
          <p className="p-6 text-center text-text-tertiary">No attendance records found.</p>
        ) : (
          records.map((r, i) => (
            <div key={i} className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-text-secondary">{formatDate(r.date)}</span>
              {r.status ? (
                <span className={`text-xs font-medium px-2.5 py-1 rounded-full ${STATUS_MAP[r.status]?.lightBg || 'bg-surface-2'} ${STATUS_MAP[r.status]?.textColor || 'text-text-secondary'}`}>
                  {STATUS_MAP[r.status]?.label || r.status}
                </span>
              ) : (
                <span className="text-xs text-text-disabled">No record</span>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
};

/* ─── Main Page ─── */

const AttendancePage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [course, setCourse] = useState(null);
  const [date, setDate] = useState(toISODate(new Date()));
  const [records, setRecords] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [view, setView] = useState('roster'); // 'roster' | 'calendar'

  const isStudent = isTeacher === false;

  const fetchAttendance = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const courseData = await api.getCourse(courseId);
      setCourse(courseData);

      // Fetch attendance records for this date
      const attData = await api.getClassAttendance(courseId, date);
      const attRecords = Array.isArray(attData) ? attData : (attData?.data || []);

      if (attRecords.length > 0) {
        setRecords(attRecords);
      } else {
        // No attendance records yet — build roster from enrollments
        const enrollmentResult = await api.getEnrollments(courseId, 1, 200);
        const enrollments = enrollmentResult.data || [];
        const studentEnrollments = enrollments.filter(
          (e) => e.type === 'StudentEnrollment' || e.role === 'StudentEnrollment' || e.enrollment_type === 'student'
        );
        const seen = new Set();
        const roster = [];
        for (const e of studentEnrollments) {
          const uid = e.user_id || e.user?.id;
          if (uid && !seen.has(uid)) {
            seen.add(uid);
            roster.push({
              user_id: uid,
              student_name: e.user?.name || e.user?.display_name || `User ${uid}`,
              status: null,
              notes: '',
            });
          }
        }
        roster.sort((a, b) => (a.student_name || '').localeCompare(b.student_name || ''));
        setRecords(roster);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId, date]);

  useEffect(() => {
    fetchAttendance();
  }, [fetchAttendance]);

  const saveAttendance = useCallback(async (updatedRecords) => {
    setSaving(true);
    try {
      await api.recordAttendance(courseId, { date, records: updatedRecords });
    } catch {
      // Silently fail — records are updated optimistically
    } finally {
      setSaving(false);
    }
  }, [courseId, date]);

  const handleStatusChange = (userId, status) => {
    const updated = records.map((r) =>
      r.user_id === userId ? { ...r, status: r.status === status ? null : status } : r
    );
    setRecords(updated);
    saveAttendance(updated);
  };

  const handleNoteChange = (userId, notes) => {
    const updated = records.map((r) =>
      r.user_id === userId ? { ...r, notes } : r
    );
    setRecords(updated);
    saveAttendance(updated);
  };

  const handleMarkAllPresent = () => {
    const updated = records.map((r) => ({ ...r, status: 'present' }));
    setRecords(updated);
    saveAttendance(updated);
  };

  const handleExportCSV = async () => {
    try {
      const res = await fetch(`/api/v1/courses/${courseId}/attendance/export.csv`, { credentials: 'include' });
      if (!res.ok) throw new Error('Export failed');
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `attendance_${courseId}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      setError('Failed to export attendance');
    }
  };

  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  /* ── Student View ── */
  if (isStudent) {
    return (
      <Layout>
        <main id="main-content" className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="mb-6">
            <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center gap-1 mb-2">
              <ArrowLeft className="w-4 h-4" aria-hidden="true" />
              Back to Course
            </Link>
            <h1 className="text-2xl font-bold text-text-primary">My Attendance</h1>
          </div>
          <StudentAttendanceView courseId={courseId} userId={user?.id} />
        </main>
      </Layout>
    );
  }

  /* ── Teacher View ── */
  return (
    <Layout>
      <CourseNav />
      <main id="main-content" className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Header */}
        <div className="mb-6">
          <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center gap-1 mb-2">
            <ArrowLeft className="w-4 h-4" aria-hidden="true" />
            Back to Course
          </Link>
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-text-primary">
                Attendance {course ? `- ${course.name}` : ''}
              </h1>
              <p className="text-sm text-text-tertiary mt-1">{formatDate(date)}</p>
            </div>
            <div className="flex items-center gap-2">
              {/* View Toggle */}
              <div className="flex rounded-lg border border-border-default overflow-hidden" role="tablist" aria-label="Attendance view">
                <button
                  type="button"
                  role="tab"
                  aria-selected={view === 'roster'}
                  aria-controls="roster-panel"
                  onClick={() => setView('roster')}
                  className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-500 ${
                    view === 'roster' ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-1'
                  }`}
                >
                  <List className="w-4 h-4" aria-hidden="true" />
                  Roster
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={view === 'calendar'}
                  aria-controls="calendar-panel"
                  onClick={() => setView('calendar')}
                  className={`flex items-center gap-1.5 px-3 py-2 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-500 ${
                    view === 'calendar' ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-1'
                  }`}
                >
                  <Calendar className="w-4 h-4" aria-hidden="true" />
                  Calendar
                </button>
              </div>
              {/* Export */}
              <button
                type="button"
                onClick={handleExportCSV}
                className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-text-secondary bg-surface-0 border border-border-default rounded-lg hover:bg-surface-1 focus:outline-none focus:ring-2 focus:ring-brand-500"
                aria-label="Export attendance to CSV"
              >
                <Download className="w-4 h-4" aria-hidden="true" />
                Export
              </button>
            </div>
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-4 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg text-accent-danger text-sm" role="alert">
            {error}
          </div>
        )}

        {/* Calendar View */}
        {view === 'calendar' && (
          <div id="calendar-panel" role="tabpanel" aria-labelledby="calendar-tab">
            <CalendarHeatmap courseId={courseId} />
          </div>
        )}

        {/* Roster View */}
        {view === 'roster' && (
          <div id="roster-panel" role="tabpanel" aria-labelledby="roster-tab">
            {/* Date Picker + Bulk Actions */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-4">
              <div className="flex items-center gap-3">
                <label htmlFor="attendance-date" className="text-sm font-medium text-text-secondary">Date:</label>
                <input
                  id="attendance-date"
                  type="date"
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                  className="border border-border-strong rounded-lg px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                />
              </div>
              <button
                type="button"
                onClick={handleMarkAllPresent}
                disabled={saving || loading}
                className="flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-accent-success bg-accent-success/10 border border-accent-success/30 rounded-lg hover:bg-accent-success/20 focus:outline-none focus:ring-2 focus:ring-green-500 disabled:opacity-50"
              >
                <CheckCircle className="w-4 h-4" aria-hidden="true" />
                Mark All Present
              </button>
            </div>

            {/* Summary */}
            {!loading && records.length > 0 && (
              <div className="bg-surface-1 border border-border-default rounded-lg p-3 mb-4">
                <SummaryBar records={records} />
              </div>
            )}

            {/* Loading */}
            {loading && (
              <div className="flex justify-center py-12">
                <div className="h-8 w-8 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" role="status" aria-label="Loading roster" />
              </div>
            )}

            {/* Empty State */}
            {!loading && records.length === 0 && (
              <div className="text-center py-12">
                <Users className="w-12 h-12 text-gray-300 mx-auto mb-3" aria-hidden="true" />
                <p className="text-text-tertiary">No students enrolled in this course.</p>
              </div>
            )}

            {/* Student Rows */}
            {!loading && records.length > 0 && (
              <div className="space-y-2">
                {records.map((record) => (
                  <StudentRow
                    key={record.user_id}
                    record={record}
                    onStatusChange={handleStatusChange}
                    onNoteChange={handleNoteChange}
                    saving={saving}
                  />
                ))}
              </div>
            )}

            {/* Saving Indicator */}
            {saving && (
              <div className="fixed bottom-4 right-4 bg-brand-600 text-white px-4 py-2 rounded-lg shadow-lg text-sm flex items-center gap-2" role="status" aria-live="polite">
                <div className="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" aria-hidden="true" />
                Saving...
              </div>
            )}
          </div>
        )}
      </main>
    </Layout>
  );
};

export default AttendancePage;
