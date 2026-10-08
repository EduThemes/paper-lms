import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  Calendar, Plus, Trash2, Edit2, ChevronLeft, ChevronRight, X, Rss, CalendarCheck, ExternalLink,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { calendarApi } from '../services/api_calendar';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { Skeleton } from '@/components/ui/skeleton';
import { fetchAllPages } from '../utils/fetchAllPages';
import {
  VIEWS, viewRange, shiftDate, describeRRule, startOfDay,
} from '../utils/calendarView';
import {
  MonthView, WeekView, AgendaView, CALENDAR_COLORS, ItemIcon, formatItemTime,
} from '../components/calendar/CalendarViews';
import CalendarEventForm from '../components/calendar/CalendarEventForm';
import CalendarFeedPanel from '../components/calendar/CalendarFeedPanel';

const VIEW_KEY = 'paper.calendar.view';
const HIDDEN_KEY = 'paper.calendar.hidden';

function readStored(key, fallback) {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? JSON.parse(raw) : fallback;
  } catch {
    return fallback;
  }
}

function writeStored(key, value) {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* storage unavailable: the choice just isn't remembered */
  }
}

// Legacy course events (an admin looking at a course they aren't
// enrolled in) shaped like aggregated items.
const legacyItem = (e) => ({
  ...e,
  id: `event_${e.id}`,
  type: 'event',
  calendar_event_id: e.id,
  can_edit: true,
});

// The aggregated calendar (G094/G146): personal, course, section, group
// and account events, assignment and quiz due dates and scheduler
// appointments across every calendar the user can see, in month, week
// and agenda views (G095), with a subscribable feed (G096).
const CalendarPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const courseCode = courseId ? `course_${courseId}` : null;

  const [view, setView] = useState(() => {
    const v = readStored(VIEW_KEY, 'month');
    return VIEWS.includes(v) ? v : 'month';
  });
  const [anchor, setAnchor] = useState(() => new Date());
  const [contexts, setContexts] = useState(null);
  const [hidden, setHidden] = useState(() => readStored(HIDDEN_KEY, []));
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selected, setSelected] = useState(null);
  const [form, setForm] = useState(null); // {event?, initialStart?}
  const [showFeed, setShowFeed] = useState(false);
  const [busy, setBusy] = useState(false);

  const range = useMemo(() => viewRange(view, anchor), [view, anchor]);

  useEffect(() => {
    let live = true;
    calendarApi.listContexts()
      .then((res) => { if (live) setContexts(res.data || []); })
      .catch(() => { if (live) setContexts([]); });
    return () => { live = false; };
  }, []);

  const colorIndex = useMemo(() => {
    const idx = {};
    (contexts || []).forEach((c, i) => { idx[c.asset_string] = i; });
    return idx;
  }, [contexts]);

  // In a course the calendar shows that course; an admin who isn't
  // enrolled (so it isn't one of their calendars) gets its events.
  const courseIsMine = Boolean(courseCode && (contexts || []).some((c) => c.asset_string === courseCode));
  const legacyCourse = Boolean(courseCode && contexts && !courseIsMine);

  const contextCodes = useMemo(() => {
    if (courseCode) return [courseCode];
    if (!contexts || hidden.length === 0) return undefined;
    return contexts.map((c) => c.asset_string).filter((code) => !hidden.includes(code));
  }, [courseCode, contexts, hidden]);

  const fetchItems = useCallback(async () => {
    if (courseCode && contexts === null) return; // wait to know which source
    setLoading(true);
    try {
      if (legacyCourse) {
        const all = await fetchAllPages((page) => api.getCourseCalendarEvents(courseId, page, 100));
        setItems(all.map(legacyItem).filter((e) => {
          const d = new Date(e.start_at);
          return d >= range.start && d < range.end;
        }));
      } else if (contextCodes && contextCodes.length === 0) {
        setItems([]);
      } else {
        const res = await calendarApi.listItems({
          start: range.start,
          end: new Date(range.end.getTime() - 1000),
          contextCodes,
        });
        setItems(res.data || []);
      }
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseCode, courseId, contexts, legacyCourse, contextCodes, range]);

  useEffect(() => { fetchItems(); }, [fetchItems]);

  const changeView = (v) => {
    setView(v);
    writeStored(VIEW_KEY, v);
  };

  const toggleCalendar = (code) => {
    setHidden((h) => {
      const next = h.includes(code) ? h.filter((c) => c !== code) : [...h, code];
      writeStored(HIDDEN_KEY, next);
      return next;
    });
  };

  const creatable = (contexts || []).filter((c) => c.can_create_calendar_events);
  const canCreate = courseCode
    ? creatable.some((c) => c.asset_string === courseCode) || legacyCourse
    : creatable.length > 0;

  const openNew = (day) => {
    if (!canCreate) return;
    setSelected(null);
    setForm({ initialStart: day || startOfDay(new Date()) });
  };

  const openEdit = async (item) => {
    try {
      // A section's event is edited through its course event.
      const id = item.parent_calendar_event_id || item.calendar_event_id;
      const res = await calendarApi.getEvent(id);
      setSelected(null);
      setForm({ event: res.data });
    } catch (err) {
      setError(err.message);
    }
  };

  const remove = async (item, which) => {
    const id = item.parent_calendar_event_id || item.calendar_event_id;
    if (!which && !window.confirm(t('calendarLane.deleteConfirm'))) return;
    setBusy(true);
    try {
      await calendarApi.deleteEvent(id, which);
      setSelected(null);
      await fetchItems();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const cancelReservation = async (item) => {
    if (!window.confirm(t('calendarLane.cancelReservationConfirm'))) return;
    setBusy(true);
    try {
      await calendarApi.cancelReservation(item.appointment_group_id, item.appointment_slot_id, item.reservation_id);
      setSelected(null);
      await fetchItems();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const handleDownload = async () => {
    try {
      const response = await fetch(api.getCalendarICalUrl(), { credentials: 'include' });
      if (!response.ok) throw new Error(t('calendarLane.exportFailed'));
      const blob = await response.blob();
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = 'calendar.ics';
      link.click();
      URL.revokeObjectURL(link.href);
    } catch (err) {
      setError(err.message);
    }
  };

  const heading = useMemo(() => {
    if (view === 'month') return anchor.toLocaleString(undefined, { month: 'long', year: 'numeric' });
    const last = new Date(range.end.getTime() - 1);
    const fmt = (d) => d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
    return `${fmt(range.start)} – ${fmt(last)}`;
  }, [view, anchor, range]);

  const viewProps = {
    items,
    range,
    anchor,
    colorIndex,
    onSelect: (item) => { setForm(null); setSelected(item); },
    onDayClick: canCreate ? openNew : undefined,
  };

  return (
    <Layout>
      {courseId && <CourseNav />}
      <div className="mb-6">
        {courseId && (
          <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
            &larr; {t('course.backToCourse')}
          </Link>
        )}
        <div className="flex flex-wrap items-center justify-between gap-2 mt-2">
          <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Calendar className="w-6 h-6" />
            {t('calendarPage.title')}
          </h2>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex rounded-md border border-border-strong overflow-hidden" role="group" aria-label={t('calendarLane.viewLabel')}>
              {VIEWS.map((v) => (
                <button
                  key={v}
                  type="button"
                  aria-pressed={view === v}
                  onClick={() => changeView(v)}
                  className={`px-3 py-1.5 text-sm border-s first:border-s-0 border-border-strong ${view === v ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-1'}`}
                >
                  {t(`calendarLane.views.${v}`)}
                </button>
              ))}
            </div>
            <Link
              to="/appointments"
              className="inline-flex items-center gap-1 border border-border-strong text-text-secondary px-3 py-1.5 rounded-md hover:bg-surface-1 text-sm font-medium"
            >
              <CalendarCheck className="w-4 h-4" />
              <span className="hidden sm:inline">{t('calendarLane.myAppointments')}</span>
            </Link>
            <button
              type="button"
              onClick={() => setShowFeed((s) => !s)}
              aria-expanded={showFeed}
              className="inline-flex items-center gap-1 border border-border-strong text-text-secondary px-3 py-1.5 rounded-md hover:bg-surface-1 text-sm font-medium"
            >
              <Rss className="w-4 h-4" />
              <span className="hidden sm:inline">{t('calendarLane.feed.button')}</span>
            </button>
            {canCreate && (
              <button
                type="button"
                onClick={() => (form ? setForm(null) : openNew())}
                className="inline-flex items-center gap-1 bg-brand-600 text-white px-3 py-1.5 rounded-md hover:bg-brand-700 text-sm font-medium"
              >
                {form ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
                <span>{form ? t('common.cancel') : t('calendarPage.newEvent')}</span>
              </button>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm" role="alert">
          {error}
          <button type="button" onClick={() => setError(null)} className="ms-2 font-bold" aria-label={t('common.close')}>&times;</button>
        </div>
      )}

      {showFeed && <CalendarFeedPanel onClose={() => setShowFeed(false)} onDownload={handleDownload} />}

      {form && (
        <CalendarEventForm
          key={form.event?.id || 'new'}
          contexts={legacyCourse
            ? [{ asset_string: courseCode, type: 'course', name: t('calendarLane.thisCourse'), can_create_calendar_events: true }]
            : (contexts || []).filter((c) => !courseCode || c.asset_string === courseCode)}
          defaultContextCode={courseCode || undefined}
          event={form.event}
          initialStart={form.initialStart}
          onCancel={() => setForm(null)}
          onSaved={() => { setForm(null); fetchItems(); }}
        />
      )}

      {selected && (
        <ItemDetail
          item={selected}
          busy={busy}
          onClose={() => setSelected(null)}
          onEdit={() => openEdit(selected)}
          onDelete={(which) => remove(selected, which)}
          onCancelReservation={() => cancelReservation(selected)}
        />
      )}

      <div className="flex flex-col lg:flex-row gap-4">
        {!courseId && contexts && contexts.length > 0 && (
          <aside className="lg:w-56 flex-shrink-0" aria-label={t('calendarLane.calendars')}>
            <h3 className="text-sm font-semibold text-text-secondary mb-2">{t('calendarLane.calendars')}</h3>
            <ul className="space-y-1">
              {contexts.map((c, i) => (
                <li key={c.asset_string}>
                  <label className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      checked={!hidden.includes(c.asset_string)}
                      onChange={() => toggleCalendar(c.asset_string)}
                      className="rounded border-border-strong"
                    />
                    <span className={`w-2.5 h-2.5 rounded-full ${CALENDAR_COLORS[i % CALENDAR_COLORS.length].dot}`} />
                    <span className="truncate">{c.type === 'user' ? t('calendarLane.personal') : c.name}</span>
                  </label>
                </li>
              ))}
            </ul>
          </aside>
        )}

        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <button type="button" onClick={() => setAnchor((a) => shiftDate(view, a, -1))} className="p-2 hover:bg-surface-2 rounded-md" aria-label={t('calendarLane.previous')}>
                <ChevronLeft className="w-5 h-5 text-text-secondary" />
              </button>
              <h3 className="text-lg font-semibold text-text-primary md:min-w-[12rem] text-center">{heading}</h3>
              <button type="button" onClick={() => setAnchor((a) => shiftDate(view, a, 1))} className="p-2 hover:bg-surface-2 rounded-md" aria-label={t('calendarLane.next')}>
                <ChevronRight className="w-5 h-5 text-text-secondary" />
              </button>
            </div>
            <button type="button" onClick={() => setAnchor(new Date())} className="text-sm text-brand-600 hover:text-brand-800 font-medium">
              {t('calendarPage.today')}
            </button>
          </div>

          {loading && items.length === 0 ? (
            <div className="space-y-3" aria-busy="true">
              <div className="grid grid-cols-7 gap-2">
                {Array.from({ length: 35 }).map((_, i) => <Skeleton key={i} className="h-16 rounded-md" />)}
              </div>
            </div>
          ) : (
            <>
              {view === 'month' && (
                <>
                  <div className="hidden md:block"><MonthView {...viewProps} /></div>
                  <div className="md:hidden"><AgendaView {...viewProps} /></div>
                </>
              )}
              {view === 'week' && <WeekView {...viewProps} />}
              {view === 'agenda' && <AgendaView {...viewProps} />}
            </>
          )}
        </div>
      </div>
    </Layout>
  );
};

const TYPE_LABEL_KEYS = {
  event: 'calendarLane.types.event',
  assignment: 'calendarLane.types.assignment',
  quiz: 'calendarLane.types.quiz',
  appointment: 'calendarLane.types.appointment',
  appointment_slot: 'calendarLane.types.appointmentSlot',
};

function ItemDetail({ item, busy, onClose, onEdit, onDelete, onCancelReservation }) {
  const { t } = useTranslation();
  const [askScope, setAskScope] = useState(false);
  const isEvent = item.type === 'event';
  const date = new Date(item.start_at).toLocaleDateString(undefined, {
    weekday: 'long', month: 'long', day: 'numeric', year: 'numeric',
  });
  return (
    <section className="bg-surface-0 rounded-lg shadow-lg border border-border-default p-4 mb-4" aria-labelledby="calendar-item-title">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs uppercase tracking-wide text-text-tertiary flex items-center gap-1">
            <ItemIcon type={item.type} /> {t(TYPE_LABEL_KEYS[item.type] || TYPE_LABEL_KEYS.event)}
            {item.context_name && <span>· {item.context_name}</span>}
            {item.section_name && <span>({item.section_name})</span>}
          </div>
          <h3 id="calendar-item-title" className="font-semibold text-text-primary mt-1">{item.title}</h3>
          <p className="text-sm text-text-tertiary mt-1">{date} · {formatItemTime(item, t)}</p>
          {item.rrule && <p className="text-sm text-text-tertiary mt-1">{describeRRule(item.rrule, t)}</p>}
          {item.points_possible != null && (
            <p className="text-sm text-text-tertiary mt-1">{t('calendarLane.points', { count: item.points_possible })}</p>
          )}
          {item.location_name && <p className="text-sm text-text-tertiary mt-1">{item.location_name}{item.location_address ? `, ${item.location_address}` : ''}</p>}
          {item.type === 'appointment_slot' && (
            <p className="text-sm text-text-tertiary mt-1">
              {t('calendarLane.slotFill', { count: item.reservation_count || 0, limit: item.participants_limit || 1 })}
              {item.reserved_by?.length > 0 && `: ${item.reserved_by.join(', ')}`}
            </p>
          )}
          {item.description && <p className="text-sm text-text-secondary mt-2 whitespace-pre-line">{item.description}</p>}
        </div>
        <button type="button" onClick={onClose} className="p-1 text-text-disabled hover:text-text-secondary" aria-label={t('common.close')}>
          <X className="w-4 h-4" />
        </button>
      </div>
      <div className="flex flex-wrap items-center gap-3 mt-3 pt-3 border-t">
        {item.html_url && !isEvent && (
          <Link to={item.html_url} className="inline-flex items-center gap-1 text-sm text-brand-600 hover:underline">
            <ExternalLink className="w-3.5 h-3.5" /> {t('calendarLane.open')}
          </Link>
        )}
        {item.type === 'appointment' && item.reservation_id && (
          <button type="button" disabled={busy} onClick={onCancelReservation} className="text-sm text-accent-danger hover:underline disabled:opacity-50">
            {t('calendarLane.cancelReservation')}
          </button>
        )}
        {isEvent && item.can_edit && (
          <>
            <button type="button" onClick={onEdit} className="inline-flex items-center gap-1 text-sm text-brand-600 hover:text-brand-800">
              <Edit2 className="w-3.5 h-3.5" /> {t('common.edit')}
            </button>
            {item.series_uuid ? (
              askScope ? (
                <span className="flex flex-wrap items-center gap-2 text-sm">
                  <span className="text-text-secondary">{t('calendarLane.scope.deleteWhich')}</span>
                  {['one', 'following', 'all'].map((w) => (
                    <button key={w} type="button" disabled={busy} onClick={() => onDelete(w)} className="px-2 py-1 rounded border border-accent-danger/40 text-accent-danger hover:bg-accent-danger/10 disabled:opacity-50">
                      {t(`calendarLane.scope.${w}`)}
                    </button>
                  ))}
                </span>
              ) : (
                <button type="button" onClick={() => setAskScope(true)} className="inline-flex items-center gap-1 text-sm text-accent-danger">
                  <Trash2 className="w-3.5 h-3.5" /> {t('common.delete')}
                </button>
              )
            ) : (
              <button type="button" disabled={busy} onClick={() => onDelete()} className="inline-flex items-center gap-1 text-sm text-accent-danger disabled:opacity-50">
                <Trash2 className="w-3.5 h-3.5" /> {t('common.delete')}
              </button>
            )}
          </>
        )}
      </div>
    </section>
  );
}

export default CalendarPage;
