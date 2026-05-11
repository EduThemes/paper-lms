import { useState, useEffect, useCallback, useMemo } from 'react';
import { Link } from 'react-router-dom';
import {
  Users,
  Heart,
  UserPlus,
  UserMinus,
  BookOpen,
  GraduationCap,
  CalendarClock,
  Bell,
  Sparkles,
  X,
  AlertCircle,
  RefreshCw,
  FileText,
  PenLine,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import RedeemCodeForm from '../components/pairing/RedeemCodeForm';
import useDocumentTitle from '../hooks/useDocumentTitle';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Separator } from '@/components/ui/separator';

const formatDueDate = (iso) => {
  if (!iso) return '';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = d - now;
  const diffDays = Math.round(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays < 0) return 'Overdue';
  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Tomorrow';
  if (diffDays < 7) return `${diffDays} days`;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const formatRelative = (iso) => {
  if (!iso) return '';
  const d = new Date(iso);
  const diffMs = Date.now() - d.getTime();
  const mins = Math.floor(diffMs / 60000);
  if (mins < 60) return `${Math.max(mins, 1)}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const gradeBadgeStyle = (pct) => {
  if (pct == null) return 'bg-surface-2 text-text-secondary';
  if (pct >= 90) return 'bg-accent-success/20 text-accent-success';
  if (pct >= 80) return 'bg-brand-100 text-brand-800';
  if (pct >= 70) return 'bg-accent-warning/20 text-accent-warning';
  return 'bg-accent-danger/20 text-accent-danger';
};

const scoreColor = (score, possible) => {
  if (score == null || !possible) return 'text-text-secondary';
  const pct = (score / possible) * 100;
  if (pct >= 90) return 'text-accent-success';
  if (pct >= 80) return 'text-brand-700';
  if (pct >= 70) return 'text-accent-warning';
  return 'text-accent-danger';
};

// Pill-style child switcher for <= 5 kids; native select for more.
const ChildSwitcher = ({ kids, selectedId, onSelect }) => {
  if (!kids || kids.length === 0) return null;

  if (kids.length > 5) {
    return (
      <div className="mb-6">
        <label htmlFor="kid-select" className="block text-sm font-medium text-text-secondary mb-1">
          Viewing
        </label>
        <select
          id="kid-select"
          value={selectedId ?? ''}
          onChange={(e) => onSelect(Number(e.target.value))}
          className="w-full sm:w-72 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        >
          {kids.map((k) => (
            <option key={k.id} value={k.id}>
              {k.name || `Student #${k.id}`}
            </option>
          ))}
        </select>
      </div>
    );
  }

  return (
    <div className="mb-6 flex flex-wrap gap-2" role="tablist" aria-label="Switch child">
      {kids.map((k) => {
        const active = k.id === selectedId;
        const initials = (k.name || `S${k.id}`)
          .split(' ')
          .map((s) => s[0])
          .filter(Boolean)
          .slice(0, 2)
          .join('')
          .toUpperCase();
        return (
          <button
            key={k.id}
            role="tab"
            aria-selected={active}
            onClick={() => onSelect(k.id)}
            className={`inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-medium transition-colors border ${
              active
                ? 'bg-brand-600 text-white border-brand-600 shadow-sm'
                : 'bg-surface-0 text-text-secondary border-border-default hover:border-blue-300 hover:bg-brand-50'
            }`}
          >
            <span
              className={`inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-semibold ${
                active ? 'bg-surface-0/20 text-white' : 'bg-brand-100 text-brand-700'
              }`}
            >
              {initials}
            </span>
            {k.name || `Student #${k.id}`}
          </button>
        );
      })}
    </div>
  );
};

const SkeletonCard = ({ rows = 3 }) => (
  <Card>
    <CardHeader>
      <Skeleton className="h-5 w-32" />
    </CardHeader>
    <CardContent className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center justify-between">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-4 w-12" />
        </div>
      ))}
    </CardContent>
  </Card>
);

const EmptyState = ({ icon: Icon, title, body }) => (
  <div className="text-center py-6">
    <Icon className="w-10 h-10 text-gray-200 mx-auto mb-2" />
    <p className="text-text-secondary text-sm font-medium">{title}</p>
    {body && <p className="text-text-disabled text-xs mt-1">{body}</p>}
  </div>
);

const ObserverDashboardPage = () => {
  useDocumentTitle('My Kids · Paper LMS');
  const { user } = useAuth();

  // Linked kids
  const [kids, setKids] = useState([]);
  const [kidsLoading, setKidsLoading] = useState(true);
  const [kidsError, setKidsError] = useState(null);

  // Selected child + per-child overview
  const [selectedChildId, setSelectedChildId] = useState(null);
  const [overviewByChild, setOverviewByChild] = useState({});
  const [overviewLoadingByChild, setOverviewLoadingByChild] = useState({});
  const [overviewErrorByChild, setOverviewErrorByChild] = useState({});

  // Link form state — RedeemCodeForm owns its own input/loading/error state.
  const [showLinkForm, setShowLinkForm] = useState(false);

  // Unlink confirmation modal (preserved)
  const [unlinkTarget, setUnlinkTarget] = useState(null);
  const [unlinkLoading, setUnlinkLoading] = useState(false);

  // ---- Data fetching ----
  const fetchKids = useCallback(async () => {
    if (!user?.id) return;
    try {
      const data = await api.getObservees(user.id);
      const list = Array.isArray(data) ? data : data?.data || [];
      setKids(list);
      if (list.length > 0) {
        setSelectedChildId((prev) => (prev && list.some((k) => k.id === prev) ? prev : list[0].id));
      } else {
        setSelectedChildId(null);
      }
    } catch (err) {
      setKidsError(err.message);
    } finally {
      setKidsLoading(false);
    }
  }, [user?.id]);

  useEffect(() => {
    fetchKids();
  }, [fetchKids]);

  const fetchOverview = useCallback(
    async (childId) => {
      if (!user?.id || !childId) return;
      setOverviewLoadingByChild((p) => ({ ...p, [childId]: true }));
      setOverviewErrorByChild((p) => ({ ...p, [childId]: null }));
      try {
        // The api.js method `getChildOverview` is documented in
        // OBSERVER_OVERVIEW_PATCH.md — fall back to the raw endpoint if it
        // hasn't been added yet so this page degrades gracefully.
        let data;
        if (typeof api.getChildOverview === 'function') {
          data = await api.getChildOverview(user.id, childId);
        } else if (typeof api.request === 'function') {
          data = await api.request(`/users/${user.id}/observees/${childId}/overview`);
        } else {
          throw new Error('Overview endpoint not available yet');
        }
        setOverviewByChild((p) => ({ ...p, [childId]: data || {} }));
      } catch (err) {
        setOverviewErrorByChild((p) => ({ ...p, [childId]: err.message }));
      } finally {
        setOverviewLoadingByChild((p) => ({ ...p, [childId]: false }));
      }
    },
    [user?.id]
  );

  useEffect(() => {
    if (selectedChildId && !overviewByChild[selectedChildId]) {
      fetchOverview(selectedChildId);
    }
  }, [selectedChildId, overviewByChild, fetchOverview]);

  // ---- Mutations ----
  const handleUnlinkStudent = async () => {
    if (!unlinkTarget || !user?.id) return;
    setUnlinkLoading(true);
    try {
      await api.unlinkObservee(user.id, unlinkTarget.id);
      const target = unlinkTarget;
      setUnlinkTarget(null);
      setOverviewByChild((p) => {
        const n = { ...p };
        delete n[target.id];
        return n;
      });
      setKidsLoading(true);
      await fetchKids();
    } catch (err) {
      setKidsError(err.message);
      setUnlinkTarget(null);
    } finally {
      setUnlinkLoading(false);
    }
  };

  // ---- Derived ----
  const selectedKid = useMemo(
    () => kids.find((k) => k.id === selectedChildId),
    [kids, selectedChildId]
  );
  const overview = selectedChildId ? overviewByChild[selectedChildId] : null;
  const overviewLoading = selectedChildId ? overviewLoadingByChild[selectedChildId] : false;
  const overviewError = selectedChildId ? overviewErrorByChild[selectedChildId] : null;

  // ---- Render: loading shell ----
  if (kidsLoading) {
    return (
      <Layout>
        <div className="mb-6">
          <Skeleton className="h-8 w-48 mb-2" />
          <Skeleton className="h-4 w-72" />
        </div>
        <div className="flex gap-2 mb-6">
          <Skeleton className="h-10 w-32 rounded-full" />
          <Skeleton className="h-10 w-32 rounded-full" />
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
          <SkeletonCard />
        </div>
      </Layout>
    );
  }

  // ---- Render: hard error & no data ----
  if (kidsError && kids.length === 0) {
    return (
      <Layout>
        <div className="text-center py-12">
          <AlertCircle className="w-12 h-12 text-red-400 mx-auto mb-4" />
          <p className="text-accent-danger mb-4">{kidsError}</p>
          <Button
            onClick={() => {
              setKidsError(null);
              setKidsLoading(true);
              fetchKids();
            }}
          >
            <RefreshCw className="w-4 h-4 mr-2" />
            Try Again
          </Button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      {/* Page Header */}
      <div className="mb-6">
        <div className="flex items-center space-x-3 mb-1">
          <Heart className="w-7 h-7 text-pink-500" />
          <h2 className="text-2xl font-bold text-text-primary">My Kids</h2>
        </div>
        <p className="text-text-secondary mt-1">
          Keep an eye on your kids&rsquo; classes, grades, and what&rsquo;s due this week.
        </p>
      </div>

      {/* Non-fatal error banner */}
      {kidsError && (
        <div
          className="mb-6 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg flex items-center justify-between"
          role="alert"
        >
          <div className="flex items-center space-x-2">
            <AlertCircle className="w-4 h-4 text-accent-danger shrink-0" />
            <span className="text-accent-danger text-sm">{kidsError}</span>
          </div>
          <button onClick={() => setKidsError(null)} className="text-red-400 hover:text-accent-danger">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Header row: switcher + actions */}
      <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3 mb-2">
        <div className="flex-1 min-w-0">
          <ChildSwitcher
            kids={kids}
            selectedId={selectedChildId}
            onSelect={setSelectedChildId}
          />
        </div>
        <div className="flex flex-wrap gap-2 mb-6 sm:mb-6">
          <Button
            variant="outline"
            onClick={() => setShowLinkForm((s) => !s)}
          >
            <UserPlus className="w-4 h-4 mr-2" />
            Link a child
          </Button>
          {selectedKid && (
            <Button
              variant="outline"
              onClick={() => setUnlinkTarget(selectedKid)}
              className="text-accent-danger hover:text-accent-danger"
            >
              <UserMinus className="w-4 h-4 mr-2" />
              Unlink
            </Button>
          )}
        </div>
      </div>

      {/* Link a child form */}
      {showLinkForm && (
        <Card className="mb-6 border-blue-200">
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle className="text-base">Link a child</CardTitle>
            <button
              onClick={() => setShowLinkForm(false)}
              className="text-text-disabled hover:text-text-secondary"
              aria-label="Close"
            >
              <X className="w-4 h-4" />
            </button>
          </CardHeader>
          <CardContent>
            <RedeemCodeForm
              onSuccess={() => {
                setShowLinkForm(false);
                setKidsLoading(true);
                fetchKids();
              }}
            />
          </CardContent>
        </Card>
      )}

      {/* No kids linked yet */}
      {kids.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <Users className="w-12 h-12 text-gray-300 mx-auto mb-3" />
            <p className="text-text-secondary font-medium mb-1">No kids linked yet</p>
            <p className="text-text-tertiary text-sm mb-4">
              Use &ldquo;Link a child&rdquo; above to start following your kid&rsquo;s classes.
            </p>
            <Button onClick={() => setShowLinkForm(true)}>
              <UserPlus className="w-4 h-4 mr-2" />
              Link a child
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Per-child overview error */}
          {overviewError && (
            <div className="mb-4 p-3 bg-accent-warning/10 border border-accent-warning/30 rounded-lg flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <AlertCircle className="w-4 h-4 text-accent-warning shrink-0" />
                <span className="text-amber-800 text-sm">
                  Couldn&rsquo;t load latest data: {overviewError}
                </span>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() => fetchOverview(selectedChildId)}
              >
                <RefreshCw className="w-4 h-4 mr-1" />
                Retry
              </Button>
            </div>
          )}

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Classes */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <BookOpen className="w-5 h-5 text-brand-600" />
                  Classes
                </CardTitle>
              </CardHeader>
              <CardContent>
                {overviewLoading ? (
                  <div className="space-y-3">
                    {[0, 1, 2].map((i) => (
                      <div key={i} className="flex items-center justify-between">
                        <Skeleton className="h-4 w-2/3" />
                        <Skeleton className="h-5 w-12" />
                      </div>
                    ))}
                  </div>
                ) : !overview?.courses?.length ? (
                  <EmptyState
                    icon={GraduationCap}
                    title="No classes yet"
                    body="When your child gets enrolled, classes show up here."
                  />
                ) : (
                  <ul className="divide-y divide-gray-100">
                    {overview.courses.map((c, idx) => (
                      <li key={c.course_id} className={idx === 0 ? 'pb-3' : 'py-3'}>
                        <Link
                          to={`/courses/${c.course_id}?as_child=${selectedChildId}`}
                          className="flex items-center justify-between hover:bg-surface-1 rounded-md -mx-2 px-2 py-1 transition"
                        >
                          <div className="min-w-0 flex-1">
                            <p className="font-medium text-text-primary truncate">{c.name}</p>
                            <div className="flex items-center gap-2 mt-0.5 text-xs text-text-tertiary">
                              {c.course_code && <span>{c.course_code}</span>}
                              {c.pending_count > 0 && (
                                <Badge variant="secondary" className="text-xs">
                                  {c.pending_count} pending
                                </Badge>
                              )}
                            </div>
                          </div>
                          <span
                            className={`shrink-0 ml-3 text-sm font-semibold px-2.5 py-1 rounded-full ${gradeBadgeStyle(
                              c.current_grade
                            )}`}
                          >
                            {c.current_grade != null ? `${c.current_grade.toFixed(1)}%` : '—'}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            {/* Due This Week */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <CalendarClock className="w-5 h-5 text-accent-warning" />
                  Due this week
                </CardTitle>
              </CardHeader>
              <CardContent>
                {overviewLoading ? (
                  <div className="space-y-3">
                    {[0, 1, 2].map((i) => (
                      <div key={i} className="flex items-center justify-between">
                        <Skeleton className="h-4 w-2/3" />
                        <Skeleton className="h-5 w-16" />
                      </div>
                    ))}
                  </div>
                ) : !overview?.upcoming_this_week?.length ? (
                  <EmptyState
                    icon={Sparkles}
                    title="Nothing due this week"
                    body="Enjoy the breather!"
                  />
                ) : (
                  <ul className="space-y-2">
                    {overview.upcoming_this_week.map((u) => (
                      <li
                        key={`${u.type}-${u.id}`}
                        className="flex items-center justify-between gap-3"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          {u.type === 'quiz' ? (
                            <PenLine className="w-4 h-4 text-purple-500 shrink-0" />
                          ) : (
                            <FileText className="w-4 h-4 text-brand-500 shrink-0" />
                          )}
                          <div className="min-w-0">
                            <p className="text-sm font-medium text-text-primary truncate">
                              {u.title}
                            </p>
                            <p className="text-xs text-text-tertiary truncate">{u.course_name}</p>
                          </div>
                        </div>
                        <Badge variant="outline" className="shrink-0">
                          {formatDueDate(u.due_at)}
                        </Badge>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            {/* Recent Grades */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <GraduationCap className="w-5 h-5 text-accent-success" />
                  Recent grades
                </CardTitle>
              </CardHeader>
              <CardContent>
                {overviewLoading ? (
                  <div className="space-y-3">
                    {[0, 1, 2].map((i) => (
                      <div key={i} className="flex items-center justify-between">
                        <Skeleton className="h-4 w-2/3" />
                        <Skeleton className="h-5 w-14" />
                      </div>
                    ))}
                  </div>
                ) : !overview?.recent_grades?.length ? (
                  <EmptyState
                    icon={GraduationCap}
                    title="No grades posted yet"
                    body="Grades will show up here once teachers post them."
                  />
                ) : (
                  <ul className="space-y-3">
                    {overview.recent_grades.map((g) => (
                      <li key={g.submission_id} className="flex items-center justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-text-primary truncate">
                            {g.assignment_name}
                          </p>
                          <div className="flex items-center gap-2 text-xs text-text-tertiary">
                            <span className="truncate">{g.course_name}</span>
                            <span>·</span>
                            <span>{formatRelative(g.graded_at)}</span>
                          </div>
                        </div>
                        <div
                          className={`shrink-0 text-sm font-semibold ${scoreColor(
                            g.score,
                            g.points_possible
                          )}`}
                        >
                          {g.score != null ? g.score : '—'}
                          {g.points_possible != null && (
                            <span className="text-text-disabled font-normal">
                              {' '}
                              / {g.points_possible}
                            </span>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            {/* Recent Activity */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Bell className="w-5 h-5 text-purple-600" />
                  Recent activity
                </CardTitle>
              </CardHeader>
              <CardContent>
                {overviewLoading ? (
                  <div className="space-y-3">
                    {[0, 1, 2].map((i) => (
                      <div key={i} className="flex items-center justify-between">
                        <Skeleton className="h-4 w-3/4" />
                        <Skeleton className="h-3 w-12" />
                      </div>
                    ))}
                  </div>
                ) : !overview?.recent_activity?.length ? (
                  <EmptyState
                    icon={Bell}
                    title="Quiet around here"
                    body="Announcements and class updates will land here."
                  />
                ) : (
                  <ul className="space-y-3">
                    {overview.recent_activity.map((a, idx) => (
                      <li key={`${a.type}-${a.id}-${idx}`} className="flex items-start gap-3">
                        {a.type === 'announcement' ? (
                          <Bell className="w-4 h-4 text-purple-500 mt-0.5 shrink-0" />
                        ) : (
                          <FileText className="w-4 h-4 text-brand-500 mt-0.5 shrink-0" />
                        )}
                        <div className="min-w-0 flex-1">
                          <p className="text-sm text-text-primary truncate">
                            <span className="font-medium">{a.title}</span>
                          </p>
                          <p className="text-xs text-text-tertiary truncate">
                            {a.course_name} · {formatRelative(a.occurred_at)}
                          </p>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>

          <Separator className="my-8" />

          {/* Footer: full kid roster + manage link */}
          <div className="text-sm text-text-tertiary flex items-center justify-between">
            <span>
              Following {kids.length} {kids.length === 1 ? 'child' : 'children'}
            </span>
            <button
              onClick={() => setShowLinkForm(true)}
              className="text-brand-600 hover:text-brand-700 font-medium inline-flex items-center gap-1"
            >
              <UserPlus className="w-4 h-4" /> Link another
            </button>
          </div>
        </>
      )}

      {/* Unlink Confirmation Modal — preserved structure from original page */}
      {unlinkTarget && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-surface-0 rounded-lg shadow-lg p-6 w-full max-w-md mx-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-text-primary">Unlink Child</h3>
              <button
                onClick={() => setUnlinkTarget(null)}
                className="text-text-disabled hover:text-text-secondary"
                disabled={unlinkLoading}
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="mb-6">
              <div className="flex items-center space-x-3 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg mb-3">
                <UserMinus className="w-5 h-5 text-accent-danger shrink-0" />
                <div>
                  <p className="font-medium text-text-primary">
                    {unlinkTarget.name || `Student #${unlinkTarget.id}`}
                  </p>
                  <p className="text-sm text-text-tertiary">
                    {unlinkTarget.email || unlinkTarget.login_id || `ID: ${unlinkTarget.id}`}
                  </p>
                </div>
              </div>
              <p className="text-sm text-text-secondary">
                Are you sure you want to unlink this child? You will no longer be able to view
                their classes or monitor their progress. You can re-link them later if needed.
              </p>
            </div>
            <div className="flex justify-end space-x-2">
              <Button
                variant="outline"
                onClick={() => setUnlinkTarget(null)}
                disabled={unlinkLoading}
              >
                Cancel
              </Button>
              <Button
                onClick={handleUnlinkStudent}
                disabled={unlinkLoading}
                className="bg-accent-danger hover:bg-accent-danger/90 text-white"
              >
                <UserMinus className="w-4 h-4 mr-2" />
                {unlinkLoading ? 'Unlinking…' : 'Unlink Child'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default ObserverDashboardPage;
