import React, { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Clock,
  Megaphone,
  GraduationCap,
  BookOpen,
  MoreHorizontal,
  Star,
  Palette,
  ArrowRight,
  ArrowLeft,
  Sparkles,
  MessageSquare,
  MessagesSquare,
  AlertTriangle,
  ListTodo,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import Layout from '../components/Layout';
import CourseInvitationsBanner from '../components/enrollments/CourseInvitationsBanner';
import { useAuth } from '../contexts/AuthContext';
import useDocumentTitle from '../hooks/useDocumentTitle';
import TimeZoneNotice from '../components/learner/TimeZoneNotice';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Separator } from '@/components/ui/separator';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  learnerApi, useDashboardCards, usePlannerItems, useActivityStream, useMissingSubmissions, useLearnerMutation,
} from '../services/api_learner';
import { isDone, itemTitle, itemKey, windowRange } from '../utils/planner';
import { dayKeyInZone } from '../lib/userTimeZone';

// Student dashboard (G144 / G145): course cards with the user's
// favorites, nicknames, colors and order; the "coming up" list from the
// planner; missing work; and the recent activity stream.

const hashHue = (id) => {
  const s = String(Number(id) || 0);
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return Math.abs(h) % 360;
};

export const cardColor = (card) => card?.color || `hsl(${hashHue(card?.id)} 70% 45%)`;

const getSeason = (date = new Date()) => {
  const m = date.getMonth();
  if (m <= 1 || m === 11) return 'Winter';
  if (m <= 4) return 'Spring';
  if (m <= 7) return 'Summer';
  return 'Fall';
};

const getGreeting = (date = new Date()) => {
  const h = Number(date.toLocaleTimeString('en-US', { hour: 'numeric', hourCycle: 'h23' }));
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
};

const formatDue = (iso) => {
  const d = new Date(iso);
  if (dayKeyInZone(d) === dayKeyInZone(new Date())) {
    return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  }
  return d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
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

const HeroGreeting = ({ firstName }) => {
  const today = new Date();
  const dateStr = today.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
  return (
    <header className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">
          {getGreeting()}, {firstName || 'there'}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">Here is what is happening across your courses.</p>
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

const ComingUpPanel = ({ items, loading, missingCount, onToggle }) => (
  <Card className="overflow-hidden">
    <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
      <div className="flex items-center gap-2">
        <Clock className="h-4 w-4 text-primary" aria-hidden="true" />
        <CardTitle className="text-base font-semibold">Coming up</CardTitle>
      </div>
      <div className="flex items-center gap-2">
        {missingCount > 0 && (
          <Link to="/planner" className="inline-flex items-center gap-1 rounded-full bg-accent-danger/10 px-2 py-0.5 text-xs font-medium text-accent-danger">
            <AlertTriangle className="h-3 w-3" aria-hidden="true" /> {missingCount} missing
          </Link>
        )}
        <Button asChild variant="link" size="sm" className="h-auto p-0">
          <Link to="/planner"><ListTodo className="me-1 h-3.5 w-3.5" aria-hidden="true" />To do</Link>
        </Button>
      </div>
    </CardHeader>
    <Separator />
    <CardContent className="p-0">
      {loading ? (
        <div className="space-y-3 p-4">
          {[0, 1, 2].map((i) => <Skeleton key={i} className="h-10 w-full" />)}
        </div>
      ) : items.length === 0 ? (
        <div className="px-6 py-10 text-center">
          <p className="text-sm text-muted-foreground">You are all caught up for the next week.</p>
        </div>
      ) : (
        <ul className="divide-y">
          {items.map((item) => {
            const title = itemTitle(item);
            return (
              <li key={itemKey(item)} className="flex items-center gap-3 px-4 py-3">
                <input
                  type="checkbox"
                  className="h-4 w-4"
                  checked={isDone(item)}
                  onChange={() => onToggle(item, !isDone(item))}
                  aria-label={`Mark done: ${title}`}
                />
                <div className="min-w-0 flex-1">
                  <Link to={item.html_url || '/planner'} className="block truncate text-sm font-medium text-foreground hover:underline">
                    {title}
                  </Link>
                  <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
                    <span className="truncate">{item.context_name}</span>
                    <span aria-hidden="true">·</span>
                    <span className="shrink-0">{formatDue(item.plannable_date)}</span>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </CardContent>
  </Card>
);

function CustomizeDialog({ card, onClose }) {
  const [nickname, setNickname] = useState(card.nickname || '');
  const [color, setColor] = useState(card.color || '#2563eb');
  const [error, setError] = useState('');
  const save = useLearnerMutation(async () => {
    await learnerApi.setNickname(card.id, nickname.trim());
    await learnerApi.setColor(card.id, color);
  }, { onSuccess: onClose, onError: (e) => setError(e.message) });
  const reset = useLearnerMutation(async () => {
    await learnerApi.setNickname(card.id, '');
    await learnerApi.setColor(card.id, '');
  }, { onSuccess: onClose, onError: (e) => setError(e.message) });
  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Customize {card.originalName}</DialogTitle>
          <DialogDescription>Only you see the nickname and color.</DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          <div>
            <label htmlFor="card-nickname" className="mb-1 block text-sm font-medium">Nickname</label>
            <input id="card-nickname" value={nickname} maxLength={60} onChange={(e) => setNickname(e.target.value)}
              placeholder={card.originalName}
              className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
          </div>
          <div>
            <label htmlFor="card-color" className="mb-1 block text-sm font-medium">Color</label>
            <input id="card-color" type="color" value={color} onChange={(e) => setColor(e.target.value)}
              className="h-9 w-16 rounded border border-border-default" />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => reset.mutate()}>Reset</Button>
            <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={save.isPending}>Save</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

const CourseTile = ({ card, nextItem, dueCount, onFavorite, onCustomize, onMove, isFirst, isLast }) => {
  const color = cardColor(card);
  return (
    <Card className="h-full overflow-hidden transition-all hover:shadow-md">
      <div className="relative h-24 w-full" style={{ background: color }}>
        <div className="absolute inset-0 bg-gradient-to-t from-black/20 to-transparent" aria-hidden="true" />
        {card.isFavorited && (
          <Star className="absolute start-2 top-2 h-4 w-4 fill-white text-white" aria-label="Favorite" />
        )}
        <div className="absolute end-2 top-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`Options for ${card.shortName}`}
                className="h-7 w-7 bg-surface-0/20 text-white backdrop-blur hover:bg-surface-0/30 hover:text-white"
              >
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => onFavorite(card, !card.isFavorited)}>
                <Star className="h-4 w-4" />
                {card.isFavorited ? 'Remove from favorites' : 'Add to favorites'}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => onCustomize(card)}>
                <Palette className="h-4 w-4" />
                Nickname and color
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled={isFirst} onSelect={() => onMove(card, -1)}>
                <ArrowLeft className="h-4 w-4" />
                Move earlier
              </DropdownMenuItem>
              <DropdownMenuItem disabled={isLast} onSelect={() => onMove(card, 1)}>
                <ArrowRight className="h-4 w-4" />
                Move later
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      <CardContent className="p-4">
        <Link to={card.href} className="group block rounded focus:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            {card.courseCode || 'Course'}{card.term ? ` · ${card.term}` : ''}
          </div>
          <h3 className="mt-0.5 line-clamp-2 text-sm font-semibold text-foreground group-hover:underline">{card.shortName}</h3>
          {card.nickname && <div className="truncate text-xs text-muted-foreground">{card.originalName}</div>}
        </Link>
        <div className="mt-4 flex items-center justify-between gap-2">
          <span className="text-xs text-muted-foreground">
            {dueCount > 0 ? `${dueCount} due this week` : 'Nothing due this week'}
          </span>
          {nextItem && (
            <Badge variant="secondary" className="max-w-[160px] truncate rounded-full font-normal" title={itemTitle(nextItem)}>
              Next: {itemTitle(nextItem)}
            </Badge>
          )}
        </div>
      </CardContent>
    </Card>
  );
};

const ACTIVITY_ICONS = {
  Announcement: Megaphone,
  DiscussionTopic: MessagesSquare,
  Submission: GraduationCap,
  SubmissionComment: MessageSquare,
};

const ActivityRail = ({ items, loading }) => (
  <Card>
    <CardHeader className="pb-3">
      <CardTitle className="text-base font-semibold">Recent activity</CardTitle>
    </CardHeader>
    <Separator />
    <CardContent className="p-0">
      {loading ? (
        <div className="space-y-4 p-4">
          {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-8 w-full" />)}
        </div>
      ) : items.length === 0 ? (
        <div className="px-3 py-8 text-center text-sm text-muted-foreground">No recent activity yet.</div>
      ) : (
        <ul className="space-y-1 p-2">
          {items.map((it) => {
            const Icon = ACTIVITY_ICONS[it.type] || BookOpen;
            let detail = it.context_name;
            if (it.type === 'Submission') {
              detail += it.excused ? ' · Excused' : it.score != null ? ` · ${it.score}${it.points_possible != null ? `/${it.points_possible}` : ''} pts` : '';
            }
            if (it.type === 'SubmissionComment' && it.author_name) detail += ` · ${it.author_name} commented`;
            return (
              <li key={it.id}>
                <Link to={it.html_url} className="flex items-start gap-3 rounded-md px-2 py-2 transition-colors hover:bg-accent/40">
                  <Avatar className="h-8 w-8">
                    <AvatarFallback className="bg-primary/10 text-primary"><Icon className="h-4 w-4" aria-hidden="true" /></AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium text-foreground">{it.title}</div>
                    <div className="truncate text-xs text-muted-foreground">{detail} · {formatRelative(it.created_at)}</div>
                  </div>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </CardContent>
  </Card>
);

const EmptyDashboard = () => {
  const { t } = useTranslation();
  return (
    <Card className="mx-auto max-w-xl text-center">
      <CardContent className="space-y-4 px-6 py-12">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
          <BookOpen className="h-6 w-6" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-foreground">{t('dashboardPage.welcomeToPaperLms')}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t('dashboardPage.notEnrolledYet')}</p>
        </div>
        <Button asChild>
          <Link to="/courses">{t('dashboardPage.browseCatalog')}</Link>
        </Button>
      </CardContent>
    </Card>
  );
};

const DashboardPage = () => {
  const { t } = useTranslation();
  useDocumentTitle(t('dashboard.title'));
  const { user } = useAuth();
  const [customizing, setCustomizing] = useState(null);
  const cardsQuery = useDashboardCards(false);
  const range = useMemo(() => windowRange(new Date(), 7), []);
  const planner = usePlannerItems({ startDate: range.start, endDate: range.end });
  const activity = useActivityStream({ perPage: 12 });
  const missing = useMissingSubmissions('self');

  const favorite = useLearnerMutation(({ card, fav }) => learnerApi.setFavorite(card.id, fav));
  const toggleDone = useLearnerMutation(({ item, done }) =>
    learnerApi.setPlannerOverride({ plannableType: item.plannable_type, plannableId: item.plannable_id, markedComplete: done }));
  const move = useLearnerMutation((order) => learnerApi.setPositions(order));

  // Accepting a course invitation (G122) adds a course: reload what lists it.
  const reloadAfterInvitation = () => {
    cardsQuery.refetch();
    planner.refetch();
    missing.refetch();
  };

  const cards = cardsQuery.data || [];
  const graded = useMemo(
    () => (planner.data || []).filter((it) => ['assignment', 'quiz', 'discussion_topic', 'planner_note'].includes(it.plannable_type)),
    [planner.data],
  );
  const upcoming = useMemo(() => graded.filter((it) => !isDone(it)).slice(0, 6), [graded]);
  const byCourse = useMemo(() => {
    const next = {};
    const count = {};
    graded.forEach((it) => {
      if (!it.course_id || isDone(it) || it.plannable_type === 'planner_note') return;
      count[it.course_id] = (count[it.course_id] || 0) + 1;
      if (!next[it.course_id]) next[it.course_id] = it;
    });
    return { next, count };
  }, [graded]);

  const onMove = (card, delta) => {
    const ids = cards.map((c) => c.id);
    const i = ids.indexOf(card.id);
    const j = i + delta;
    if (i < 0 || j < 0 || j >= ids.length) return;
    [ids[i], ids[j]] = [ids[j], ids[i]];
    move.mutate(ids);
  };

  const firstName = user?.first_name || user?.name?.split(' ')[0] || '';
  const missingCount = (missing.data || []).filter((m) => !m.dismissed).length;

  if (cardsQuery.isLoading) {
    return (
      <Layout>
        <div className="space-y-6">
          <Skeleton className="h-8 w-72" />
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {[0, 1, 2, 3, 4, 5].map((i) => <Skeleton key={i} className="h-44 w-full" />)}
          </div>
        </div>
      </Layout>
    );
  }

  if (cardsQuery.isError) {
    return (
      <Layout>
        <Card className="mx-auto max-w-md">
          <CardContent className="space-y-3 px-6 py-10 text-center">
            <p role="alert" className="text-sm text-destructive">{cardsQuery.error?.message || 'Could not load dashboard'}</p>
            <Button onClick={() => cardsQuery.refetch()} variant="outline">Try again</Button>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  if (cards.length === 0) {
    return (
      <Layout>
        <CourseInvitationsBanner onAccepted={reloadAfterInvitation} />
        <EmptyDashboard />
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="space-y-6">
        <HeroGreeting firstName={firstName} />
        <CourseInvitationsBanner onAccepted={reloadAfterInvitation} />

        <TimeZoneNotice />
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-6">
            <ComingUpPanel
              items={upcoming}
              loading={planner.isLoading}
              missingCount={missingCount}
              onToggle={(item, done) => toggleDone.mutate({ item, done })}
            />
            <section aria-labelledby="my-courses-heading">
              <div className="mb-3 flex items-center justify-between">
                <h2 id="my-courses-heading" className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                  {cards.some((c) => c.isFavorited) ? 'Favorite courses' : 'My courses'}
                </h2>
                <Button asChild variant="link" size="sm" className="h-auto p-0">
                  <Link to="/courses">All courses</Link>
                </Button>
              </div>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                {cards.map((card, i) => (
                  <CourseTile
                    key={card.id}
                    card={card}
                    nextItem={byCourse.next[card.id]}
                    dueCount={byCourse.count[card.id] || 0}
                    onFavorite={(c, fav) => favorite.mutate({ card: c, fav })}
                    onCustomize={setCustomizing}
                    onMove={onMove}
                    isFirst={i === 0}
                    isLast={i === cards.length - 1}
                  />
                ))}
              </div>
            </section>
          </div>
          <aside className="xl:sticky xl:top-6 xl:self-start" aria-label="Recent activity">
            <ActivityRail items={activity.data || []} loading={activity.isLoading} />
          </aside>
        </div>
      </div>
      {customizing && <CustomizeDialog card={customizing} onClose={() => setCustomizing(null)} />}
    </Layout>
  );
};

export default DashboardPage;
