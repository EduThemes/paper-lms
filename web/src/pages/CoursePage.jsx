import React, { useState, useEffect, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ChevronRight,
  ChevronDown,
  FileText,
  PenTool,
  HelpCircle,
  ExternalLink,
  Minus,
  Book,
  Award,
  Calendar,
  CheckCircle2,
  Circle,
  Users,
  Settings,
  Megaphone,
  Layout as LayoutIcon,
  ListChecks,
  GraduationCap,
  MessagesSquare,
  Sparkles,
  Activity,
  CalendarClock,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import { useCourseUI } from '../contexts/CourseUIContext';
import useDocumentTitle from '../hooks/useDocumentTitle';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import HomeEngine from '../components/home/HomeEngine';
import K2Layout from '../components/home/K2Layout';
import { sanitizeHTML } from '../components/RichContentViewer';
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Badge } from '../components/ui/badge';
import { Skeleton } from '../components/ui/skeleton';
import { Avatar, AvatarFallback, AvatarImage } from '../components/ui/avatar';
import { Separator } from '../components/ui/separator';
import { cn } from '@/lib/utils';

const ITEM_ICONS = {
  Page: FileText,
  Assignment: PenTool,
  Quiz: HelpCircle,
  ExternalUrl: ExternalLink,
  SubHeader: Minus,
};

const hashHue = (id) => {
  const s = String(id ?? '0');
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = (h * 31 + s.charCodeAt(i)) >>> 0;
  }
  return h % 360;
};

const initialsOf = (name) => {
  if (!name) return '?';
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() || '')
    .join('') || '?';
};

const daysUntil = (dateStr) => {
  if (!dateStr) return null;
  const due = new Date(dateStr).getTime();
  if (Number.isNaN(due)) return null;
  const now = Date.now();
  return Math.round((due - now) / 86_400_000);
};

const urgencyVariant = (days) => {
  if (days === null || days === undefined) return 'outline';
  if (days < 0) return 'destructive';
  if (days <= 1) return 'destructive';
  if (days <= 3) return 'default';
  if (days <= 7) return 'secondary';
  return 'outline';
};

const formatDueLabel = (days) => {
  if (days === null || days === undefined) return 'No due date';
  if (days < 0) return `Overdue by ${Math.abs(days)}d`;
  if (days === 0) return 'Due today';
  if (days === 1) return 'Due tomorrow';
  return `Due in ${days}d`;
};

const ProgressRing = ({ value = 0, size = 92, stroke = 8, label = 'Course progress' }) => {
  const pct = Math.max(0, Math.min(100, value));
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const offset = c - (pct / 100) * c;
  return (
    <div
      role="img"
      aria-label={`${label}: ${Math.round(pct)} percent`}
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="rgba(255,255,255,0.25)"
          strokeWidth={stroke}
          fill="none"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="white"
          strokeWidth={stroke}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(.2,.8,.2,1)' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center text-white">
        <span className="text-xl font-semibold leading-none">{Math.round(pct)}%</span>
        <span className="text-[10px] uppercase tracking-wider opacity-80 mt-1">Complete</span>
      </div>
    </div>
  );
};

const DotGrid = () => (
  <svg
    aria-hidden="true"
    className="absolute inset-0 h-full w-full opacity-20 pointer-events-none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <defs>
      <pattern id="dotgrid" width="20" height="20" patternUnits="userSpaceOnUse">
        <circle cx="2" cy="2" r="1.2" fill="white" />
      </pattern>
    </defs>
    <rect width="100%" height="100%" fill="url(#dotgrid)" />
  </svg>
);

const CourseHero = ({ course, courseId, instructors, progress }) => {
  const hue = useMemo(() => hashHue(courseId), [courseId]);
  const gradient = `linear-gradient(135deg, hsl(${hue} 70% 42%) 0%, hsl(${(hue + 40) % 360} 70% 32%) 60%, hsl(${(hue + 80) % 360} 65% 28%) 100%)`;
  const term = course?.term?.name || course?.enrollment_term_id ? (course?.term?.name || `Term ${course?.enrollment_term_id ?? ''}`).trim() : null;
  const teachers = instructors?.length ? instructors : [];
  const teacherNames = teachers.slice(0, 2).map((t) => t.user?.name || t.name).filter(Boolean).join(', ');
  const extraTeachers = Math.max(0, teachers.length - 2);

  return (
    <Card className="relative h-48 overflow-hidden border-0 text-white rounded-card shadow-md">
      <div className="absolute inset-0" style={{ background: gradient }} />
      <DotGrid />
      <div className="relative z-10 flex h-full items-center justify-between gap-6 p-6">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 mb-2">
            {course?.course_code && (
              <span className="inline-flex items-center rounded-pill bg-surface-0/20 px-2.5 py-0.5 text-xs font-medium backdrop-blur-sm">
                {course.course_code}
              </span>
            )}
            {term && (
              <span className="inline-flex items-center rounded-pill bg-surface-0/15 px-2.5 py-0.5 text-xs font-medium backdrop-blur-sm">
                {term}
              </span>
            )}
          </div>
          <h1 className="font-display text-3xl md:text-4xl font-bold leading-tight tracking-tight truncate">
            {course?.name || 'Course'}
          </h1>
          {teachers.length > 0 && (
            <div className="mt-3 flex items-center gap-3">
              <div className="flex -space-x-2">
                {teachers.slice(0, 3).map((t, i) => {
                  const name = t.user?.name || t.name || 'Instructor';
                  const avatar = t.user?.avatar_url || t.avatar_url;
                  return (
                    <Avatar key={t.id ?? i} className="h-7 w-7 ring-2 ring-white/70">
                      {avatar && <AvatarImage src={avatar} alt={name} />}
                      <AvatarFallback className="bg-surface-0/30 text-white text-[10px] font-semibold">
                        {initialsOf(name)}
                      </AvatarFallback>
                    </Avatar>
                  );
                })}
              </div>
              <p className="text-sm text-white/90 truncate">
                Taught by {teacherNames || 'your instructor'}
                {extraTeachers > 0 && ` +${extraTeachers} more`}
              </p>
            </div>
          )}
        </div>
        <div className="hidden sm:block shrink-0">
          <ProgressRing value={progress} />
        </div>
      </div>
    </Card>
  );
};

const QuickActionRow = ({ courseId, isTeacher }) => {
  const actions = useMemo(() => {
    const base = [
      { label: 'Modules', icon: LayoutIcon, to: `/courses/${courseId}/modules` },
      { label: 'Assignments', icon: PenTool, to: `/courses/${courseId}/assignments` },
      { label: 'Gradebook', icon: GraduationCap, to: `/courses/${courseId}/gradebook` },
      { label: 'Discussions', icon: MessagesSquare, to: `/courses/${courseId}/discussions` },
      { label: 'Announcements', icon: Megaphone, to: `/courses/${courseId}/announcements` },
    ];
    if (isTeacher) {
      base.push({ label: 'Settings', icon: Settings, to: `/courses/${courseId}/settings` });
    }
    return base;
  }, [courseId, isTeacher]);

  return (
    <div className="flex flex-wrap gap-2" role="navigation" aria-label="Course quick actions">
      {actions.map(({ label, icon: Icon, to }) => (
        <Button
          key={label}
          variant="outline"
          size="sm"
          asChild
          className="gap-2 rounded-pill"
        >
          <Link to={to} aria-label={label}>
            <Icon className="h-4 w-4" />
            <span>{label}</span>
          </Link>
        </Button>
      ))}
    </div>
  );
};

const UpNextList = ({ courseId, items, loading }) => {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <CalendarClock className="h-5 w-5 text-brand-600" /> Up Next
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex items-center gap-3">
              <Skeleton className="h-9 w-9 rounded-md" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/3" />
              </div>
              <Skeleton className="h-6 w-16 rounded-pill" />
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle className="text-lg flex items-center gap-2">
          <CalendarClock className="h-5 w-5 text-brand-600" /> Up Next
        </CardTitle>
        <Button variant="link" size="sm" asChild className="h-auto p-0 text-xs">
          <Link to={`/courses/${courseId}/assignments`}>View all</Link>
        </Button>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-10 text-center">
            <div className="rounded-full bg-brand-50 p-3 mb-3">
              <Sparkles className="h-6 w-6 text-brand-600" aria-hidden="true" />
            </div>
            <p className="text-sm font-medium text-text-primary">You're all caught up</p>
            <p className="text-xs text-text-tertiary mt-1">
              Nothing due in the next 30 days.
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-border-subtle">
            {items.map((it) => {
              const Icon = it.kind === 'quiz' ? HelpCircle : PenTool;
              const days = daysUntil(it.due_at);
              return (
                <li key={`${it.kind}-${it.id}`}>
                  <Link
                    to={it.link}
                    className="flex items-center gap-3 py-3 -mx-2 px-2 rounded-control hover:bg-surface-1 transition-colors"
                  >
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-brand-50 text-brand-700">
                      <Icon className="h-4 w-4" aria-hidden="true" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-text-primary truncate">{it.title}</p>
                      <p className="text-xs text-text-tertiary truncate">
                        {it.kind === 'quiz' ? 'Quiz' : 'Assignment'}
                        {it.points_possible != null && ` · ${it.points_possible} pts`}
                      </p>
                    </div>
                    <Badge variant={urgencyVariant(days)} className="shrink-0 rounded-pill">
                      {formatDueLabel(days)}
                    </Badge>
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

const ActivityFeed = ({ items, loading }) => {
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Activity className="h-5 w-5 text-brand-600" /> Recent Activity
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex items-start gap-3">
              <Skeleton className="h-8 w-8 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-3 w-full" />
                <Skeleton className="h-3 w-2/3" />
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Activity className="h-5 w-5 text-brand-600" /> Recent Activity
        </CardTitle>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-text-tertiary">No recent activity yet.</p>
          </div>
        ) : (
          <ul className="space-y-4">
            {items.map((it, i) => (
              <li key={`${it.kind}-${it.id ?? i}`} className="flex items-start gap-3">
                <Avatar className="h-8 w-8 shrink-0">
                  <AvatarFallback
                    className={cn(
                      'text-[10px] font-semibold',
                      it.kind === 'announcement' && 'bg-brand-100 text-brand-700',
                      it.kind === 'page' && 'bg-accent-info/10 text-accent-info',
                      it.kind === 'grade' && 'bg-accent-success/10 text-accent-success'
                    )}
                  >
                    {initialsOf(it.actor || it.title)}
                  </AvatarFallback>
                </Avatar>
                <div className="min-w-0 flex-1">
                  {it.link ? (
                    <Link to={it.link} className="text-sm text-text-primary hover:underline line-clamp-2">
                      <span className="font-medium">{it.actor || 'Update'}</span>{' '}
                      <span className="text-text-secondary">{it.verb}</span>{' '}
                      <span className="font-medium">{it.title}</span>
                    </Link>
                  ) : (
                    <p className="text-sm text-text-primary line-clamp-2">
                      <span className="font-medium">{it.actor || 'Update'}</span>{' '}
                      <span className="text-text-secondary">{it.verb}</span>{' '}
                      <span className="font-medium">{it.title}</span>
                    </p>
                  )}
                  {it.timestamp && (
                    <p className="text-xs text-text-tertiary mt-0.5">{it.timestamp}</p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
};

const SetupChecklistCard = ({ courseId, items, completedCount, totalCount }) => {
  return (
    <Card className="border-brand-200 bg-brand-50/60">
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Megaphone className="h-5 w-5 text-brand-600" aria-hidden="true" />
            <CardTitle className="text-lg text-brand-900">Get your course ready</CardTitle>
          </div>
          <Badge variant="secondary" className="bg-surface-0 text-brand-700 rounded-pill">
            {completedCount}/{totalCount}
          </Badge>
        </div>
        <CardDescription className="text-brand-700">
          Finish these steps so students have a great first day.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="grid gap-2 sm:grid-cols-2">
          {items.map((item) => {
            const Icon = item.icon;
            return (
              <li key={item.label}>
                <Link
                  to={item.link}
                  className={cn(
                    'flex items-center gap-3 rounded-control border px-3 py-2.5 transition-colors',
                    item.done
                      ? 'border-accent-success/30 bg-surface-0/60 text-text-secondary'
                      : 'border-brand-200 bg-surface-0 hover:bg-brand-100/60 text-text-primary'
                  )}
                >
                  {item.done ? (
                    <CheckCircle2 className="h-5 w-5 shrink-0 text-accent-success" aria-hidden="true" />
                  ) : (
                    <Circle className="h-5 w-5 shrink-0 text-brand-300" aria-hidden="true" />
                  )}
                  <Icon className="h-4 w-4 shrink-0 text-brand-600" aria-hidden="true" />
                  <span className={cn('text-sm flex-1', item.done && 'line-through')}>
                    {item.label}
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      </CardContent>
    </Card>
  );
};

const CourseLandingSkeleton = () => (
  <div className="space-y-6" aria-busy="true" aria-label="Loading course">
    <Skeleton className="h-48 w-full rounded-card" />
    <div className="flex flex-wrap gap-2">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-9 w-28 rounded-pill" />
      ))}
    </div>
    <div className="grid gap-6 md:grid-cols-3">
      <div className="md:col-span-2 space-y-3">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-72 w-full rounded-card" />
      </div>
      <div className="space-y-3">
        <Skeleton className="h-7 w-40" />
        <Skeleton className="h-72 w-full rounded-card" />
      </div>
    </div>
  </div>
);

const ModuleList = ({ courseId, modules }) => {
  const [expandedModules, setExpandedModules] = useState({});

  useEffect(() => {
    const expanded = {};
    modules.forEach((m) => {
      expanded[m.id] = true;
    });
    setExpandedModules(expanded);
  }, [modules]);

  const toggleModule = (moduleId) => {
    setExpandedModules((prev) => ({ ...prev, [moduleId]: !prev[moduleId] }));
  };

  const getItemIcon = (type) => {
    const Icon = ITEM_ICONS[type] || Book;
    return <Icon className="w-4 h-4 text-text-tertiary" />;
  };

  const formatDueDate = (dateStr) => {
    if (!dateStr) return null;
    const date = new Date(dateStr);
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  };

  const getItemLink = (item) => {
    if (item.type === 'Assignment' && item.content_id) {
      return `/courses/${courseId}/assignments/${item.content_id}`;
    }
    if (item.type === 'Quiz' && item.content_id) {
      return `/courses/${courseId}/quizzes/${item.content_id}/take`;
    }
    if (item.type === 'Page' && item.content_id) {
      return `/courses/${courseId}/pages/${item.content_id}`;
    }
    if (item.type === 'Discussion' && item.content_id) {
      return `/courses/${courseId}/discussions/${item.content_id}`;
    }
    if (item.type === 'ExternalUrl' && item.url) {
      return item.url;
    }
    return null;
  };

  const renderModuleItem = (item) => {
    const isAssignment = item.type === 'Assignment';
    const link = getItemLink(item);
    const isExternal = item.type === 'ExternalUrl';
    const content = (
      <>
        {getItemIcon(item.type)}
        <span className="text-sm flex-1">{item.title}</span>
        {isAssignment && item.content_details && (
          <div className="flex items-center space-x-3 text-xs text-text-disabled">
            {item.content_details.points_possible !== undefined && (
              <span className="flex items-center space-x-1">
                <Award className="w-3 h-3" />
                <span>{item.content_details.points_possible} pts</span>
              </span>
            )}
            {item.content_details.due_at && (
              <span className="flex items-center space-x-1">
                <Calendar className="w-3 h-3" />
                <span>Due {formatDueDate(item.content_details.due_at)}</span>
              </span>
            )}
          </div>
        )}
      </>
    );

    if (link) {
      return isExternal ? (
        <a
          key={item.id}
          href={link}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center space-x-3 py-2 px-6 hover:bg-surface-2"
          style={{ paddingLeft: `${1.5 + item.indent * 1.5}rem` }}
        >
          {content}
        </a>
      ) : (
        <Link
          key={item.id}
          to={link}
          className="flex items-center space-x-3 py-2 px-6 hover:bg-surface-2"
          style={{ paddingLeft: `${1.5 + item.indent * 1.5}rem` }}
        >
          {content}
        </Link>
      );
    }

    return (
      <div
        key={item.id}
        className="flex items-center space-x-3 py-2 px-6 hover:bg-surface-2"
        style={{ paddingLeft: `${1.5 + item.indent * 1.5}rem` }}
      >
        {content}
      </div>
    );
  };

  return (
    <Card className="overflow-hidden">
      <CardHeader className="py-4">
        <CardTitle className="text-lg flex items-center gap-2">
          <ListChecks className="h-5 w-5 text-brand-600" /> Modules
        </CardTitle>
      </CardHeader>
      <Separator />
      {modules.length === 0 ? (
        <CardContent className="py-10 text-center text-text-tertiary">
          No modules yet.
        </CardContent>
      ) : (
        <div className="divide-y">
          {modules.map((module) => (
            <div key={module.id}>
              <button
                className="w-full px-4 py-3 flex items-center justify-between hover:bg-surface-1"
                onClick={() => toggleModule(module.id)}
                aria-expanded={!!expandedModules[module.id]}
                aria-label={`Toggle ${module.name}`}
              >
                <span className="font-medium">{module.name}</span>
                {expandedModules[module.id] ? (
                  <ChevronDown className="w-5 h-5 text-text-disabled" />
                ) : (
                  <ChevronRight className="w-5 h-5 text-text-disabled" />
                )}
              </button>

              {expandedModules[module.id] && module.items && (
                <div className="bg-surface-1 border-t">
                  {module.items.map((item) => renderModuleItem(item))}
                  {module.items.length === 0 && (
                    <div className="py-3 px-6 text-sm text-text-disabled">No items</div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
};

const useCourseLandingData = (courseId) => {
  const [state, setState] = useState({
    assignments: null,
    quizzes: null,
    announcements: null,
    pages: null,
    enrollments: null,
  });

  useEffect(() => {
    let cancelled = false;
    Promise.allSettled([
      api.getAssignments(courseId, 1, 50),
      api.getQuizzes(courseId, 1, 50),
      api.getCourseAnnouncements(courseId, 1, 5),
      api.getPages(courseId, 1, 5),
      api.getEnrollments(courseId, 1, 50),
    ]).then((results) => {
      if (cancelled) return;
      const [a, q, an, p, e] = results;
      setState({
        assignments: a.status === 'fulfilled' ? a.value?.data || [] : [],
        quizzes: q.status === 'fulfilled' ? q.value?.data || [] : [],
        announcements: an.status === 'fulfilled' ? an.value?.data || [] : [],
        pages: p.status === 'fulfilled' ? p.value?.data || [] : [],
        enrollments: e.status === 'fulfilled' ? e.value?.data || [] : [],
      });
    });
    return () => {
      cancelled = true;
    };
  }, [courseId]);

  return state;
};

const CoursePage = () => {
  const { courseId } = useParams();
  const { user } = useAuth(); // eslint-disable-line no-unused-vars
  const isTeacher = useIsTeacher(courseId);
  const { isSimplified } = useCourseUI();
  const [course, setCourse] = useState(null);
  const [modules, setModules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useDocumentTitle(course?.name);

  useEffect(() => {
    let cancelled = false;
    const fetchData = async () => {
      try {
        const [courseData, modulesResult] = await Promise.all([
          api.getCourse(courseId),
          api.getModules(courseId, 1, 100, true),
        ]);
        if (cancelled) return;
        setCourse(courseData);
        setModules(modulesResult.data || []);
      } catch (err) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    fetchData();
    return () => {
      cancelled = true;
    };
  }, [courseId]);

  const landing = useCourseLandingData(courseId);
  const landingLoading =
    landing.assignments === null ||
    landing.quizzes === null ||
    landing.announcements === null ||
    landing.pages === null;

  const upNext = useMemo(() => {
    if (landingLoading) return [];
    const horizon = 30 * 86_400_000;
    const now = Date.now();
    const fromAssignments = (landing.assignments || [])
      .filter((a) => a.due_at)
      .map((a) => ({
        kind: 'assignment',
        id: a.id,
        title: a.name || a.title,
        due_at: a.due_at,
        points_possible: a.points_possible,
        link: `/courses/${courseId}/assignments/${a.id}`,
      }));
    const fromQuizzes = (landing.quizzes || [])
      .filter((q) => q.due_at)
      .map((q) => ({
        kind: 'quiz',
        id: q.id,
        title: q.title,
        due_at: q.due_at,
        points_possible: q.points_possible,
        link: `/courses/${courseId}/quizzes/${q.id}/take`,
      }));
    return [...fromAssignments, ...fromQuizzes]
      .filter((it) => {
        const t = new Date(it.due_at).getTime();
        return !Number.isNaN(t) && t - now <= horizon && t - now >= -7 * 86_400_000;
      })
      .sort((a, b) => new Date(a.due_at) - new Date(b.due_at))
      .slice(0, 6);
  }, [landingLoading, landing.assignments, landing.quizzes, courseId]);

  const activity = useMemo(() => {
    if (landingLoading) return [];
    const formatDate = (d) => {
      if (!d) return null;
      const dt = new Date(d);
      if (Number.isNaN(dt.getTime())) return null;
      const diff = Math.round((Date.now() - dt.getTime()) / 86_400_000);
      if (diff < 1) return 'Today';
      if (diff === 1) return 'Yesterday';
      if (diff < 7) return `${diff}d ago`;
      return dt.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
    };
    const announcementItems = (landing.announcements || []).slice(0, 4).map((a) => ({
      kind: 'announcement',
      id: a.id,
      actor: a.author?.display_name || a.user_name || 'Instructor',
      verb: 'posted',
      title: a.title,
      timestamp: formatDate(a.posted_at || a.created_at),
      sortAt: new Date(a.posted_at || a.created_at || 0).getTime(),
      link: `/courses/${courseId}/announcements/${a.id}`,
    }));
    const pageItems = (landing.pages || []).slice(0, 3).map((p) => ({
      kind: 'page',
      id: p.page_id || p.id || p.url,
      actor: p.last_edited_by?.display_name || 'Page',
      verb: 'updated',
      title: p.title,
      timestamp: formatDate(p.updated_at),
      sortAt: new Date(p.updated_at || 0).getTime(),
      link: p.url ? `/courses/${courseId}/pages/${p.url}` : null,
    }));
    return [...announcementItems, ...pageItems]
      .filter((it) => it.title)
      .sort((a, b) => b.sortAt - a.sortAt)
      .slice(0, 6);
  }, [landingLoading, landing.announcements, landing.pages, courseId]);

  const instructors = useMemo(() => {
    if (!landing.enrollments) return [];
    return landing.enrollments.filter(
      (e) => e.type === 'TeacherEnrollment' || e.enrollment_type === 'teacher' || e.role === 'TeacherEnrollment'
    );
  }, [landing.enrollments]);

  const checklistItems = useMemo(() => {
    const hasModules = modules && modules.length > 0;
    const hasAssignments = (landing.assignments || []).length > 0;
    const hasStudents = (landing.enrollments || []).some(
      (e) => e.type === 'StudentEnrollment' || e.enrollment_type === 'student'
    );
    const hasSyllabus = !!course?.syllabus_body;
    return [
      { label: 'Create your first module', done: hasModules, link: `/courses/${courseId}/modules`, icon: LayoutIcon },
      { label: 'Add an assignment', done: hasAssignments, link: `/courses/${courseId}/assignments`, icon: PenTool },
      { label: 'Enroll students', done: hasStudents, link: `/courses/${courseId}/people`, icon: Users },
      { label: 'Add syllabus content', done: hasSyllabus, link: `/courses/${courseId}/syllabus`, icon: FileText },
      { label: 'Configure course settings', done: false, link: `/courses/${courseId}/settings`, icon: Settings },
    ];
  }, [modules, landing.assignments, landing.enrollments, course?.syllabus_body, courseId]);

  const completedCount = checklistItems.filter((i) => i.done).length;
  const setupComplete = completedCount >= 4;
  const showSetup = isTeacher && !setupComplete && !landingLoading;

  const courseProgress = useMemo(() => {
    if (typeof course?.progress === 'number') return course.progress;
    const total = (landing.assignments || []).length;
    if (!total) return 0;
    return 0;
  }, [course?.progress, landing.assignments]);

  if (loading) {
    return (
      <Layout>
        <CourseLandingSkeleton />
      </Layout>
    );
  }
  if (error) {
    return (
      <Layout>
        <Card className="mx-auto max-w-md">
          <CardContent className="py-10 text-center">
            <p className="text-accent-danger mb-3">{error}</p>
            <Button variant="outline" onClick={() => window.location.reload()}>
              Try Again
            </Button>
          </CardContent>
        </Card>
      </Layout>
    );
  }
  if (!course) {
    return (
      <Layout>
        <Card className="mx-auto max-w-md">
          <CardContent className="py-10 text-center text-text-secondary">
            Course not found
          </CardContent>
        </Card>
      </Layout>
    );
  }

  const defaultView = course.default_view || 'modules';

  if (isSimplified && defaultView === 'home_engine') {
    return (
      <K2Layout>
        <HomeEngine />
      </K2Layout>
    );
  }

  const renderMainContent = () => {
    switch (defaultView) {
      case 'home_engine':
        return <HomeEngine />;
      case 'syllabus':
        return course.syllabus_body ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <FileText className="h-5 w-5 text-brand-600" /> Syllabus
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div
                className="prose max-w-none text-text-primary"
                dangerouslySetInnerHTML={{ __html: sanitizeHTML(course.syllabus_body) }}
              />
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="py-10 text-center text-text-tertiary">
              No syllabus content.
            </CardContent>
          </Card>
        );
      case 'modules':
      default:
        return (
          <div className="space-y-6">
            {course.syllabus_body && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <FileText className="h-5 w-5 text-brand-600" /> Syllabus
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div
                    className="prose max-w-none text-text-primary"
                    dangerouslySetInnerHTML={{ __html: sanitizeHTML(course.syllabus_body) }}
                  />
                </CardContent>
              </Card>
            )}
            <ModuleList courseId={courseId} modules={modules} />
          </div>
        );
    }
  };

  return (
    <Layout>
      <div className="space-y-6">
        <CourseHero
          course={course}
          courseId={courseId}
          instructors={instructors}
          progress={courseProgress}
        />

        <QuickActionRow courseId={courseId} isTeacher={isTeacher} />

        <CourseNav />

        {showSetup && (
          <SetupChecklistCard
            courseId={courseId}
            items={checklistItems}
            completedCount={completedCount}
            totalCount={checklistItems.length - 1}
          />
        )}

        <div className="grid gap-6 md:grid-cols-3">
          <div className="md:col-span-2 space-y-6">
            <UpNextList courseId={courseId} items={upNext} loading={landingLoading} />
            {renderMainContent()}
          </div>
          <div className="space-y-6">
            <ActivityFeed items={activity} loading={landingLoading} />
          </div>
        </div>
      </div>
    </Layout>
  );
};

export default CoursePage;
