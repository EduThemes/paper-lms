import React from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, X } from 'lucide-react';
import { WEEKDAYS, dateShiftProblem, defaultDateShift } from '../../lib/contentMigration';

const inputCls = 'w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500';

// DateShiftFields is the "Adjust events and due dates" section of the
// course copy / import forms (Canvas semantics: shift proportionally
// between the old and new course windows, optional weekday
// substitutions, or remove every date).
export default function DateShiftFields({ value, onChange, idPrefix = 'ds', oldDatesHint }) {
  const { t } = useTranslation();
  const v = value || defaultDateShift();
  const set = (patch) => onChange({ ...v, ...patch });
  const problem = dateShiftProblem(v);

  const setSub = (i, patch) => {
    const subs = v.substitutions.map((s, j) => (j === i ? { ...s, ...patch } : s));
    set({ substitutions: subs });
  };

  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium text-text-primary mb-2">
        {t('contentMigration.dateShift.legend', 'Events and due dates')}
      </legend>
      <div className="flex flex-wrap gap-4 text-sm">
        {[
          ['keep', t('contentMigration.dateShift.keep', 'Keep dates as they are')],
          ['shift', t('contentMigration.dateShift.shift', 'Adjust events and due dates')],
          ['remove', t('contentMigration.dateShift.remove', 'Remove dates')],
        ].map(([mode, label]) => (
          <label key={mode} className="inline-flex items-center gap-2 text-text-secondary">
            <input
              type="radio"
              name={`${idPrefix}-mode`}
              value={mode}
              checked={v.mode === mode}
              onChange={() => set({ mode })}
            />
            {label}
          </label>
        ))}
      </div>

      {v.mode === 'shift' && (
        <div className="space-y-3 rounded-md border border-border-default p-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label htmlFor={`${idPrefix}-old-start`} className="block text-xs font-medium text-text-secondary mb-1">
                {t('contentMigration.dateShift.oldStart', 'Original course start')}
              </label>
              <input id={`${idPrefix}-old-start`} type="date" className={inputCls} value={v.oldStart} onChange={(e) => set({ oldStart: e.target.value })} />
            </div>
            <div>
              <label htmlFor={`${idPrefix}-old-end`} className="block text-xs font-medium text-text-secondary mb-1">
                {t('contentMigration.dateShift.oldEnd', 'Original course end')}
              </label>
              <input id={`${idPrefix}-old-end`} type="date" className={inputCls} value={v.oldEnd} onChange={(e) => set({ oldEnd: e.target.value })} />
            </div>
            <div>
              <label htmlFor={`${idPrefix}-new-start`} className="block text-xs font-medium text-text-secondary mb-1">
                {t('contentMigration.dateShift.newStart', 'New course start')}
              </label>
              <input id={`${idPrefix}-new-start`} type="date" className={inputCls} value={v.newStart} onChange={(e) => set({ newStart: e.target.value })} />
            </div>
            <div>
              <label htmlFor={`${idPrefix}-new-end`} className="block text-xs font-medium text-text-secondary mb-1">
                {t('contentMigration.dateShift.newEnd', 'New course end')}
              </label>
              <input id={`${idPrefix}-new-end`} type="date" className={inputCls} value={v.newEnd} onChange={(e) => set({ newEnd: e.target.value })} />
            </div>
          </div>
          <p className="text-xs text-text-tertiary">
            {oldDatesHint || t('contentMigration.dateShift.hint', 'Dates move proportionally from the original course window to the new one and stay on the same weekday. Leave the original dates empty to use the source course dates; leave the new end empty to keep the course length.')}
          </p>

          <div>
            <div className="text-xs font-medium text-text-secondary mb-1">
              {t('contentMigration.dateShift.substitutions', 'Day substitutions')}
            </div>
            {v.substitutions.map((s, i) => (
              <div key={i} className="flex items-center gap-2 mb-2">
                <label className="sr-only" htmlFor={`${idPrefix}-sub-from-${i}`}>{t('contentMigration.dateShift.moveFrom', 'Move from')}</label>
                <select id={`${idPrefix}-sub-from-${i}`} className={inputCls} value={s.from} onChange={(e) => setSub(i, { from: e.target.value })}>
                  <option value="">—</option>
                  {WEEKDAYS.map((d, n) => <option key={d} value={n}>{d}</option>)}
                </select>
                <span className="text-xs text-text-tertiary">{t('contentMigration.dateShift.to', 'to')}</span>
                <label className="sr-only" htmlFor={`${idPrefix}-sub-to-${i}`}>{t('contentMigration.dateShift.moveTo', 'Move to')}</label>
                <select id={`${idPrefix}-sub-to-${i}`} className={inputCls} value={s.to} onChange={(e) => setSub(i, { to: e.target.value })}>
                  <option value="">—</option>
                  {WEEKDAYS.map((d, n) => <option key={d} value={n}>{d}</option>)}
                </select>
                <button
                  type="button"
                  aria-label={t('contentMigration.dateShift.removeSubstitution', 'Remove substitution')}
                  className="p-1 text-text-tertiary hover:text-accent-danger"
                  onClick={() => set({ substitutions: v.substitutions.filter((_, j) => j !== i) })}
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            ))}
            <button
              type="button"
              className="inline-flex items-center gap-1 text-xs text-brand-600 hover:underline"
              onClick={() => set({ substitutions: [...v.substitutions, { from: '', to: '' }] })}
            >
              <Plus className="w-3 h-3" />
              {t('contentMigration.dateShift.addSubstitution', 'Add a day substitution')}
            </button>
          </div>
          {problem && <p role="alert" className="text-xs text-accent-danger">{problem}</p>}
        </div>
      )}
    </fieldset>
  );
}
