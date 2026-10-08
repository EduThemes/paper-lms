import React from 'react';
import { useTranslation } from 'react-i18next';
import { Calendar, ClipboardList, HelpCircle, CalendarCheck, Users, Repeat } from 'lucide-react';
import { daysIn, dayKey, itemsByDay } from '../../utils/calendarView';

// One color per calendar, in the order the calendar list shows them.
export const CALENDAR_COLORS = [
  { chip: 'bg-blue-100 text-blue-900 hover:bg-blue-200', dot: 'bg-blue-500' },
  { chip: 'bg-emerald-100 text-emerald-900 hover:bg-emerald-200', dot: 'bg-emerald-500' },
  { chip: 'bg-amber-100 text-amber-900 hover:bg-amber-200', dot: 'bg-amber-500' },
  { chip: 'bg-violet-100 text-violet-900 hover:bg-violet-200', dot: 'bg-violet-500' },
  { chip: 'bg-rose-100 text-rose-900 hover:bg-rose-200', dot: 'bg-rose-500' },
  { chip: 'bg-cyan-100 text-cyan-900 hover:bg-cyan-200', dot: 'bg-cyan-500' },
  { chip: 'bg-lime-100 text-lime-900 hover:bg-lime-200', dot: 'bg-lime-500' },
  { chip: 'bg-fuchsia-100 text-fuchsia-900 hover:bg-fuchsia-200', dot: 'bg-fuchsia-500' },
];

export function colorFor(colorIndex, contextCode) {
  const idx = colorIndex[contextCode];
  return CALENDAR_COLORS[(idx ?? 0) % CALENDAR_COLORS.length];
}

const ICONS = {
  event: Calendar,
  assignment: ClipboardList,
  quiz: HelpCircle,
  appointment: CalendarCheck,
  appointment_slot: Users,
};

export function ItemIcon({ type, className = 'w-3.5 h-3.5' }) {
  const Icon = ICONS[type] || Calendar;
  return <Icon className={className} aria-hidden="true" />;
}

export function formatItemTime(item, t) {
  if (item.all_day) return t('calendarLane.allDay');
  const time = (v) => new Date(v).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  if (item.type === 'assignment' || item.type === 'quiz') {
    return t('calendarLane.dueAt', { time: time(item.start_at) });
  }
  return item.end_at ? `${time(item.start_at)} – ${time(item.end_at)}` : time(item.start_at);
}

function ItemChip({ item, colorIndex, onSelect, compact }) {
  const { t } = useTranslation();
  const color = colorFor(colorIndex, item.context_code);
  const time = !item.all_day && compact
    ? new Date(item.start_at).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
    : '';
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onSelect(item);
      }}
      className={`w-full text-start px-1 py-0.5 text-xs rounded truncate flex items-center gap-1 transition-colors ${color.chip}`}
      title={`${item.title} — ${item.context_name || ''}`}
      data-item-type={item.type}
    >
      <ItemIcon type={item.type} className="w-3 h-3 flex-shrink-0" />
      {item.rrule && <Repeat className="w-3 h-3 flex-shrink-0" aria-label={t('calendarLane.recurring')} />}
      {time && <span className="font-medium flex-shrink-0">{time}</span>}
      <span className="truncate">{item.title}</span>
    </button>
  );
}

export function MonthView({ items, range, anchor, colorIndex, onSelect, onDayClick }) {
  const { t } = useTranslation();
  const byDay = itemsByDay(items);
  const days = daysIn(range.start, range.end);
  const todayKey = dayKey(new Date());
  const month = anchor.getMonth();
  const weekdays = days.slice(0, 7).map((d) => d.toLocaleDateString(undefined, { weekday: 'short' }));
  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
      <div className="grid grid-cols-7 border-b bg-surface-1">
        {weekdays.map((wd) => (
          <div key={wd} className="px-2 py-2 text-xs font-medium text-text-tertiary text-center">{wd}</div>
        ))}
      </div>
      <div className="grid grid-cols-7 auto-rows-fr">
        {days.map((d) => {
          const key = dayKey(d);
          const list = byDay[key] || [];
          const inMonth = d.getMonth() === month;
          return (
            <div
              key={key}
              className={`border-b border-e border-border-subtle min-h-[6rem] p-1 hover:bg-brand-50 cursor-pointer transition-colors ${inMonth ? '' : 'bg-surface-1'}`}
              onClick={() => onDayClick && onDayClick(d)}
              data-day={key}
            >
              <div className="flex items-center justify-between mb-0.5">
                <span
                  className={`text-xs font-medium w-6 h-6 flex items-center justify-center rounded-full ${
                    key === todayKey ? 'bg-brand-600 text-white' : inMonth ? 'text-text-secondary' : 'text-text-disabled'
                  }`}
                >
                  {d.getDate()}
                </span>
              </div>
              <div className="space-y-0.5 overflow-hidden">
                {list.slice(0, 4).map((item) => (
                  <ItemChip key={item.id} item={item} colorIndex={colorIndex} onSelect={onSelect} compact />
                ))}
                {list.length > 4 && (
                  <div className="text-xs text-text-disabled px-1">{t('calendarLane.more', { count: list.length - 4 })}</div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function WeekView({ items, range, colorIndex, onSelect, onDayClick }) {
  const { t } = useTranslation();
  const byDay = itemsByDay(items);
  const days = daysIn(range.start, range.end);
  const todayKey = dayKey(new Date());
  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-hidden grid grid-cols-1 md:grid-cols-7 md:divide-x divide-border-subtle">
      {days.map((d) => {
        const key = dayKey(d);
        const list = byDay[key] || [];
        return (
          <div key={key} className="min-h-[8rem] md:min-h-[24rem] border-b md:border-b-0 border-border-subtle" data-day={key}>
            <button
              type="button"
              onClick={() => onDayClick && onDayClick(d)}
              className={`w-full px-2 py-2 text-start text-xs font-medium border-b border-border-subtle ${
                key === todayKey ? 'bg-brand-600 text-white' : 'bg-surface-1 text-text-secondary hover:bg-brand-50'
              }`}
            >
              {d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}
            </button>
            <div className="p-1 space-y-1">
              {list.length === 0 && <div className="text-xs text-text-disabled px-1 py-2 md:hidden">{t('calendarLane.nothing')}</div>}
              {list.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => onSelect(item)}
                  className={`w-full text-start rounded px-1.5 py-1 text-xs ${colorFor(colorIndex, item.context_code).chip}`}
                  data-item-type={item.type}
                >
                  <div className="flex items-center gap-1 font-medium">
                    <ItemIcon type={item.type} className="w-3 h-3 flex-shrink-0" />
                    {item.rrule && <Repeat className="w-3 h-3 flex-shrink-0" aria-label={t('calendarLane.recurring')} />}
                    <span className="truncate">{item.title}</span>
                  </div>
                  <div className="opacity-80">{formatItemTime(item, t)}</div>
                </button>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function AgendaView({ items, colorIndex, onSelect }) {
  const { t } = useTranslation();
  const byDay = itemsByDay(items);
  const keys = Object.keys(byDay).sort();
  if (keys.length === 0) {
    return <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">{t('calendarLane.noItems')}</div>;
  }
  return (
    <div className="bg-surface-0 rounded-lg shadow divide-y">
      {keys.map((key) => {
        const [y, m, d] = key.split('-').map(Number);
        const label = new Date(y, m - 1, d).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric' });
        return (
          <div key={key} data-day={key}>
            <div className="px-4 py-2 bg-surface-1 text-sm font-medium text-text-secondary">{label}</div>
            <ul className="divide-y divide-border-subtle">
              {byDay[key].map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => onSelect(item)}
                    className="w-full text-start flex items-start gap-3 px-4 py-3 hover:bg-surface-1"
                    data-item-type={item.type}
                  >
                    <span className={`mt-1.5 w-2.5 h-2.5 rounded-full flex-shrink-0 ${colorFor(colorIndex, item.context_code).dot}`} />
                    <ItemIcon type={item.type} className="w-4 h-4 text-text-tertiary flex-shrink-0 mt-0.5" />
                    <span className="min-w-0 flex-1">
                      <span className="block font-medium text-text-primary text-sm truncate">{item.title}</span>
                      <span className="block text-xs text-text-tertiary">
                        {formatItemTime(item, t)}
                        {item.context_name && <span className="ms-2">· {item.context_name}</span>}
                        {item.section_name && <span className="ms-1">({item.section_name})</span>}
                        {item.location_name && <span className="ms-2">· {item.location_name}</span>}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
