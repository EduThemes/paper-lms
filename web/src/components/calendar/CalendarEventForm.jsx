import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { calendarApi } from '../../services/api_calendar';
import {
  WEEKDAY_CODES, buildRRule, parseRRule, toLocalInput, fromLocalInput, dayKey,
} from '../../utils/calendarView';
import { getUserTimeZone, isValidTimeZone, isoToZonedWallClock, zonedWallClockToISO } from '../../lib/userTimeZone';
import RichContentEditorV2 from '../rce/RichContentEditorV2';
import { toEditorHTML, richValueOrEmpty } from '../../utils/richContent';

const inputCls = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';
const labelCls = 'block text-sm font-medium text-text-secondary mb-1';

function sectionRowsFrom(event, sections) {
  const byCode = {};
  (event?.child_events || []).forEach((ch) => { byCode[ch.context_code] = ch; });
  return sections.map((s) => {
    const ch = byCode[`course_section_${s.id}`];
    return {
      id: s.id,
      name: s.name,
      enabled: Boolean(ch),
      start_at: ch ? toLocalInput(ch.start_at) : '',
      end_at: ch ? toLocalInput(ch.end_at) : '',
    };
  });
}

// CalendarEventForm creates or edits a calendar event: the calendar it
// goes on, a repeat rule (G095) and, for a course an instructor teaches,
// different times per section (child events, G095). Editing one event of
// a series asks which occurrences the change applies to.
const CalendarEventForm = ({ contexts, defaultContextCode, event, initialStart, onSaved, onCancel }) => {
  const { t } = useTranslation();
  const editing = Boolean(event);
  const creatable = contexts.filter((c) => c.can_create_calendar_events);
  const [contextCode, setContextCode] = useState(
    event?.context_code || defaultContextCode || creatable[0]?.asset_string || '',
  );
  const context = contexts.find((c) => c.asset_string === contextCode);
  const sections = context?.type === 'course' && context.is_instructor ? (context.sections || []) : [];

  const startDefault = initialStart ? `${dayKey(initialStart)}T09:00` : '';
  // An existing all-day event keeps the zone it was created in: its date
  // is shown and saved as midnight there, not in the editor's zone.
  const eventZone = event?.all_day && isValidTimeZone(event?.time_zone) ? event.time_zone : null;
  const [form, setForm] = useState({
    title: event?.title || '',
    // Descriptions are rich content (G010); older ones are plain text.
    description: toEditorHTML(event?.description || ''),
    start_at: event ? (eventZone ? isoToZonedWallClock(event.start_at, eventZone) : toLocalInput(event.start_at)) : startDefault,
    end_at: event ? toLocalInput(event.end_at) : '',
    all_day: Boolean(event?.all_day),
    location_name: event?.location_name || '',
    location_address: event?.location_address || '',
  });
  const originalRule = event?.rrule || '';
  const [repeat, setRepeat] = useState(parseRRule(originalRule));
  const hadChildren = (event?.child_events || []).length > 0;
  const [useSections, setUseSections] = useState(hadChildren);
  const [sectionRows, setSectionRows] = useState(() => sectionRowsFrom(event, sections));
  const [which, setWhich] = useState('one');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const inSeries = Boolean(event?.series_uuid);

  // A different calendar can change the section list.
  const sectionKey = sections.map((s) => s.id).join(',');
  useEffect(() => {
    setSectionRows(sectionRowsFrom(event, sections));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sectionKey]);

  const set = (field) => (e) => {
    const value = e.target.type === 'checkbox' ? e.target.checked : e.target.value;
    setForm((f) => ({ ...f, [field]: value }));
  };
  const setSection = (id, field, value) => setSectionRows((rows) => rows.map((r) => (r.id === id ? { ...r, [field]: value } : r)));
  const toggleDay = (code) => setRepeat((r) => ({
    ...r,
    byDay: r.byDay.includes(code) ? r.byDay.filter((d) => d !== code) : [...r.byDay, code],
  }));

  // The zone the event is saved in: an all-day event being edited keeps
  // its own; anything else uses the profile zone.
  const saveZone = (form.all_day && eventZone) || getUserTimeZone();

  const toISO = (value) => {
    if (!value) return null;
    if (form.all_day) {
      // An all-day event starts at midnight of its day in its zone (sent
      // as time_zone below), not the browser's midnight.
      return zonedWallClockToISO(value.substring(0, 10), saveZone) || null;
    }
    return fromLocalInput(value);
  };

  const submit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const payload = {
        title: form.title,
        description: richValueOrEmpty(form.description),
        all_day: form.all_day,
        location_name: form.location_name,
        location_address: form.location_address,
        time_zone: saveZone,
      };
      const sectionTimes = useSections
        ? sectionRows.filter((r) => r.enabled).map((r) => ({
          context_code: `course_section_${r.id}`,
          start_at: fromLocalInput(r.start_at),
          end_at: fromLocalInput(r.end_at),
        }))
        : [];
      if (useSections && sectionTimes.length === 0) {
        throw new Error(t('calendarLane.form.pickSection'));
      }
      if (sectionTimes.some((s) => !s.start_at || !s.end_at)) {
        throw new Error(t('calendarLane.form.sectionTimesRequired'));
      }
      if (sectionTimes.length) {
        payload.child_event_data = sectionTimes;
      } else {
        payload.start_at = toISO(form.start_at);
        payload.end_at = form.all_day ? null : toISO(form.end_at);
        if (editing && hadChildren) payload.remove_child_events = true;
      }
      const rule = sectionTimes.length ? '' : buildRRule(repeat);
      if (!editing) {
        payload.context_code = contextCode;
        if (rule) payload.rrule = rule;
        await calendarApi.createEvent(payload);
      } else {
        if (rule !== originalRule && (rule || inSeries)) payload.rrule = rule;
        await calendarApi.updateEvent(event.id, payload, inSeries ? which : undefined);
      }
      onSaved();
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
      <h3 className="font-semibold mb-4">{editing ? t('calendarLane.form.editTitle') : t('calendarLane.form.newTitle')}</h3>
      {error && <div className="mb-3 text-sm text-accent-danger" role="alert">{error}</div>}
      <form onSubmit={submit} className="space-y-4">
        {!editing && creatable.length > 1 && (
          <div>
            <label htmlFor="cal-form-context" className={labelCls}>{t('calendarLane.form.calendar')}</label>
            <select id="cal-form-context" value={contextCode} onChange={(e) => setContextCode(e.target.value)} className={inputCls}>
              {creatable.map((c) => <option key={c.asset_string} value={c.asset_string}>{c.name}</option>)}
            </select>
          </div>
        )}
        <div>
          <label htmlFor="cal-form-title" className={labelCls}>{t('common.title')}</label>
          <input id="cal-form-title" type="text" required value={form.title} onChange={set('title')} className={inputCls} />
        </div>
        <div>
          <label htmlFor="cal-form-description" className={labelCls}>{t('common.description')}</label>
          <RichContentEditorV2
            label={t('common.description')}
            value={form.description}
            onChange={(html) => setForm((prev) => ({ ...prev, description: html }))}
            placeholder="Details, links, files…"
            courseId={context?.type === 'course' ? context.id : undefined}
          />
        </div>

        {sections.length > 0 && (
          <div className="flex items-center gap-2">
            <input id="cal-form-use-sections" type="checkbox" checked={useSections} onChange={(e) => setUseSections(e.target.checked)} className="rounded border-border-strong" />
            <label htmlFor="cal-form-use-sections" className="text-sm text-text-secondary">{t('calendarLane.form.useSections')}</label>
          </div>
        )}

        {useSections && sections.length > 0 ? (
          <fieldset className="rounded-md border border-border-default p-3 space-y-2">
            <legend className="px-1 text-sm font-medium text-text-secondary">{t('calendarLane.form.sectionTimes')}</legend>
            {sectionRows.map((row) => (
              <div key={row.id} className="grid grid-cols-1 md:grid-cols-[minmax(8rem,1fr)_1fr_1fr] gap-2 items-center">
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={row.enabled} onChange={(e) => setSection(row.id, 'enabled', e.target.checked)} />
                  {row.name}
                </label>
                <input
                  type="datetime-local"
                  aria-label={t('calendarLane.form.sectionStart', { name: row.name })}
                  value={row.start_at}
                  disabled={!row.enabled}
                  onChange={(e) => setSection(row.id, 'start_at', e.target.value)}
                  className={inputCls}
                />
                <input
                  type="datetime-local"
                  aria-label={t('calendarLane.form.sectionEnd', { name: row.name })}
                  value={row.end_at}
                  disabled={!row.enabled}
                  onChange={(e) => setSection(row.id, 'end_at', e.target.value)}
                  className={inputCls}
                />
              </div>
            ))}
          </fieldset>
        ) : (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label htmlFor="cal-form-start" className={labelCls}>{t('calendarLane.form.start')}</label>
                <input
                  id="cal-form-start"
                  type={form.all_day ? 'date' : 'datetime-local'}
                  value={form.all_day ? form.start_at.substring(0, 10) : form.start_at}
                  onChange={set('start_at')}
                  className={inputCls}
                  required
                />
              </div>
              {!form.all_day && (
                <div>
                  <label htmlFor="cal-form-end" className={labelCls}>{t('calendarLane.form.end')}</label>
                  <input id="cal-form-end" type="datetime-local" value={form.end_at} onChange={set('end_at')} className={inputCls} />
                </div>
              )}
            </div>
            <div className="flex items-center gap-2">
              <input id="cal-form-all-day" type="checkbox" checked={form.all_day} onChange={set('all_day')} className="rounded border-border-strong" />
              <label htmlFor="cal-form-all-day" className="text-sm text-text-secondary">{t('calendarLane.form.allDay')}</label>
            </div>

            <fieldset className="rounded-md border border-border-default p-3 space-y-3">
              <legend className="px-1 text-sm font-medium text-text-secondary">{t('calendarLane.form.repeat')}</legend>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label htmlFor="cal-form-freq" className="sr-only">{t('calendarLane.form.repeat')}</label>
                  <select id="cal-form-freq" value={repeat.freq} onChange={(e) => setRepeat((r) => ({ ...r, freq: e.target.value }))} className={inputCls}>
                    <option value="none">{t('calendarLane.repeat.none')}</option>
                    <option value="DAILY">{t('calendarLane.repeat.daily')}</option>
                    <option value="WEEKLY">{t('calendarLane.repeat.weekly')}</option>
                    <option value="MONTHLY">{t('calendarLane.repeat.monthly')}</option>
                  </select>
                </div>
                {repeat.freq !== 'none' && (
                  <div className="flex items-center gap-2 text-sm">
                    <label htmlFor="cal-form-interval">{t('calendarLane.repeat.everyLabel')}</label>
                    <input
                      id="cal-form-interval"
                      type="number"
                      min={1}
                      max={365}
                      value={repeat.interval}
                      onChange={(e) => setRepeat((r) => ({ ...r, interval: e.target.value }))}
                      className="w-20 border border-border-strong rounded-md px-2 py-1.5 text-sm"
                    />
                    <span>{t(`calendarLane.repeat.unit.${repeat.freq}`)}</span>
                  </div>
                )}
              </div>
              {repeat.freq === 'WEEKLY' && (
                <div className="flex flex-wrap gap-1" role="group" aria-label={t('calendarLane.repeat.on')}>
                  {WEEKDAY_CODES.map((code) => (
                    <button
                      key={code}
                      type="button"
                      aria-pressed={repeat.byDay.includes(code)}
                      onClick={() => toggleDay(code)}
                      className={`px-2 py-1 rounded text-xs border ${repeat.byDay.includes(code) ? 'bg-brand-600 text-white border-brand-600' : 'border-border-strong text-text-secondary'}`}
                    >
                      {t(`calendarLane.weekdays.${code}`)}
                    </button>
                  ))}
                </div>
              )}
              {repeat.freq !== 'none' && (
                <div className="flex flex-wrap items-center gap-3 text-sm">
                  <label className="flex items-center gap-1">
                    <input type="radio" name="cal-ends" checked={repeat.ends === 'count'} onChange={() => setRepeat((r) => ({ ...r, ends: 'count' }))} />
                    {t('calendarLane.repeat.after')}
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={200}
                    aria-label={t('calendarLane.repeat.occurrences')}
                    value={repeat.count}
                    disabled={repeat.ends !== 'count'}
                    onChange={(e) => setRepeat((r) => ({ ...r, count: e.target.value }))}
                    className="w-20 border border-border-strong rounded-md px-2 py-1.5 text-sm"
                  />
                  <span>{t('calendarLane.repeat.occurrences')}</span>
                  <label className="flex items-center gap-1">
                    <input type="radio" name="cal-ends" checked={repeat.ends === 'until'} onChange={() => setRepeat((r) => ({ ...r, ends: 'until' }))} />
                    {t('calendarLane.repeat.onDate')}
                  </label>
                  <input
                    type="date"
                    aria-label={t('calendarLane.repeat.endDate')}
                    value={repeat.until}
                    disabled={repeat.ends !== 'until'}
                    onChange={(e) => setRepeat((r) => ({ ...r, until: e.target.value }))}
                    className="border border-border-strong rounded-md px-2 py-1.5 text-sm"
                  />
                </div>
              )}
            </fieldset>
          </>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label htmlFor="cal-form-location" className={labelCls}>{t('calendarLane.form.locationName')}</label>
            <input id="cal-form-location" type="text" value={form.location_name} onChange={set('location_name')} className={inputCls} />
          </div>
          <div>
            <label htmlFor="cal-form-address" className={labelCls}>{t('calendarLane.form.locationAddress')}</label>
            <input id="cal-form-address" type="text" value={form.location_address} onChange={set('location_address')} className={inputCls} />
          </div>
        </div>

        {inSeries && (
          <fieldset className="text-sm space-y-1">
            <legend className="font-medium text-text-secondary mb-1">{t('calendarLane.scope.applyTo')}</legend>
            {['one', 'following', 'all'].map((w) => (
              <label key={w} className="flex items-center gap-2">
                <input type="radio" name="cal-which" value={w} checked={which === w} onChange={() => setWhich(w)} />
                {t(`calendarLane.scope.${w}`)}
              </label>
            ))}
          </fieldset>
        )}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onCancel} className="border border-border-strong text-text-secondary px-4 py-2 rounded-md hover:bg-surface-1 text-sm font-medium">
            {t('common.cancel')}
          </button>
          <button type="submit" disabled={saving} className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
            {saving ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </form>
    </div>
  );
};

export default CalendarEventForm;
