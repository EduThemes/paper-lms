import React from 'react';
import { Calendar, Users, Check, MapPin } from 'lucide-react';

/**
 * SlotPicker — student-facing grid of available appointment slots.
 *
 * Props:
 *  - slots:        array of slot objects ({id, start_at, end_at, available, reservation_count, effective_limit})
 *  - reservedIds:  Set<number> of slot IDs the current user has already reserved
 *  - onReserve:    (slot) => void
 *  - onCancel:     (slot) => void  (called for slots the user already reserved)
 *  - locationName: string (optional, displayed once at top)
 *  - busy:         boolean — disables buttons while a request is in flight
 */
const SlotPicker = ({ slots = [], reservedIds = new Set(), onReserve, onCancel, locationName, busy }) => {
  if (!slots.length) {
    return (
      <div className="rounded-lg border border-dashed border-border-strong bg-surface-1 px-6 py-10 text-center text-text-tertiary">
        <Calendar className="mx-auto mb-2 h-6 w-6 opacity-50" />
        No appointments are currently available.
      </div>
    );
  }

  const fmtDate = (iso) => new Date(iso).toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
  const fmtTime = (iso) => new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });

  // Group slots by date for nicer scanning.
  const byDate = slots.reduce((acc, slot) => {
    const key = fmtDate(slot.start_at);
    if (!acc[key]) acc[key] = [];
    acc[key].push(slot);
    return acc;
  }, {});

  return (
    <div className="space-y-6">
      {locationName && (
        <div className="flex items-center gap-2 text-sm text-text-secondary">
          <MapPin className="h-4 w-4" /> {locationName}
        </div>
      )}
      {Object.entries(byDate).map(([date, daySlots]) => (
        <div key={date}>
          <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-text-tertiary">{date}</h3>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {daySlots.map((slot) => {
              const reserved = reservedIds.has(slot.id);
              const full = !slot.available && !reserved;
              return (
                <div
                  key={slot.id}
                  className={`rounded-lg border p-3 transition ${
                    reserved
                      ? 'border-emerald-300 bg-accent-success/10'
                      : full
                      ? 'border-border-default bg-surface-1 opacity-60'
                      : 'border-border-default bg-surface-0 hover:border-blue-400 hover:shadow-sm'
                  }`}
                >
                  <div className="flex items-baseline justify-between">
                    <div className="text-base font-medium text-text-primary">
                      {fmtTime(slot.start_at)} – {fmtTime(slot.end_at)}
                    </div>
                    <div className="flex items-center gap-1 text-xs text-text-tertiary">
                      <Users className="h-3 w-3" />
                      {slot.reservation_count ?? 0}/{slot.effective_limit ?? 1}
                    </div>
                  </div>
                  <div className="mt-3">
                    {reserved ? (
                      <button
                        type="button"
                        onClick={() => onCancel && onCancel(slot)}
                        disabled={busy}
                        className="inline-flex w-full items-center justify-center gap-1 rounded-md border border-emerald-400 bg-surface-0 px-3 py-1.5 text-sm font-medium text-emerald-700 hover:bg-accent-success/10 disabled:opacity-50"
                      >
                        <Check className="h-4 w-4" /> Reserved — Cancel
                      </button>
                    ) : full ? (
                      <span className="block w-full rounded-md bg-surface-2 px-3 py-1.5 text-center text-sm text-text-tertiary">
                        Full
                      </span>
                    ) : (
                      <button
                        type="button"
                        onClick={() => onReserve && onReserve(slot)}
                        disabled={busy}
                        className="w-full rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
                      >
                        Reserve
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
};

export default SlotPicker;
