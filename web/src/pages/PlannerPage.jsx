import React, { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronLeft, ChevronRight, Plus, Pencil, Trash2, AlertTriangle, CalendarDays } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import useDocumentTitle from '../hooks/useDocumentTitle';
import TimeZoneNotice from '../components/learner/TimeZoneNotice';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import {
  learnerApi, usePlannerItems, useMissingSubmissions, useDashboardCards, useLearnerMutation,
} from '../services/api_learner';
import { groupByDay, dayLabel, isDone, itemTitle, itemKey, PLANNABLE_LABELS, windowRange } from '../utils/planner';
import { dateTimeLocalToISO, isoToDateTimeLocal } from '../utils/datetimeLocal';

// To-do list / planner (G144): everything due or scheduled across the
// user's courses, grouped by day in their time zone, with mark-as-done,
// personal to-do notes and the missing-work list.

const DAYS = 14;

function formatTime(iso) {
  return iso ? new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' }) : '';
}

function StatusChips({ item }) {
  const s = item.submissions;
  if (!s || typeof s !== 'object') return null;
  const chips = [];
  if (s.missing && !isDone(item)) chips.push(['Missing', 'bg-accent-danger/15 text-accent-danger']);
  if (s.late) chips.push(['Late', 'bg-accent-warning/20 text-accent-warning']);
  if (s.excused) chips.push(['Excused', 'bg-surface-2 text-text-secondary']);
  else if (s.graded) chips.push(['Graded', 'bg-accent-success/15 text-accent-success']);
  else if (s.submitted) chips.push(['Submitted', 'bg-brand-100 text-brand-800']);
  if (item.new_activity) chips.push(['New', 'bg-orange-100 text-orange-700']);
  return chips.map(([label, cls]) => (
    <span key={label} className={`rounded-full px-2 py-0.5 text-xs ${cls}`}>{label}</span>
  ));
}

function NoteDialog({ open, onOpenChange, note, courses }) {
  const [title, setTitle] = useState(note?.plannable?.title || '');
  const [details, setDetails] = useState(note?.plannable?.details || '');
  const [when, setWhen] = useState(isoToDateTimeLocal(note?.plannable_date || new Date().toISOString()));
  const [courseId, setCourseId] = useState(note?.course_id ? String(note.course_id) : '');
  const [error, setError] = useState('');
  const save = useLearnerMutation(async () => {
    const body = {
      title,
      details,
      todo_date: dateTimeLocalToISO(when) || new Date().toISOString(),
      course_id: courseId ? Number(courseId) : null,
    };
    return note ? learnerApi.updatePlannerNote(note.plannable_id, body) : learnerApi.createPlannerNote(body);
  }, { onSuccess: () => onOpenChange(false), onError: (e) => setError(e.message) });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{note ? 'Edit to do' : 'Add to do'}</DialogTitle>
          <DialogDescription>Personal reminders appear only on your planner.</DialogDescription>
        </DialogHeader>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (!title.trim()) {
              setError('Title is required.');
              return;
            }
            save.mutate();
          }}
        >
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          <div>
            <label htmlFor="note-title" className="mb-1 block text-sm font-medium">Title</label>
            <input id="note-title" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={255}
              className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
          </div>
          <div>
            <label htmlFor="note-date" className="mb-1 block text-sm font-medium">Date</label>
            <input id="note-date" type="datetime-local" value={when} onChange={(e) => setWhen(e.target.value)}
              className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
          </div>
          <div>
            <label htmlFor="note-course" className="mb-1 block text-sm font-medium">Course (optional)</label>
            <select id="note-course" value={courseId} onChange={(e) => setCourseId(e.target.value)}
              className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm">
              <option value="">Optional: add a course</option>
              {(courses || []).map((c) => <option key={c.id} value={c.id}>{c.shortName}</option>)}
            </select>
          </div>
          <div>
            <label htmlFor="note-details" className="mb-1 block text-sm font-medium">Details</label>
            <textarea id="note-details" value={details} onChange={(e) => setDetails(e.target.value)} rows={3}
              className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={save.isPending}>{save.isPending ? 'Saving…' : 'Save'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PlannerRow({ item, onToggle, onEdit, onDelete }) {
  const done = isDone(item);
  const title = itemTitle(item);
  const checkboxId = `done-${itemKey(item).replace(':', '-')}`;
  return (
    <li className="flex items-start gap-3 px-4 py-3">
      <input
        id={checkboxId}
        type="checkbox"
        className="mt-1 h-4 w-4"
        checked={done}
        onChange={() => onToggle(item, !done)}
        aria-label={`${done ? 'Mark not done' : 'Mark done'}: ${title}`}
      />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs uppercase tracking-wide text-text-secondary">
            {item.context_name ? `${item.context_name} · ` : ''}{PLANNABLE_LABELS[item.plannable_type] || item.plannable_type}
          </span>
          <StatusChips item={item} />
        </div>
        {item.html_url ? (
          <Link to={item.html_url} className={`text-sm font-medium hover:underline ${done ? 'text-text-secondary line-through' : 'text-text-primary'}`}>
            {title}
          </Link>
        ) : (
          <span className={`text-sm font-medium ${done ? 'text-text-secondary line-through' : 'text-text-primary'}`}>{title}</span>
        )}
        {item.plannable_type === 'planner_note' && item.plannable?.details && (
          <p className="mt-0.5 whitespace-pre-wrap text-xs text-text-secondary">{item.plannable.details}</p>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1 text-xs text-text-secondary">
        {item.plannable_type === 'assignment' || item.plannable_type === 'quiz' || item.plannable_type === 'discussion_topic'
          ? <span>Due {formatTime(item.plannable_date)}</span>
          : <span>{formatTime(item.plannable_date)}</span>}
        {item.plannable?.points_possible != null && <span>· {item.plannable.points_possible} pts</span>}
        {item.plannable_type === 'planner_note' && (
          <>
            <button type="button" onClick={() => onEdit(item)} className="rounded p-1 hover:bg-surface-1" aria-label={`Edit ${title}`}>
              <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
            <button type="button" onClick={() => onDelete(item)} className="rounded p-1 hover:bg-surface-1" aria-label={`Delete ${title}`}>
              <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </>
        )}
      </div>
    </li>
  );
}

export default function PlannerPage() {
  useDocumentTitle('To do');
  const [anchor, setAnchor] = useState(() => new Date());
  const [showDone, setShowDone] = useState(true);
  const [dialog, setDialog] = useState(null); // {note} | {note:null}
  const range = useMemo(() => windowRange(anchor, DAYS), [anchor]);
  const query = usePlannerItems({ startDate: range.start, endDate: range.end }, { placeholderData: (prev) => prev });
  const missing = useMissingSubmissions('self');
  const cards = useDashboardCards(true);
  const toggle = useLearnerMutation(({ item, done }) =>
    learnerApi.setPlannerOverride({ plannableType: item.plannable_type, plannableId: item.plannable_id, markedComplete: done }));
  const remove = useLearnerMutation((item) => learnerApi.deletePlannerNote(item.plannable_id));
  const dismissMissing = useLearnerMutation((m) =>
    learnerApi.setPlannerOverride({ plannableType: 'assignment', plannableId: m.id, dismissed: true }));

  const shift = (days) => setAnchor((d) => new Date(d.getTime() + days * 86400000));
  const missingList = (missing.data || []).filter((m) => !m.dismissed);

  return (
    <Page query={query} title="To do">
      {(items) => {
        const visible = showDone ? items : items.filter((it) => !isDone(it));
        const groups = groupByDay(visible);
        return (
          <Layout>
            <div className="mx-auto max-w-4xl space-y-6">
              <header className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h1 className="text-2xl font-bold text-text-primary">To do</h1>
                  <p className="text-sm text-text-secondary">Everything due or scheduled across your courses.</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => shift(-DAYS)} aria-label="Previous two weeks">
                    <ChevronLeft className="h-4 w-4" aria-hidden="true" />
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setAnchor(new Date())}>Today</Button>
                  <Button variant="outline" size="sm" onClick={() => shift(DAYS)} aria-label="Next two weeks">
                    <ChevronRight className="h-4 w-4" aria-hidden="true" />
                  </Button>
                  <label className="ms-2 flex items-center gap-1.5 text-sm">
                    <input type="checkbox" checked={showDone} onChange={(e) => setShowDone(e.target.checked)} />
                    Show completed
                  </label>
                  <Button size="sm" onClick={() => setDialog({ note: null })}>
                    <Plus className="me-1 h-4 w-4" aria-hidden="true" /> Add to do
                  </Button>
                </div>
              </header>

              <TimeZoneNotice />

              {missingList.length > 0 && (
                <section aria-labelledby="missing-heading" className="rounded-lg border border-accent-danger/30 bg-accent-danger/5">
                  <h2 id="missing-heading" className="flex items-center gap-2 px-4 pt-3 text-sm font-semibold text-accent-danger">
                    <AlertTriangle className="h-4 w-4" aria-hidden="true" />
                    {missingList.length} missing item{missingList.length === 1 ? '' : 's'}
                  </h2>
                  <ul className="divide-y divide-border-subtle">
                    {missingList.map((m) => (
                      <li key={m.id} className="flex items-center gap-3 px-4 py-2 text-sm">
                        <Link to={m.html_url} className="flex-1 font-medium hover:underline">{m.name}</Link>
                        <span className="text-xs text-text-secondary">{m.course_name}</span>
                        <span className="text-xs text-text-secondary">
                          was due {m.due_at ? new Date(m.due_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) : ''}
                        </span>
                        <button type="button" className="text-xs text-text-secondary underline" onClick={() => dismissMissing.mutate(m)}>
                          Dismiss
                        </button>
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              {groups.length === 0 ? (
                <div className="rounded-lg border border-border-default bg-surface-0 px-6 py-12 text-center">
                  <CalendarDays className="mx-auto mb-2 h-8 w-8 text-text-disabled" aria-hidden="true" />
                  <p className="text-sm text-text-secondary">Nothing planned for these two weeks.</p>
                </div>
              ) : (
                groups.map((g) => (
                  <section key={g.day} aria-labelledby={`day-${g.day}`} className="overflow-hidden rounded-lg border border-border-default bg-surface-0">
                    <h2 id={`day-${g.day}`} className="border-b border-border-subtle bg-surface-1 px-4 py-2 text-sm font-semibold">
                      {dayLabel(g.day)}
                    </h2>
                    <ul className="divide-y divide-border-subtle">
                      {g.items.map((item) => (
                        <PlannerRow
                          key={itemKey(item)}
                          item={item}
                          onToggle={(it, done) => toggle.mutate({ item: it, done })}
                          onEdit={(it) => setDialog({ note: it })}
                          onDelete={(it) => remove.mutate(it)}
                        />
                      ))}
                    </ul>
                  </section>
                ))
              )}
            </div>
            {dialog && (
              <NoteDialog
                key={dialog.note ? itemKey(dialog.note) : 'new'}
                open
                onOpenChange={(o) => { if (!o) setDialog(null); }}
                note={dialog.note}
                courses={cards.data}
              />
            )}
          </Layout>
        );
      }}
    </Page>
  );
}
