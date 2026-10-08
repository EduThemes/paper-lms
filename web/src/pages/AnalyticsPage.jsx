import React, { useMemo, useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, BarChart3, Users, FileText, Activity, Mail } from 'lucide-react';
import useIsTeacher from '../hooks/useIsTeacher';
import { useCourse } from '../services/apiQueries';
import { analyticsApi } from '../services/api_learner_outcomes';
import { groupByWeek, fillDays, formatDuration, TARDINESS_LABELS } from '../utils/courseAnalytics';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import ActivityChart from '../components/analytics/ActivityChart';
import MessageStudentsModal from '../components/analytics/MessageStudentsModal';

const TAB_ACTIVITY = 'activity';
const TAB_ASSIGNMENTS = 'assignments';
const TAB_STUDENTS = 'students';

const Spinner = ({ label }) => (
  <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary" role="status">
    <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
    {label}
  </div>
);

const ErrorNote = ({ query }) => (
  <div className="rounded-md bg-accent-danger/10 border border-accent-danger/30 px-4 py-3 text-sm text-accent-danger flex items-center justify-between">
    <span>{query.error?.message || 'Could not load analytics.'}</span>
    <button type="button" onClick={() => query.refetch()} className="underline">Try again</button>
  </div>
);

const Stat = ({ label, value }) => (
  <div className="bg-surface-0 rounded-lg shadow p-4">
    <p className="text-xs font-medium text-text-tertiary uppercase">{label}</p>
    <p className="text-2xl font-bold text-text-primary">{value}</p>
  </div>
);

const pct = (v, pp) => (pp ? Math.max(0, Math.min(100, (v / pp) * 100)) : 0);

function ActivityTab({ courseId }) {
  const [granularity, setGranularity] = useState('week');
  const activity = useQuery({ queryKey: ['analytics', courseId, 'activity'], queryFn: () => analyticsApi.courseActivity(courseId) });
  const categories = useQuery({ queryKey: ['analytics', courseId, 'categories'], queryFn: () => analyticsApi.courseCategories(courseId) });

  if (activity.isLoading) return <Spinner label="Loading activity…" />;
  if (activity.isError) return <ErrorNote query={activity} />;
  const days = activity.data || [];
  if (days.length === 0) {
    return (
      <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
        <Activity className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
        <p className="text-text-tertiary">No student activity recorded yet.</p>
        <p className="text-text-disabled text-sm mt-1">Page views and participations appear here as students use the course.</p>
      </div>
    );
  }
  const totalViews = days.reduce((s, d) => s + (d.views || 0), 0);
  const totalParts = days.reduce((s, d) => s + (d.participations || 0), 0);
  const rows = granularity === 'week'
    ? groupByWeek(days).map((w) => ({ label: w.week_start, views: w.views, participations: w.participations }))
    : fillDays(days).map((d) => ({ label: d.date, views: d.views, participations: d.participations }));
  const cats = categories.data || [];
  const maxCat = Math.max(1, ...cats.map((c) => (c.views || 0) + (c.participations || 0)));

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Stat label="Page views" value={totalViews} />
        <Stat label="Participations" value={totalParts} />
        <Stat label="Active days" value={days.length} />
      </div>
      <div className="bg-surface-0 rounded-lg shadow p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-text-primary">{granularity === 'week' ? 'Weekly online activity' : 'Daily online activity'}</h3>
          <div className="inline-flex rounded border border-border-strong overflow-hidden text-sm" role="group" aria-label="Group activity by">
            {['week', 'day'].map((g) => (
              <button key={g} type="button" aria-pressed={granularity === g} onClick={() => setGranularity(g)}
                className={`px-3 py-1 ${granularity === g ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-1'}`}>
                {g === 'week' ? 'By week' : 'By day'}
              </button>
            ))}
          </div>
        </div>
        <ActivityChart rows={rows} title="Course activity" />
      </div>
      {cats.length > 0 && (
        <div className="bg-surface-0 rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold text-text-primary mb-4">Activity by content</h3>
          <ul className="space-y-2">
            {cats.map((c) => (
              <li key={c.category} className="flex items-center gap-3">
                <span className="w-32 text-sm text-text-secondary capitalize">{c.category.replace('_', ' ')}</span>
                <div className="flex-1 bg-surface-2 rounded-full h-4 overflow-hidden" aria-hidden="true">
                  <div className="bg-indigo-500 h-full" style={{ width: `${(((c.views || 0) + (c.participations || 0)) / maxCat) * 100}%` }} />
                </div>
                <span className="w-40 text-right text-xs text-text-tertiary">{c.views} views · {c.participations} participations</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function AssignmentsTab({ query }) {
  if (query.isLoading) return <Spinner label="Loading assignments…" />;
  if (query.isError) return <ErrorNote query={query} />;
  const stats = query.data || [];
  if (stats.length === 0) {
    return (
      <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
        <FileText className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
        <p className="text-text-tertiary">No published assignments yet.</p>
      </div>
    );
  }
  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead className="bg-surface-1 text-xs uppercase text-text-secondary">
          <tr>
            <th scope="col" className="px-4 py-2 text-left">Assignment</th>
            <th scope="col" className="px-4 py-2 text-left">Score distribution</th>
            <th scope="col" className="px-4 py-2 text-right">On time</th>
            <th scope="col" className="px-4 py-2 text-right">Late</th>
            <th scope="col" className="px-4 py-2 text-right">Missing</th>
          </tr>
        </thead>
        <tbody>
          {stats.map((s) => {
            const pp = s.points_possible || 0;
            const t = s.tardiness || {};
            return (
              <tr key={s.assignment_id} className="border-t border-border-subtle">
                <td className="px-4 py-3">
                  <div className="font-medium text-text-primary">{s.title}</div>
                  <div className="text-xs text-text-tertiary">{pp} pts{s.due_at ? ` · due ${new Date(s.due_at).toLocaleDateString()}` : ''}</div>
                </td>
                <td className="px-4 py-3 min-w-[220px]">
                  {s.median == null ? (
                    <span className="text-xs text-text-disabled">No posted scores</span>
                  ) : (
                    <div>
                      <div className="relative h-4 bg-surface-2 rounded" aria-hidden="true">
                        <div className="absolute top-1.5 h-1 bg-text-disabled" style={{ left: `${pct(s.min_score, pp)}%`, width: `${pct(s.max_score, pp) - pct(s.min_score, pp)}%` }} />
                        <div className="absolute top-0 h-4 bg-indigo-300 rounded-sm" style={{ left: `${pct(s.first_quartile, pp)}%`, width: `${Math.max(pct(s.third_quartile, pp) - pct(s.first_quartile, pp), 0.5)}%` }} />
                        <div className="absolute top-0 h-4 w-0.5 bg-indigo-800" style={{ left: `${pct(s.median, pp)}%` }} />
                      </div>
                      <div className="text-xs text-text-tertiary mt-1">
                        min {s.min_score} · median {s.median} · max {s.max_score} · avg {s.avg_score}
                      </div>
                    </div>
                  )}
                </td>
                <td className="px-4 py-3 text-right">{t.on_time ?? 0}</td>
                <td className="px-4 py-3 text-right">{t.late ?? 0}</td>
                <td className="px-4 py-3 text-right">{t.missing ?? 0}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function StudentsTab({ courseId, query, onMessage }) {
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState({});
  const students = useMemo(() => query.data || [], [query.data]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return q ? students.filter((s) => (s.name || '').toLowerCase().includes(q)) : students;
  }, [students, search]);

  if (query.isLoading) return <Spinner label="Loading students…" />;
  if (query.isError) return <ErrorNote query={query} />;
  if (students.length === 0) {
    return (
      <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
        <Users className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
        <p className="text-text-tertiary">No students enrolled.</p>
      </div>
    );
  }
  const selectedIds = Object.keys(selected).filter((k) => selected[k]);
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <input type="search" placeholder="Search students…" aria-label="Search students" value={search} onChange={(e) => setSearch(e.target.value)}
          className="w-full max-w-xs rounded border border-border-strong px-3 py-1.5 text-sm" />
        <button type="button" disabled={selectedIds.length === 0} onClick={() => onMessage(selectedIds)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded border border-border-strong hover:bg-surface-1 disabled:opacity-50">
          <Mail className="w-4 h-4" aria-hidden="true" /> Message selected ({selectedIds.length})
        </button>
      </div>
      <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="bg-surface-1 text-xs uppercase text-text-secondary">
            <tr>
              <th scope="col" className="px-3 py-2"><span className="sr-only">Select</span></th>
              <th scope="col" className="px-3 py-2 text-left">Student</th>
              <th scope="col" className="px-3 py-2 text-right">Page views</th>
              <th scope="col" className="px-3 py-2 text-right">Participations</th>
              <th scope="col" className="px-3 py-2 text-right">Time</th>
              <th scope="col" className="px-3 py-2 text-left">Last activity</th>
              <th scope="col" className="px-3 py-2 text-left">Submissions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((s) => {
              const t = s.tardiness_breakdown || {};
              return (
                <tr key={s.id} className="border-t border-border-subtle hover:bg-surface-1">
                  <td className="px-3 py-2 text-center">
                    <input type="checkbox" aria-label={`Select ${s.name}`} checked={!!selected[s.id]} onChange={() => setSelected((p) => ({ ...p, [s.id]: !p[s.id] }))} />
                  </td>
                  <td className="px-3 py-2">
                    <Link to={`/courses/${courseId}/analytics/users/${s.id}`} className="text-brand-600 hover:underline font-medium">{s.name || `Student ${s.id}`}</Link>
                  </td>
                  <td className="px-3 py-2 text-right">{s.page_views}</td>
                  <td className="px-3 py-2 text-right">{s.participations}</td>
                  <td className="px-3 py-2 text-right">{formatDuration(s.interaction_seconds)}</td>
                  <td className="px-3 py-2 text-text-secondary">{s.last_activity_at ? new Date(s.last_activity_at).toLocaleString() : 'Never'}</td>
                  <td className="px-3 py-2">
                    <span className="text-xs text-text-secondary">
                      {TARDINESS_LABELS.on_time} {t.on_time || 0} · {TARDINESS_LABELS.late} {t.late || 0} · <span className={t.missing ? 'text-accent-danger font-medium' : ''}>{TARDINESS_LABELS.missing} {t.missing || 0}</span>
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

const AnalyticsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [activeTab, setActiveTab] = useState(TAB_ACTIVITY);
  const [messaging, setMessaging] = useState(null); // null | { ids?: string[] }
  const enabled = isTeacher === true;
  const course = useCourse(courseId, { enabled });
  const assignments = useQuery({ queryKey: ['analytics', courseId, 'assignments'], queryFn: () => analyticsApi.courseAssignments(courseId), enabled });
  const students = useQuery({ queryKey: ['analytics', courseId, 'students'], queryFn: () => analyticsApi.studentSummaries(courseId), enabled });

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><Spinner label="Loading…" /></Layout>;

  const tabs = [
    { id: TAB_ACTIVITY, label: 'Activity', icon: Activity },
    { id: TAB_ASSIGNMENTS, label: 'Assignments', icon: FileText },
    { id: TAB_STUDENTS, label: 'Students', icon: Users },
  ];

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center space-x-1">
          <ArrowLeft className="w-3.5 h-3.5" aria-hidden="true" />
          <span>Back to Course</span>
        </Link>
        <div className="flex flex-wrap items-center justify-between gap-3 mt-2">
          <div className="flex items-center space-x-3">
            <BarChart3 className="w-7 h-7 text-indigo-600" aria-hidden="true" />
            <div>
              <h2 className="text-2xl font-bold text-text-primary">Course Analytics</h2>
              {course.data && <p className="text-text-tertiary text-sm">{course.data.name}</p>}
            </div>
          </div>
          <button type="button" onClick={() => setMessaging({})} disabled={!students.data?.length}
            className="inline-flex items-center gap-2 px-4 py-2 text-sm rounded bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50">
            <Mail className="w-4 h-4" aria-hidden="true" /> Message students who…
          </button>
        </div>
      </div>

      <div className="border-b border-border-default mb-6">
        <nav className="flex space-x-6" role="tablist" aria-label="Analytics sections">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button key={tab.id} type="button" role="tab" aria-selected={activeTab === tab.id} onClick={() => setActiveTab(tab.id)}
                className={`flex items-center space-x-2 py-3 px-1 border-b-2 text-sm font-medium transition-colors ${
                  activeTab === tab.id ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
                }`}>
                <Icon className="w-4 h-4" aria-hidden="true" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
      </div>

      {activeTab === TAB_ACTIVITY && <ActivityTab courseId={courseId} />}
      {activeTab === TAB_ASSIGNMENTS && <AssignmentsTab query={assignments} />}
      {activeTab === TAB_STUDENTS && <StudentsTab courseId={courseId} query={students} onMessage={(ids) => setMessaging({ ids })} />}

      {messaging && (
        <MessageStudentsModal
          students={students.data || []}
          assignments={assignments.data || []}
          initialStudentIds={messaging.ids}
          onClose={() => setMessaging(null)}
        />
      )}
    </Layout>
  );
};

export default AnalyticsPage;
