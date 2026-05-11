import React, { useState, useEffect, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { BookOpen, Calendar, CheckCircle, Clock, AlertTriangle, Star, Filter, ChevronDown } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { useAuth } from '../contexts/AuthContext';
import { sanitizeHTML } from '../components/RichContentViewer';

const GROUP_COLORS = {
  '#3b82f6': { bg: 'bg-brand-100', text: 'text-brand-800', border: 'border-brand-300', dot: 'bg-brand-500' },
  '#10b981': { bg: 'bg-accent-success/20', text: 'text-accent-success', border: 'border-accent-success/40', dot: 'bg-accent-success' },
  '#f59e0b': { bg: 'bg-accent-warning/20', text: 'text-accent-warning', border: 'border-accent-warning/40', dot: 'bg-accent-warning' },
  '#f43f5e': { bg: 'bg-rose-100', text: 'text-rose-800', border: 'border-rose-300', dot: 'bg-rose-500' },
  '#8b5cf6': { bg: 'bg-purple-100', text: 'text-purple-800', border: 'border-purple-300', dot: 'bg-purple-500' },
  '#14b8a6': { bg: 'bg-teal-100', text: 'text-teal-800', border: 'border-teal-300', dot: 'bg-teal-500' },
  '#f97316': { bg: 'bg-orange-100', text: 'text-orange-800', border: 'border-orange-300', dot: 'bg-orange-500' },
  '#6366f1': { bg: 'bg-indigo-100', text: 'text-indigo-800', border: 'border-indigo-300', dot: 'bg-indigo-500' },
};

const STATUS_CONFIG = {
  submitted: { icon: CheckCircle, label: 'Submitted', className: 'text-accent-success' },
  graded: { icon: Star, label: 'Graded', className: 'text-accent-warning' },
  upcoming: { icon: Clock, label: 'Upcoming', className: 'text-brand-500' },
  missing: { icon: AlertTriangle, label: 'Overdue', className: 'text-accent-danger' },
};

function getColorClasses(hex) {
  return GROUP_COLORS[hex] || { bg: 'bg-surface-2', text: 'text-text-primary', border: 'border-border-strong', dot: 'bg-text-tertiary' };
}

function formatDate(dateStr) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  return d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
}

function formatTime(dateStr) {
  if (!dateStr) return null;
  const d = new Date(dateStr);
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

function formatDateRange(startAt, endAt) {
  if (!startAt && !endAt) return 'Dates not set';
  const parts = [];
  if (startAt) parts.push(new Date(startAt).toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' }));
  if (endAt) parts.push(new Date(endAt).toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' }));
  return parts.join(' \u2013 ');
}

function getWeekKey(dateStr) {
  if (!dateStr) return 'No Date';
  const d = new Date(dateStr);
  const startOfWeek = new Date(d);
  startOfWeek.setDate(d.getDate() - d.getDay());
  return startOfWeek.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function getWeekLabel(dateStr) {
  if (!dateStr) return 'No Date Assigned';
  const d = new Date(dateStr);
  const startOfWeek = new Date(d);
  startOfWeek.setDate(d.getDate() - d.getDay());
  const endOfWeek = new Date(startOfWeek);
  endOfWeek.setDate(startOfWeek.getDate() + 6);
  const startLabel = startOfWeek.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  const endLabel = endOfWeek.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  return 'Week of ' + startLabel + ' \u2013 ' + endLabel;
}

function getItemDate(item) {
  return item.due_at || item.start_at || null;
}

function StatusBadge({ status }) {
  const config = STATUS_CONFIG[status];
  if (!config) return null;
  const Icon = config.icon;
  return (
    <span className={'inline-flex items-center gap-1 text-xs font-medium ' + config.className} title={config.label}>
      <Icon className="w-3.5 h-3.5" />
      {config.label}
    </span>
  );
}

function GradingBar({ breakdown }) {
  const totalWeight = breakdown.reduce((sum, g) => sum + g.group_weight, 0);
  const remaining = Math.max(0, 100 - totalWeight);

  return (
    <div className="space-y-3">
      <div className="flex h-8 w-full rounded-lg overflow-hidden shadow-inner bg-surface-2" role="img" aria-label="Grading weight distribution">
        {breakdown.map((group, idx) => {
          if (group.group_weight <= 0) return null;
          const widthPct = group.group_weight;
          return (
            <div
              key={idx}
              className="h-full flex items-center justify-center text-xs font-semibold text-white transition-all duration-300 hover:opacity-90"
              style={{ width: widthPct + '%', backgroundColor: group.group_color, minWidth: widthPct > 3 ? undefined : '12px' }}
              title={group.group_name + ': ' + group.group_weight + '%'}
            >
              {widthPct >= 8 ? widthPct + '%' : ''}
            </div>
          );
        })}
        {remaining > 0 && (
          <div
            className="h-full flex items-center justify-center text-xs font-medium text-text-disabled bg-border-default"
            style={{ width: remaining + '%' }}
            title={'Unassigned: ' + remaining.toFixed(1) + '%'}
          >
            {remaining >= 8 ? remaining.toFixed(0) + '%' : ''}
          </div>
        )}
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
        {breakdown.map((group, idx) => {
          const colors = getColorClasses(group.group_color);
          return (
            <div key={idx} className={'rounded-lg border px-3 py-2 ' + colors.bg + ' ' + colors.border}>
              <div className="flex items-center gap-2">
                <span className={'w-2.5 h-2.5 rounded-full flex-shrink-0 ' + colors.dot}></span>
                <span className={'text-sm font-medium truncate ' + colors.text}>{group.group_name}</span>
              </div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className={'text-lg font-bold ' + colors.text}>{group.group_weight}%</span>
                <span className="text-xs text-text-tertiary">
                  {group.assignment_count} {group.assignment_count === 1 ? 'assignment' : 'assignments'}
                </span>
              </div>
            </div>
          );
        })}
        {remaining > 0 && (
          <div className="rounded-lg border px-3 py-2 bg-surface-1 border-border-default">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full flex-shrink-0 bg-border-strong"></span>
              <span className="text-sm font-medium text-text-tertiary">Unassigned</span>
            </div>
            <div className="mt-1">
              <span className="text-lg font-bold text-text-disabled">{remaining.toFixed(1)}%</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function TimelineItem({ item, isStudent }) {
  const date = getItemDate(item);
  const isAssignment = item.type === 'assignment';
  const colors = item.group_color ? getColorClasses(item.group_color) : null;

  return (
    <div className="flex gap-4 py-3 px-4 rounded-lg hover:bg-surface-1 transition-colors group">
      {/* Date column */}
      <div className="w-20 flex-shrink-0 text-right pt-0.5">
        {date ? (
          <div>
            <div className="text-sm font-medium text-text-secondary">{formatDate(date)}</div>
            <div className="text-xs text-text-disabled">{formatTime(date)}</div>
          </div>
        ) : (
          <span className="text-xs text-text-disabled italic">No date</span>
        )}
      </div>

      {/* Timeline dot and line */}
      <div className="flex flex-col items-center flex-shrink-0">
        <div className={'w-3 h-3 rounded-full mt-1.5 border-2 ' + (isAssignment ? 'border-brand-400 bg-brand-100' : 'border-border-strong bg-surface-0')}></div>
        <div className="w-0.5 flex-1 bg-border-default mt-1"></div>
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pb-2">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              {isAssignment ? (
                <BookOpen className="w-4 h-4 text-brand-500 flex-shrink-0" />
              ) : (
                <Calendar className="w-4 h-4 text-text-disabled flex-shrink-0" />
              )}
              <span className="font-medium text-text-primary truncate">{item.title}</span>
              {colors && item.group_name && (
                <span className={'inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ' + colors.bg + ' ' + colors.text}>
                  <span className={'w-1.5 h-1.5 rounded-full ' + colors.dot}></span>
                  {item.group_name}
                </span>
              )}
            </div>
            {isAssignment && item.points_possible != null && (
              <div className="mt-1 text-sm text-text-tertiary">
                {item.points_possible} {item.points_possible === 1 ? 'point' : 'points'}
              </div>
            )}
          </div>

          {/* Status indicator */}
          {isStudent && item.status && (
            <div className="flex-shrink-0 mt-0.5">
              <StatusBadge status={item.status} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const FILTER_OPTIONS = [
  { value: 'all', label: 'All Items' },
  { value: 'assignment', label: 'Assignments Only' },
  { value: 'event', label: 'Events Only' },
  { value: 'overdue', label: 'Overdue' },
];

const SyllabusPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [syllabusData, setSyllabusData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [filter, setFilter] = useState('all');
  const [filterOpen, setFilterOpen] = useState(false);

  useEffect(() => {
    const fetchSyllabus = async () => {
      try {
        setLoading(true);
        const { data } = await api.request('/courses/' + courseId + '/syllabus');
        setSyllabusData(data);
      } catch (err) {
        setError(err.message || 'Failed to load syllabus');
      } finally {
        setLoading(false);
      }
    };
    fetchSyllabus();
  }, [courseId]);

  const isStudent = syllabusData?.user_role === 'StudentEnrollment';

  // Filter timeline items
  const filteredTimeline = useMemo(() => {
    if (!syllabusData?.timeline) return [];
    let items = syllabusData.timeline;
    if (filter === 'assignment') {
      items = items.filter(i => i.type === 'assignment');
    } else if (filter === 'event') {
      items = items.filter(i => i.type === 'event');
    } else if (filter === 'overdue') {
      items = items.filter(i => i.status === 'missing');
    }
    return items;
  }, [syllabusData, filter]);

  // Group by week
  const groupedTimeline = useMemo(() => {
    const groups = [];
    let currentKey = null;
    let currentGroup = null;
    for (const item of filteredTimeline) {
      const date = getItemDate(item);
      const key = getWeekKey(date);
      if (key !== currentKey) {
        currentKey = key;
        currentGroup = { label: getWeekLabel(date), items: [] };
        groups.push(currentGroup);
      }
      currentGroup.items.push(item);
    }
    return groups;
  }, [filteredTimeline]);

  // Stats for student
  const stats = useMemo(() => {
    if (!syllabusData?.timeline || !isStudent) return null;
    const assignments = syllabusData.timeline.filter(i => i.type === 'assignment');
    return {
      total: assignments.length,
      submitted: assignments.filter(i => i.status === 'submitted').length,
      graded: assignments.filter(i => i.status === 'graded').length,
      missing: assignments.filter(i => i.status === 'missing').length,
      upcoming: assignments.filter(i => i.status === 'upcoming').length,
    };
  }, [syllabusData, isStudent]);

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center min-h-[400px]">
          <div className="text-center">
            <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-brand-600 mx-auto"></div>
            <p className="mt-4 text-text-tertiary">Loading syllabus...</p>
          </div>
        </div>
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="bg-accent-danger/10 border border-accent-danger/30 rounded-lg p-6 text-center">
          <AlertTriangle className="w-8 h-8 text-accent-danger mx-auto mb-2" />
          <h2 className="text-lg font-semibold text-accent-danger">Failed to load syllabus</h2>
          <p className="text-accent-danger mt-1">{error}</p>
        </div>
      </Layout>
    );
  }

  if (!syllabusData) return null;

  const { course, grading_breakdown } = syllabusData;
  const currentFilter = FILTER_OPTIONS.find(f => f.value === filter);

  return (
    <Layout>
      <CourseNav />
      <div className="max-w-4xl mx-auto space-y-8 print:space-y-6">
        {/* Header */}
        <div className="bg-surface-0 rounded-xl shadow-sm border border-border-default overflow-hidden">
          <div className="bg-gradient-to-r from-blue-600 to-indigo-600 px-6 py-8 sm:px-8">
            <h1 className="text-2xl sm:text-3xl font-bold text-white">{course.name}</h1>
            {course.course_code && (
              <p className="mt-1 text-brand-100 text-sm font-medium">{course.course_code}</p>
            )}
            <p className="mt-2 text-brand-200 text-sm">
              {formatDateRange(course.start_at, course.end_at)}
            </p>
          </div>

          {/* Custom syllabus content */}
          {course.syllabus_body && (
            <div className="px-6 py-6 sm:px-8 border-t border-border-subtle">
              <div
                className="prose prose-sm max-w-none text-text-secondary prose-headings:text-text-primary prose-a:text-brand-600"
                dangerouslySetInnerHTML={{ __html: sanitizeHTML(course.syllabus_body) }}
              />
            </div>
          )}
        </div>

        {/* Student Progress Stats */}
        {isStudent && stats && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <div className="bg-surface-0 rounded-lg border border-border-default p-4 text-center shadow-sm">
              <div className="text-2xl font-bold text-accent-success">{stats.submitted}</div>
              <div className="text-xs text-text-tertiary mt-1">Submitted</div>
            </div>
            <div className="bg-surface-0 rounded-lg border border-border-default p-4 text-center shadow-sm">
              <div className="text-2xl font-bold text-accent-warning">{stats.graded}</div>
              <div className="text-xs text-text-tertiary mt-1">Graded</div>
            </div>
            <div className="bg-surface-0 rounded-lg border border-border-default p-4 text-center shadow-sm">
              <div className="text-2xl font-bold text-brand-500">{stats.upcoming}</div>
              <div className="text-xs text-text-tertiary mt-1">Upcoming</div>
            </div>
            <div className="bg-surface-0 rounded-lg border border-border-default p-4 text-center shadow-sm">
              <div className={'text-2xl font-bold ' + (stats.missing > 0 ? 'text-accent-danger' : 'text-text-disabled')}>{stats.missing}</div>
              <div className="text-xs text-text-tertiary mt-1">Overdue</div>
            </div>
          </div>
        )}

        {/* Grading Breakdown */}
        {grading_breakdown && grading_breakdown.length > 0 && (
          <div className="bg-surface-0 rounded-xl shadow-sm border border-border-default p-6 sm:p-8">
            <h2 className="text-lg font-semibold text-text-primary mb-4 flex items-center gap-2">
              <div className="w-1 h-5 bg-brand-500 rounded-full"></div>
              Grading Breakdown
            </h2>
            <GradingBar breakdown={grading_breakdown} />
          </div>
        )}

        {/* Assignment Timeline */}
        <div className="bg-surface-0 rounded-xl shadow-sm border border-border-default overflow-hidden">
          <div className="px-6 py-4 sm:px-8 border-b border-border-subtle flex items-center justify-between">
            <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2">
              <div className="w-1 h-5 bg-indigo-500 rounded-full"></div>
              Assignment Timeline
              <span className="text-sm font-normal text-text-disabled ml-2">
                {filteredTimeline.length} {filteredTimeline.length === 1 ? 'item' : 'items'}
              </span>
            </h2>

            {/* Filter Dropdown */}
            <div className="relative print:hidden">
              <button
                onClick={() => setFilterOpen(!filterOpen)}
                className="inline-flex items-center gap-2 px-3 py-1.5 text-sm font-medium text-text-secondary bg-surface-1 border border-border-default rounded-lg hover:bg-surface-2 transition-colors"
              >
                <Filter className="w-3.5 h-3.5" />
                {currentFilter?.label || 'Filter'}
                <ChevronDown className={'w-3.5 h-3.5 transition-transform ' + (filterOpen ? 'rotate-180' : '')} />
              </button>
              {filterOpen && (
                <div className="absolute right-0 mt-1 w-44 bg-surface-0 border border-border-default rounded-lg shadow-lg z-10 py-1">
                  {FILTER_OPTIONS.map(opt => (
                    <button
                      key={opt.value}
                      onClick={() => { setFilter(opt.value); setFilterOpen(false); }}
                      className={'w-full text-left px-4 py-2 text-sm hover:bg-surface-1 transition-colors ' +
                        (filter === opt.value ? 'text-brand-600 font-medium bg-brand-50' : 'text-text-secondary')}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div className="px-2 sm:px-4 py-2">
            {groupedTimeline.length === 0 ? (
              <div className="text-center py-12 text-text-disabled">
                <Calendar className="w-10 h-10 mx-auto mb-3 opacity-50" />
                <p className="font-medium">No items to display</p>
                <p className="text-sm mt-1">
                  {filter !== 'all' ? 'Try changing the filter above.' : 'Assignments and events will appear here.'}
                </p>
              </div>
            ) : (
              groupedTimeline.map((group, gIdx) => (
                <div key={gIdx} className="mb-2">
                  <div className="sticky top-0 bg-surface-0 z-[5] px-4 py-2">
                    <h3 className="text-xs font-semibold text-text-disabled uppercase tracking-wider">{group.label}</h3>
                  </div>
                  {group.items.map((item, iIdx) => (
                    <TimelineItem key={item.type + '-' + item.id + '-' + iIdx} item={item} isStudent={isStudent} />
                  ))}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </Layout>
  );
};

export default SyllabusPage;
