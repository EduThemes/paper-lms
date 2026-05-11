import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import {
  Clock,
  Megaphone,
  GraduationCap,
  BookOpen,
  MoreHorizontal,
  Star,
  BellOff,
  Settings,
  ArrowRight,
  Sparkles,
} from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';
import useDocumentTitle from '../hooks/useDocumentTitle';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Separator } from '@/components/ui/separator';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

const hashHue = (id) => {
  const n = Number(id) || 0;
  let h = 2166136261;
  const s = String(n);
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return Math.abs(h) % 360;
};

const getSeason = (date = new Date()) => {
  const m = date.getMonth();
  if (m <= 1 || m === 11) return 'Winter';
  if (m <= 4) return 'Spring';
  if (m <= 7) return 'Summer';
  return 'Fall';
};

const getGreeting = (date = new Date()) => {
  const h = date.getHours();
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
};

const formatDueTime = (iso) => {
  const d = new Date(iso);
  const now = new Date();
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();
  if (sameDay) {
    return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  }
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const formatRelative = (iso) => {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 60) return `${Math.max(1, m)}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d ago`;
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const initialsFor = (name) => {
  if (!name) return '??';
  return name
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0].toUpperCase())
    .join('');
};

const HeroGreeting = ({ firstName }) => {
  const today = new Date();
  const dateStr = today.toLocaleDateString(undefined, {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
  });
  return (
    <header className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">
          {getGreeting()}, {firstName || 'there'}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Here is what is happening across your courses today.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">{dateStr}</span>
        <Badge variant="secondary" className="rounded-full">
          <Sparkles className="me-1 h-3 w-3" aria-hidden="true" />
          {getSeason(today)}
        </Badge>
      </div>
    </header>
  );
};

const TodayPanel = ({ items, loading }) => {
  return (
    <Card className="overflow-hidden">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-primary" aria-hidden="true" />
          <CardTitle className="text-base font-semibold">Due today</CardTitle>
        </div>
        <Badge variant="outline" className="font-normal">
          {loading ? '—' : `${items.length} item${items.length === 1 ? '' : 's'}`}
        </Badge>
      </CardHeader>
      <Separator />
      <CardContent className="p-0">
        {loading ? (
          <div className="space-y-3 p-4">
            {[0, 1, 2].map((i) => (
              <div key={i} className="flex items-center gap-3">
                <Skeleton className="h-10 w-1 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3 w-2/3" />
                  <Skeleton className="h-3 w-1/3" />
                </div>
                <Skeleton className="h-8 w-16 rounded-md" />
              </div>
            ))}
          </div>
        ) : items.length === 0 ? (
          <div className="px-6 py-10 text-center">
            <p className="text-sm text-muted-foreground">
              You are all caught up — nothing due today.
            </p>
          </div>
        ) : (
          <ul className="divide-y">
            {items.map((item) => {
              const hue = hashHue(item.course_id);
              return (
                <li key={`${item.course_id}-${item.id}`}>
                  <Link
                    to={`/courses/${item.course_id}/assignments/${item.id}`}
                    className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-accent/40"
                  >
                    <span
                      aria-hidden="true"
                      className="h-10 w-1 shrink-0 rounded-full"
                      style={{ backgroundColor: `hsl(${hue} 70% 50%)` }}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-foreground">
                        {item.name}
                      </div>
                      <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
                        <span className="truncate">
                          {item.course_code || item.course_name}
                        </span>
                        <span aria-hidden="true">·</span>
                        <span className="shrink-0">{formatDueTime(item.due_at)}</span>
                      </div>
                    </div>
                    <Button
                      asChild
                      size="sm"
                      variant="ghost"
                      className="opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
                    >
                      <span>
                        Start <ArrowRight className="ms-1 h-3 w-3" aria-hidden="true" />
                      </span>
                    </Button>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
};

const ProgressRing = ({ percent = 0, size = 36, stroke = 4 }) => {
  const radius = (size - stroke) / 2;
  const circ = 2 * Math.PI * radius;
  const clamped = Math.max(0, Math.min(100, percent));
  const offset = circ - (clamped / 100) * circ;
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} role="img" aria-label={`${Math.round(clamped)} percent complete`}>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="currentColor"
          strokeOpacity="0.15"
          strokeWidth={stroke}
          fill="none"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="currentColor"
          strokeWidth={stroke}
          strokeDasharray={circ}
          strokeDashoffset={offset}
          strokeLinecap="round"
          fill="none"
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
          className="text-primary transition-all"
        />
      </svg>
      <span className="absolute text-[10px] font-semibold tabular-nums text-foreground">
        {Math.round(clamped)}
      </span>
    </div>
  );
};

const CourseTile = ({ course, nextItem, percent }) => {
  const hue = hashHue(course.id);
  const gradient = `linear-gradient(135deg, hsl(${hue} 75% 55%) 0%, hsl(${(hue + 40) % 360} 70% 45%) 100%)`;

  const handleMenuClick = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };

  return (
    <Link
      to={`/courses/${course.id}`}
      className="group block focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 rounded-lg"
    >
      <Card className="h-full overflow-hidden transition-all hover:shadow-md hover:-translate-y-0.5">
        <div
          className="relative h-24 w-full"
          style={{ background: gradient }}
          aria-hidden="true"
        >
          <div className="absolute inset-0 bg-gradient-to-t from-black/20 to-transparent" />
          <div
            className="absolute end-2 top-2"
            onClick={handleMenuClick}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') e.stopPropagation();
            }}
          >
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Course options"
                  className="h-7 w-7 bg-surface-0/20 text-white hover:bg-surface-0/30 hover:text-white backdrop-blur"
                >
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                <DropdownMenuItem onSelect={() => {}}>
                  <Star className="h-4 w-4" />
                  Favorite
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => {}}>
                  <BellOff className="h-4 w-4" />
                  Mute
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => {}}>
                  <Settings className="h-4 w-4" />
                  Open settings
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <CardContent className="p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                {course.course_code || 'Course'}
              </div>
              <h3 className="mt-0.5 line-clamp-2 text-sm font-semibold text-foreground">
                {course.name}
              </h3>
            </div>
          </div>

          <div className="mt-4 flex items-center justify-between gap-2">
            <ProgressRing percent={percent} />
            {nextItem ? (
              <Badge
                variant="secondary"
                className="max-w-[160px] truncate rounded-full font-normal"
                title={nextItem.name}
              >
                Next: {nextItem.name}
              </Badge>
            ) : (
              <span className="text-xs text-muted-foreground">No upcoming work</span>
            )}
          </div>
        </CardContent>
      </Card>
    </Link>
  );
};

const ActivityRail = ({ announcements, grades, loading }) => {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base font-semibold">Activity</CardTitle>
      </CardHeader>
      <Separator />
      <CardContent className="p-0">
        {loading ? (
          <div className="space-y-4 p-4">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="flex items-start gap-3">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3 w-3/4" />
                  <Skeleton className="h-3 w-1/2" />
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="space-y-1 p-2">
            {announcements.length > 0 && (
              <div className="px-2 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                Announcements
              </div>
            )}
            {announcements.map((a) => (
              <Link
                key={`a-${a.id}`}
                to={`/courses/${a.course_id}/announcements`}
                className="flex items-start gap-3 rounded-md px-2 py-2 transition-colors hover:bg-accent/40"
              >
                <Avatar className="h-8 w-8">
                  <AvatarFallback className="bg-primary/10 text-primary">
                    <Megaphone className="h-4 w-4" />
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-foreground">{a.title}</div>
                  <div className="truncate text-xs text-muted-foreground">
                    {a.course_name} · {formatRelative(a.created_at)}
                  </div>
                </div>
              </Link>
            ))}

            {grades.length > 0 && (
              <div className="px-2 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                Recent grades
              </div>
            )}
            {grades.map((g) => (
              <Link
                key={`g-${g.course_id}-${g.id}`}
                to={`/courses/${g.course_id}/assignments/${g.assignment_id || ''}`}
                className="flex items-start gap-3 rounded-md px-2 py-2 transition-colors hover:bg-accent/40"
              >
                <Avatar className="h-8 w-8">
                  <AvatarFallback className="bg-accent-success/10 text-accent-success">
                    <GraduationCap className="h-4 w-4" />
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-foreground">
                    {g.title || 'Assignment graded'}
                  </div>
                  <div className="truncate text-xs text-muted-foreground">
                    {g.course_name} · {g.score != null ? `${g.score} pts` : 'Graded'}
                    {g.graded_at ? ` · ${formatRelative(g.graded_at)}` : ''}
                  </div>
                </div>
              </Link>
            ))}

            {!loading && announcements.length === 0 && grades.length === 0 && (
              <div className="px-3 py-8 text-center text-sm text-muted-foreground">
                No recent activity yet.
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
};

const DashboardSkeleton = () => (
  <div className="space-y-6">
    <div className="flex items-end justify-between">
      <div className="space-y-2">
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-4 w-56" />
      </div>
      <Skeleton className="h-6 w-40" />
    </div>

    <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
      <div className="space-y-6">
        <Card>
          <CardContent className="space-y-3 p-4">
            <Skeleton className="h-4 w-32" />
            {[0, 1, 2].map((i) => (
              <div key={i} className="flex items-center gap-3">
                <Skeleton className="h-10 w-1 rounded-full" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3 w-2/3" />
                  <Skeleton className="h-3 w-1/3" />
                </div>
                <Skeleton className="h-8 w-16 rounded-md" />
              </div>
            ))}
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Card key={i} className="overflow-hidden">
              <Skeleton className="h-24 w-full rounded-none" />
              <CardContent className="space-y-3 p-4">
                <Skeleton className="h-3 w-1/3" />
                <Skeleton className="h-4 w-3/4" />
                <div className="flex items-center justify-between">
                  <Skeleton className="h-9 w-9 rounded-full" />
                  <Skeleton className="h-5 w-24 rounded-full" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>

      <Card>
        <CardContent className="space-y-4 p-4">
          {[0, 1, 2, 3, 4].map((i) => (
            <div key={i} className="flex items-start gap-3">
              <Skeleton className="h-8 w-8 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-3 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  </div>
);

const EmptyDashboard = () => (
  <Card className="mx-auto max-w-xl text-center">
    <CardContent className="space-y-4 px-6 py-12">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
        <BookOpen className="h-6 w-6" />
      </div>
      <div>
        <h2 className="text-lg font-semibold text-foreground">Welcome to Paper LMS</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          You are not enrolled in any courses yet. Browse the catalog to get started.
        </p>
      </div>
      <Button asChild>
        <Link to="/courses">Browse course catalog</Link>
      </Button>
    </CardContent>
  </Card>
);

const DashboardPage = () => {
  useDocumentTitle('Dashboard');
  const { user } = useAuth();
  const [courses, setCourses] = useState([]);
  const [loading, setLoading] = useState(true);
  const [detailsLoading, setDetailsLoading] = useState(true);
  const [error, setError] = useState(null);
  const [upcomingAssignments, setUpcomingAssignments] = useState([]);
  const [announcements, setAnnouncements] = useState([]);
  const [recentGrades, setRecentGrades] = useState([]);
  const [progressByCourse, setProgressByCourse] = useState({});

  const fetchData = async () => {
    setError(null);
    setLoading(true);
    setDetailsLoading(true);
    try {
      let courseData = [];
      if (typeof api.getDashboardCards === 'function') {
        try {
          const result = await api.getDashboardCards();
          courseData = result?.data || result || [];
        } catch {
          const fallback = await api.getCourses(1, 50);
          courseData = fallback?.data || [];
        }
      } else {
        const result = await api.getCourses(1, 50);
        courseData = result?.data || [];
      }

      setCourses(courseData);
      setLoading(false);

      const now = new Date();
      const cohort = courseData.slice(0, 8);
      const allAssignments = [];
      const allAnnouncements = [];
      const allGrades = [];
      const progress = {};

      const fetches = cohort.map(async (course) => {
        const [assignResult, announcementResult, subResult] = await Promise.allSettled([
          api.getAssignments(course.id, 1, 25),
          api.getCourseAnnouncements(course.id, 1, 5),
          api.getCourseSubmissions(course.id, 1, 25, 'self').catch(() => ({ data: [] })),
        ]);

        const subMap = {};
        let submittedCount = 0;
        let totalGradeable = 0;
        if (subResult.status === 'fulfilled') {
          const subs = subResult.value?.data || subResult.value || [];
          (Array.isArray(subs) ? subs : []).forEach((s) => {
            if (s.assignment_id) subMap[s.assignment_id] = s;
            if (s.workflow_state === 'graded' && s.score != null) {
              allGrades.push({
                id: s.id,
                assignment_id: s.assignment_id,
                title: s.assignment_name || 'Assignment',
                score: s.score,
                graded_at: s.graded_at || s.updated_at,
                course_name: course.name,
                course_id: course.id,
              });
            }
          });
        }

        if (assignResult.status === 'fulfilled') {
          const assignments = assignResult.value?.data || [];
          assignments.forEach((a) => {
            if (a.points_possible > 0 || a.due_at) totalGradeable += 1;
            const sub = subMap[a.id];
            if (sub && (sub.workflow_state === 'submitted' || sub.workflow_state === 'graded')) {
              submittedCount += 1;
            }
            if (a.due_at) {
              const dueDate = new Date(a.due_at);
              if (dueDate >= now && !sub) {
                allAssignments.push({
                  id: a.id,
                  name: a.name,
                  due_at: a.due_at,
                  course_id: course.id,
                  course_name: course.name,
                  course_code: course.course_code,
                });
              }
            }
          });
        }

        progress[course.id] =
          totalGradeable > 0 ? Math.round((submittedCount / totalGradeable) * 100) : 0;

        if (announcementResult.status === 'fulfilled') {
          const anns = announcementResult.value?.data || [];
          anns.forEach((a) => {
            allAnnouncements.push({
              ...a,
              course_name: course.name,
              course_id: course.id,
            });
          });
        }
      });

      await Promise.allSettled(fetches);

      allAssignments.sort((a, b) => new Date(a.due_at) - new Date(b.due_at));
      setUpcomingAssignments(allAssignments);

      allAnnouncements.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
      setAnnouncements(allAnnouncements.slice(0, 5));

      allGrades.sort((a, b) => new Date(b.graded_at || 0) - new Date(a.graded_at || 0));
      setRecentGrades(allGrades.slice(0, 5));

      setProgressByCourse(progress);
      setDetailsLoading(false);
    } catch (err) {
      setError(err.message || 'Could not load dashboard');
      setLoading(false);
      setDetailsLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const todayItems = useMemo(() => {
    const now = new Date();
    const endOfToday = new Date(now);
    endOfToday.setHours(23, 59, 59, 999);
    const dueToday = upcomingAssignments.filter((a) => {
      const d = new Date(a.due_at);
      return d >= now && d <= endOfToday;
    });
    const source = dueToday.length > 0 ? dueToday : upcomingAssignments;
    return source.slice(0, 5);
  }, [upcomingAssignments]);

  const nextByCourse = useMemo(() => {
    const map = {};
    upcomingAssignments.forEach((a) => {
      if (!map[a.course_id]) map[a.course_id] = a;
    });
    return map;
  }, [upcomingAssignments]);

  const firstName = user?.first_name || user?.name?.split(' ')[0] || '';

  if (loading) {
    return (
      <Layout>
        <DashboardSkeleton />
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <Card className="mx-auto max-w-md">
          <CardContent className="space-y-3 px-6 py-10 text-center">
            <p className="text-sm text-destructive">{error}</p>
            <Button onClick={fetchData} variant="outline">
              Try Again
            </Button>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  if (!courses || courses.length === 0) {
    return (
      <Layout>
        <EmptyDashboard />
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="space-y-6">
        <HeroGreeting firstName={firstName} />

        <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-6">
            <TodayPanel items={todayItems} loading={detailsLoading} />

            <section>
              <div className="mb-3 flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                  My courses
                </h2>
                <Button asChild variant="link" size="sm" className="h-auto p-0">
                  <Link to="/courses">View all</Link>
                </Button>
              </div>
              <div className={cn('grid gap-4', 'grid-cols-1 md:grid-cols-2 xl:grid-cols-3')}>
                {courses.map((course) => (
                  <CourseTile
                    key={course.id}
                    course={course}
                    nextItem={nextByCourse[course.id]}
                    percent={progressByCourse[course.id] || 0}
                  />
                ))}
              </div>
            </section>
          </div>

          <aside className="xl:sticky xl:top-6 xl:self-start">
            <ActivityRail
              announcements={announcements}
              grades={recentGrades}
              loading={detailsLoading}
            />
          </aside>
        </div>
      </div>
    </Layout>
  );
};

export default DashboardPage;
