import { useState } from 'react';
import { CalendarOff, Trash2 } from 'lucide-react';
import { useBlackoutDates, useCreateBlackoutDate, useDeleteBlackoutDate } from '../services/api_modules';
import { Button } from '@/components/ui/button';

// BlackoutDatesPanel manages a course's pacing blackout dates (G079):
// days a published pace schedules no work.
export default function BlackoutDatesPanel({ courseId, onChange }) {
  const query = useBlackoutDates(courseId);
  const create = useCreateBlackoutDate(courseId);
  const remove = useDeleteBlackoutDate(courseId);
  const [form, setForm] = useState({ event_title: '', start_date: '', end_date: '' });
  const [error, setError] = useState(null);

  const canAdd = form.event_title.trim() && form.start_date && (form.end_date || form.start_date);

  const handleAdd = (e) => {
    e.preventDefault();
    if (!canAdd) return;
    setError(null);
    create.mutate(
      { event_title: form.event_title.trim(), start_date: form.start_date, end_date: form.end_date || form.start_date },
      {
        onSuccess: () => {
          setForm({ event_title: '', start_date: '', end_date: '' });
          onChange?.();
        },
        onError: (err) => setError(err.message),
      },
    );
  };

  const handleDelete = (id) => {
    remove.mutate(id, { onSuccess: () => onChange?.(), onError: (err) => setError(err.message) });
  };

  const rows = query.data || [];

  return (
    <section className="bg-surface-0 rounded-lg shadow mb-6" aria-labelledby="blackout-dates-heading">
      <div className="flex items-center gap-2 px-4 py-3 border-b">
        <CalendarOff className="w-5 h-5 text-indigo-500" aria-hidden />
        <h3 id="blackout-dates-heading" className="font-semibold text-text-secondary">Blackout dates</h3>
      </div>
      <div className="p-4 space-y-3">
        <p className="text-sm text-text-tertiary">Pacing skips these days, like weekends, when it schedules due dates.</p>
        {query.isLoading && <p className="text-sm text-text-tertiary">Loading…</p>}
        {query.isError && <p className="text-sm text-accent-danger">{query.error?.message}</p>}
        {rows.length > 0 && (
          <ul className="divide-y border rounded-md">
            {rows.map((b) => (
              <li key={b.id} className="flex items-center justify-between px-3 py-2 text-sm">
                <span>
                  <span className="font-medium text-text-primary">{b.event_title}</span>
                  <span className="text-text-tertiary ml-2">
                    {b.start_date}{b.end_date && b.end_date !== b.start_date ? ` – ${b.end_date}` : ''}
                  </span>
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(b.id)}
                  aria-label={`Delete blackout date ${b.event_title}`}
                >
                  <Trash2 className="w-4 h-4" />
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={handleAdd} className="flex flex-wrap items-end gap-2">
          <label className="text-xs text-text-tertiary flex flex-col">
            Title
            <input
              type="text"
              value={form.event_title}
              onChange={(e) => setForm({ ...form, event_title: e.target.value })}
              className="mt-1 border border-border-strong rounded px-2 py-1 text-sm"
              placeholder="Spring break"
            />
          </label>
          <label className="text-xs text-text-tertiary flex flex-col">
            Start
            <input
              type="date"
              value={form.start_date}
              onChange={(e) => setForm({ ...form, start_date: e.target.value })}
              className="mt-1 border border-border-strong rounded px-2 py-1 text-sm"
            />
          </label>
          <label className="text-xs text-text-tertiary flex flex-col">
            End
            <input
              type="date"
              value={form.end_date}
              min={form.start_date || undefined}
              onChange={(e) => setForm({ ...form, end_date: e.target.value })}
              className="mt-1 border border-border-strong rounded px-2 py-1 text-sm"
            />
          </label>
          <Button type="submit" size="sm" disabled={!canAdd || create.isPending}>
            {create.isPending ? 'Adding…' : 'Add blackout date'}
          </Button>
        </form>
        {error && <p className="text-sm text-accent-danger" role="alert">{error}</p>}
      </div>
    </section>
  );
}
