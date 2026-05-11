import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import {
  ArrowLeft,
  BarChart3,
  Users,
  FileText,
  Activity,
  Clock,
  Eye,
  CheckSquare,
  TrendingUp,
  TrendingDown,
  Minus,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const TAB_ACTIVITY = 'activity';
const TAB_ASSIGNMENTS = 'assignments';
const TAB_STUDENTS = 'students';

const AnalyticsPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [course, setCourse] = useState(null);
  const [activeTab, setActiveTab] = useState(TAB_ACTIVITY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Activity data
  const [activityData, setActivityData] = useState([]);
  const [activityLoading, setActivityLoading] = useState(false);

  // Assignment stats
  const [assignmentStats, setAssignmentStats] = useState([]);
  const [assignmentLoading, setAssignmentLoading] = useState(false);

  // Student summaries
  const [studentSummaries, setStudentSummaries] = useState([]);
  const [studentLoading, setStudentLoading] = useState(false);

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

  const fetchActivity = useCallback(async () => {
    setActivityLoading(true);
    try {
      const result = await api.request(`/courses/${courseId}/analytics/activity`);
      setActivityData(result.data || []);
    } catch (err) {
      setActivityData([]);
    } finally {
      setActivityLoading(false);
    }
  }, [courseId]);

  const fetchAssignments = useCallback(async () => {
    setAssignmentLoading(true);
    try {
      const result = await api.request(`/courses/${courseId}/analytics/assignments`);
      setAssignmentStats(result.data || []);
    } catch (err) {
      setAssignmentStats([]);
    } finally {
      setAssignmentLoading(false);
    }
  }, [courseId]);

  const fetchStudents = useCallback(async () => {
    setStudentLoading(true);
    try {
      const result = await api.request(`/courses/${courseId}/analytics/student_summaries?per_page=100`);
      setStudentSummaries(result.data || []);
    } catch (err) {
      setStudentSummaries([]);
    } finally {
      setStudentLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    if (activeTab === TAB_ACTIVITY) fetchActivity();
    if (activeTab === TAB_ASSIGNMENTS) fetchAssignments();
    if (activeTab === TAB_STUDENTS) fetchStudents();
  }, [activeTab, fetchActivity, fetchAssignments, fetchStudents]);

  const tabs = [
    { id: TAB_ACTIVITY, label: 'Activity', icon: Activity },
    { id: TAB_ASSIGNMENTS, label: 'Assignments', icon: FileText },
    { id: TAB_STUDENTS, label: 'Students', icon: Users },
  ];

  // Find max value for bar chart scaling
  const maxActivityViews = Math.max(...activityData.map((d) => d.views || 0), 1);
  const maxAssignmentScore = Math.max(
    ...assignmentStats.map((s) => {
      const pp = s.points_possible || 0;
      const mx = s.max_score || 0;
      return Math.max(pp, mx);
    }),
    1
  );

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
  Loading analytics...
</div>
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center space-x-1">
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>Back to Course</span>
        </Link>
        <div className="flex items-center space-x-3 mt-2">
          <BarChart3 className="w-7 h-7 text-indigo-600" />
          <div>
            <h2 className="text-2xl font-bold text-text-primary">Course Analytics</h2>
            {course && <p className="text-text-tertiary text-sm">{course.name}</p>}
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-border-default mb-6">
        <nav className="flex space-x-6">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center space-x-2 py-3 px-1 border-b-2 text-sm font-medium transition-colors ${
                  activeTab === tab.id
                    ? 'border-indigo-600 text-indigo-600'
                    : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{tab.label}</span>
              </button>
            );
          })}
        </nav>
      </div>

      {/* Activity Tab */}
      {activeTab === TAB_ACTIVITY && (
        <div>
          {activityLoading ? (
            <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading activity data...
</div>
          ) : activityData.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <Activity className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary">No activity data recorded yet.</p>
              <p className="text-text-disabled text-sm mt-1">Page views will appear here as students interact with the course.</p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow p-6">
              <h3 className="text-lg font-semibold text-text-primary mb-4 flex items-center space-x-2">
                <Eye className="w-5 h-5 text-indigo-500" />
                <span>Page Views by Date</span>
              </h3>

              {/* Summary Stats */}
              <div className="grid grid-cols-3 gap-4 mb-6">
                <div className="bg-indigo-50 rounded-lg p-4">
                  <p className="text-xs font-medium text-indigo-600 uppercase">Total Views</p>
                  <p className="text-2xl font-bold text-indigo-900">
                    {activityData.reduce((sum, d) => sum + (d.views || 0), 0)}
                  </p>
                </div>
                <div className="bg-accent-success/10 rounded-lg p-4">
                  <p className="text-xs font-medium text-accent-success uppercase">Active Days</p>
                  <p className="text-2xl font-bold text-accent-success">{activityData.length}</p>
                </div>
                <div className="bg-purple-50 rounded-lg p-4">
                  <p className="text-xs font-medium text-purple-600 uppercase">Avg Views/Day</p>
                  <p className="text-2xl font-bold text-purple-900">
                    {activityData.length > 0
                      ? Math.round(
                          activityData.reduce((sum, d) => sum + (d.views || 0), 0) / activityData.length
                        )
                      : 0}
                  </p>
                </div>
              </div>

              {/* Bar Chart */}
              <div className="space-y-2">
                {activityData.map((day, idx) => {
                  const widthPct = ((day.views || 0) / maxActivityViews) * 100;
                  return (
                    <div key={idx} className="flex items-center space-x-3">
                      <span className="text-xs text-text-tertiary w-24 text-right flex-shrink-0">
                        {day.date}
                      </span>
                      <div className="flex-1 bg-surface-2 rounded-full h-6 relative overflow-hidden">
                        <div
                          className="bg-indigo-500 h-full rounded-full transition-all duration-300"
                          style={{ width: `${Math.max(widthPct, 1)}%` }}
                        />
                      </div>
                      <span className="text-sm font-medium text-text-secondary w-12 text-right flex-shrink-0">
                        {day.views}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Assignments Tab */}
      {activeTab === TAB_ASSIGNMENTS && (
        <div>
          {assignmentLoading ? (
            <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading assignment data...
</div>
          ) : assignmentStats.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <FileText className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary">No assignment data available yet.</p>
              <p className="text-text-disabled text-sm mt-1">Statistics will appear once assignments are created and graded.</p>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Summary Row */}
              <div className="grid grid-cols-4 gap-4">
                <div className="bg-surface-0 rounded-lg shadow p-4">
                  <p className="text-xs font-medium text-text-tertiary uppercase">Assignments</p>
                  <p className="text-2xl font-bold text-text-primary">{assignmentStats.length}</p>
                </div>
                <div className="bg-surface-0 rounded-lg shadow p-4">
                  <p className="text-xs font-medium text-text-tertiary uppercase">Total Submissions</p>
                  <p className="text-2xl font-bold text-text-primary">
                    {assignmentStats.reduce((sum, s) => sum + (s.submission_count || 0), 0)}
                  </p>
                </div>
                <div className="bg-surface-0 rounded-lg shadow p-4">
                  <p className="text-xs font-medium text-text-tertiary uppercase">Avg Score (all)</p>
                  <p className="text-2xl font-bold text-text-primary">
                    {(() => {
                      const scored = assignmentStats.filter((s) => s.avg_score !== null && s.avg_score !== undefined);
                      if (scored.length === 0) return '-';
                      const avg = scored.reduce((sum, s) => sum + s.avg_score, 0) / scored.length;
                      return avg.toFixed(1);
                    })()}
                  </p>
                </div>
                <div className="bg-surface-0 rounded-lg shadow p-4">
                  <p className="text-xs font-medium text-text-tertiary uppercase">Highest Avg</p>
                  <p className="text-2xl font-bold text-accent-success">
                    {(() => {
                      const scored = assignmentStats.filter((s) => s.avg_score !== null && s.avg_score !== undefined);
                      if (scored.length === 0) return '-';
                      return Math.max(...scored.map((s) => s.avg_score)).toFixed(1);
                    })()}
                  </p>
                </div>
              </div>

              {/* Per-assignment bars */}
              <div className="bg-surface-0 rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold text-text-primary mb-4">Score Distribution by Assignment</h3>
                <div className="space-y-4">
                  {assignmentStats.map((stat) => {
                    const pp = stat.points_possible || 0;
                    const minPct = stat.min_score != null ? (stat.min_score / Math.max(pp, 1)) * 100 : 0;
                    const maxPct = stat.max_score != null ? (stat.max_score / Math.max(pp, 1)) * 100 : 0;
                    const avgPct = stat.avg_score != null ? (stat.avg_score / Math.max(pp, 1)) * 100 : 0;

                    return (
                      <div key={stat.assignment_id} className="border-b border-border-subtle pb-4 last:border-0 last:pb-0">
                        <div className="flex items-center justify-between mb-2">
                          <div>
                            <p className="text-sm font-medium text-text-primary">{stat.title}</p>
                            <p className="text-xs text-text-disabled">
                              {pp} pts possible | {stat.submission_count || 0} submissions
                            </p>
                          </div>
                          <div className="flex items-center space-x-4 text-xs">
                            {stat.min_score != null && (
                              <span className="flex items-center space-x-1 text-accent-danger">
                                <TrendingDown className="w-3 h-3" />
                                <span>Min: {stat.min_score}</span>
                              </span>
                            )}
                            {stat.avg_score != null && (
                              <span className="flex items-center space-x-1 text-brand-600">
                                <Minus className="w-3 h-3" />
                                <span>Avg: {stat.avg_score}</span>
                              </span>
                            )}
                            {stat.max_score != null && (
                              <span className="flex items-center space-x-1 text-accent-success">
                                <TrendingUp className="w-3 h-3" />
                                <span>Max: {stat.max_score}</span>
                              </span>
                            )}
                          </div>
                        </div>

                        {/* Stacked bar showing min/avg/max */}
                        <div className="relative bg-surface-2 rounded-full h-5 overflow-hidden">
                          {stat.max_score != null && (
                            <div
                              className="absolute top-0 left-0 h-full bg-accent-success/30 rounded-full"
                              style={{ width: `${Math.min(maxPct, 100)}%` }}
                            />
                          )}
                          {stat.avg_score != null && (
                            <div
                              className="absolute top-0 left-0 h-full bg-brand-500 rounded-full"
                              style={{ width: `${Math.min(avgPct, 100)}%` }}
                            />
                          )}
                          {stat.min_score != null && (
                            <div
                              className="absolute top-0 left-0 h-full bg-accent-danger rounded-full"
                              style={{ width: `${Math.min(minPct, 100)}%` }}
                            />
                          )}
                        </div>

                        {/* Scale labels */}
                        <div className="flex justify-between mt-1">
                          <span className="text-xs text-text-disabled">0</span>
                          <span className="text-xs text-text-disabled">{pp}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>

                {/* Legend */}
                <div className="flex items-center space-x-6 mt-4 pt-4 border-t border-border-subtle">
                  <div className="flex items-center space-x-2 text-xs text-text-tertiary">
                    <div className="w-3 h-3 rounded bg-accent-danger" />
                    <span>Min Score</span>
                  </div>
                  <div className="flex items-center space-x-2 text-xs text-text-tertiary">
                    <div className="w-3 h-3 rounded bg-brand-500" />
                    <span>Avg Score</span>
                  </div>
                  <div className="flex items-center space-x-2 text-xs text-text-tertiary">
                    <div className="w-3 h-3 rounded bg-accent-success/30" />
                    <span>Max Score</span>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Students Tab */}
      {activeTab === TAB_STUDENTS && (
        <div>
          {studentLoading ? (
            <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading student data...
</div>
          ) : studentSummaries.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <Users className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary">No student data available yet.</p>
              <p className="text-text-disabled text-sm mt-1">Enroll students to see their analytics summaries.</p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
              <div className="px-6 py-4 border-b bg-surface-1">
                <h3 className="text-lg font-semibold text-text-primary">Student Summaries</h3>
                <p className="text-sm text-text-tertiary mt-1">{studentSummaries.length} {studentSummaries.length === 1 ? 'student' : 'students'}</p>
              </div>

              <div className="overflow-x-auto">
                <table className="min-w-full border-collapse">
                  <thead>
                    <tr className="bg-surface-1">
                      <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">
                        Student
                      </th>
                      <th className="px-6 py-3 text-center text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">
                        Page Views
                      </th>
                      <th className="px-6 py-3 text-center text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">
                        Time on Site
                      </th>
                      <th className="px-6 py-3 text-center text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">
                        Submissions
                      </th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider border-b">
                        Engagement
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-default">
                    {studentSummaries.map((student) => {
                      const maxViews = Math.max(...studentSummaries.map((s) => s.page_views || 0), 1);
                      const viewsPct = ((student.page_views || 0) / maxViews) * 100;
                      const hours = Math.floor((student.interaction_seconds || 0) / 3600);
                      const minutes = Math.floor(((student.interaction_seconds || 0) % 3600) / 60);

                      return (
                        <tr key={student.id} className="hover:bg-surface-1">
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="flex items-center">
                              <div className="w-8 h-8 bg-indigo-100 rounded-full flex items-center justify-center text-indigo-600 font-semibold text-sm flex-shrink-0">
                                {(student.name || '?').charAt(0).toUpperCase()}
                              </div>
                              <div className="ml-3">
                                <p className="text-sm font-medium text-text-primary">
                                  {student.name || `User ${student.id}`}
                                </p>
                              </div>
                            </div>
                          </td>
                          <td className="px-6 py-4 text-center whitespace-nowrap">
                            <div className="flex items-center justify-center space-x-2">
                              <Eye className="w-3.5 h-3.5 text-text-disabled" />
                              <span className="text-sm font-medium text-text-primary">
                                {student.page_views || 0}
                              </span>
                            </div>
                          </td>
                          <td className="px-6 py-4 text-center whitespace-nowrap">
                            <div className="flex items-center justify-center space-x-2">
                              <Clock className="w-3.5 h-3.5 text-text-disabled" />
                              <span className="text-sm text-text-secondary">
                                {hours > 0 ? `${hours}h ` : ''}{minutes}m
                              </span>
                            </div>
                          </td>
                          <td className="px-6 py-4 text-center whitespace-nowrap">
                            <div className="flex items-center justify-center space-x-2">
                              <CheckSquare className="w-3.5 h-3.5 text-text-disabled" />
                              <span className="text-sm font-medium text-text-primary">
                                {student.submissions || 0}
                              </span>
                            </div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="w-full bg-surface-2 rounded-full h-3 max-w-[200px]">
                              <div
                                className={`h-full rounded-full transition-all duration-300 ${
                                  viewsPct > 66
                                    ? 'bg-accent-success'
                                    : viewsPct > 33
                                    ? 'bg-accent-warning'
                                    : 'bg-accent-danger'
                                }`}
                                style={{ width: `${Math.max(viewsPct, 2)}%` }}
                              />
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}
    </Layout>
  );
};

export default AnalyticsPage;
