import React, { useMemo, useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Mail, User } from 'lucide-react';
import useIsTeacher from '../hooks/useIsTeacher';
import { analyticsApi } from '../services/api_learner_outcomes';
import { groupByWeek, formatDuration, TARDINESS_LABELS } from '../utils/courseAnalytics';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import ActivityChart from '../components/analytics/ActivityChart';
import MessageStudentsModal from '../components/analytics/MessageStudentsModal';

const STATUS_CLASS = {
  on_time: 'bg-accent-success/10 text-accent-success',
  late: 'bg-amber-100 text-amber-800',
  missing: 'bg-accent-danger/10 text-accent-danger',
  excused: 'bg-surface-2 text-text-secondary',
  floating: 'bg-surface-2 text-text-tertiary',
};

/**
 * Per-student course analytics drill-down (G154): the student's weekly
 * page views and participations, what they looked at, their recent page
 * views, and every assignment with its submission status and score.
 */
export default function StudentAnalyticsPage() {
  const { courseId, userId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const enabled = isTeacher === true;
  const [messaging, setMessaging] = useState(false);

  const activity = useQuery({ queryKey: ['analytics', courseId, 'user', userId, 'activity'], queryFn: () => analyticsApi.studentActivity(courseId, userId), enabled, retry: false });
  const assignments = useQuery({ queryKey: ['analytics', courseId, 'user', userId, 'assignments'], queryFn: () => analyticsApi.studentAssignments(courseId, userId), enabled, retry: false });
  const summaries = useQuery({ queryKey: ['analytics', courseId, 'students'], queryFn: () => analyticsApi.studentSummaries(courseId), enabled });

  const student = useMemo(
    () => (summaries.data || []).find((s) => String(s.id) === String(userId)),
    [summaries.data, userId],
  );

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;

  const notFound = activity.error?.status === 404 || assignments.error?.status === 404;
  const weeks = groupByWeek(activity.data?.by_day || []).map((w) => ({ label: w.week_start, views: w.views, participations: w.participations }));
  const totals = activity.data?.totals || {};

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}/analytics`} className="text-brand-600 hover:underline text-sm flex items-center space-x-1">
          <ArrowLeft className="w-3.5 h-3.5" aria-hidden="true" />
          <span>Back to Course Analytics</span>
        </Link>
        <div className="flex flex-wrap items-center justify-between gap-3 mt-2">
          <div className="flex items-center gap-3">
            <User className="w-7 h-7 text-indigo-600" aria-hidden="true" />
            <h2 className="text-2xl font-bold text-text-primary">{student?.name || 'Student analytics'}</h2>
          </div>
          {student && (
            <button type="button" onClick={() => setMessaging(true)} className="inline-flex items-center gap-2 px-4 py-2 text-sm rounded border border-border-strong hover:bg-surface-1">
              <Mail className="w-4 h-4" aria-hidden="true" /> Message student
            </button>
          )}
        </div>
      </div>

      {notFound ? (
        <div className="rounded-md border border-border-default bg-surface-0 p-6 text-text-secondary">This user is not a student in this course.</div>
      ) : activity.isLoading || assignments.isLoading || isTeacher === null ? (
        <div className="py-12 text-center text-text-tertiary" role="status">Loading…</div>
      ) : activity.isError || assignments.isError ? (
        <div className="rounded-md bg-accent-danger/10 border border-accent-danger/30 px-4 py-3 text-sm text-accent-danger">
          {(activity.error || assignments.error)?.message || 'Could not load analytics.'}
        </div>
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            {[
              ['Page views', totals.page_views ?? 0],
              ['Participations', totals.participations ?? 0],
              ['Time in course', formatDuration(totals.interaction_seconds)],
              ['Last activity', student?.last_activity_at ? new Date(student.last_activity_at).toLocaleDateString() : 'Never'],
            ].map(([label, value]) => (
              <div key={label} className="bg-surface-0 rounded-lg shadow p-4">
                <p className="text-xs font-medium text-text-tertiary uppercase">{label}</p>
                <p className="text-xl font-bold text-text-primary">{value}</p>
              </div>
            ))}
          </div>

          <section className="bg-surface-0 rounded-lg shadow p-6" aria-labelledby="sa-activity">
            <h3 id="sa-activity" className="text-lg font-semibold text-text-primary mb-4">Weekly online activity</h3>
            {weeks.length === 0 ? <p className="text-sm text-text-tertiary">No activity recorded yet.</p> : <ActivityChart rows={weeks} title="Student weekly activity" />}
          </section>

          <section className="bg-surface-0 rounded-lg shadow overflow-x-auto" aria-labelledby="sa-assignments">
            <h3 id="sa-assignments" className="text-lg font-semibold text-text-primary px-6 pt-5 pb-3">Assignments</h3>
            <table className="min-w-full text-sm">
              <thead className="bg-surface-1 text-xs uppercase text-text-secondary">
                <tr>
                  <th scope="col" className="px-4 py-2 text-left">Assignment</th>
                  <th scope="col" className="px-4 py-2 text-left">Due</th>
                  <th scope="col" className="px-4 py-2 text-left">Submitted</th>
                  <th scope="col" className="px-4 py-2 text-left">Status</th>
                  <th scope="col" className="px-4 py-2 text-right">Score</th>
                </tr>
              </thead>
              <tbody>
                {(assignments.data || []).map((a) => (
                  <tr key={a.assignment_id} className="border-t border-border-subtle">
                    <td className="px-4 py-2 font-medium text-text-primary">{a.title}</td>
                    <td className="px-4 py-2 text-text-secondary">{a.due_at ? new Date(a.due_at).toLocaleDateString() : '—'}</td>
                    <td className="px-4 py-2 text-text-secondary">{a.submission?.submitted_at ? new Date(a.submission.submitted_at).toLocaleString() : '—'}</td>
                    <td className="px-4 py-2">
                      <span className={`inline-block px-2 py-0.5 rounded-full text-xs ${STATUS_CLASS[a.status] || ''}`}>{TARDINESS_LABELS[a.status] || a.status}</span>
                    </td>
                    <td className="px-4 py-2 text-right">
                      {a.submission?.score != null ? `${a.submission.score} / ${a.points_possible ?? '—'}` : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>

          <section className="bg-surface-0 rounded-lg shadow p-6" aria-labelledby="sa-recent">
            <h3 id="sa-recent" className="text-lg font-semibold text-text-primary mb-3">Recent page views</h3>
            {(activity.data?.recent_page_views || []).length === 0 ? (
              <p className="text-sm text-text-tertiary">None yet.</p>
            ) : (
              <ul className="divide-y divide-border-subtle text-sm">
                {activity.data.recent_page_views.map((pv) => (
                  <li key={pv.id} className="py-2 flex flex-wrap items-center justify-between gap-2">
                    <span className="text-text-primary">
                      {pv.participated ? <span className="mr-2 text-xs px-1.5 py-0.5 rounded bg-amber-100 text-amber-800">participated</span> : null}
                      <span className="capitalize">{(pv.asset_category || 'other').replace('_', ' ')}</span>
                      <span className="text-text-tertiary"> · {pv.url}</span>
                    </span>
                    <span className="text-xs text-text-tertiary">
                      {new Date(pv.created_at).toLocaleString()}{pv.interaction_seconds ? ` · ${formatDuration(pv.interaction_seconds)}` : ''}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      )}

      {messaging && student && (
        <MessageStudentsModal students={[student]} assignments={[]} initialStudentIds={[String(student.id)]} onClose={() => setMessaging(false)} />
      )}
    </Layout>
  );
}
