import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Copy, Loader2, AlertCircle, CheckCircle } from 'lucide-react';
import { useCopySources, useCreateMigration, useSelectableContent } from '../../services/api_content_migration';
import { buildCourseCopyRequest, dateShiftProblem, defaultDateShift, emptySelection, selectionCount } from '../../lib/contentMigration';
import ContentSelectionPicker from './ContentSelectionPicker';
import DateShiftFields from './DateShiftFields';

const inputCls = 'w-full rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500';

// CourseCopyPanel is "Copy a Canvas course": pick a course you teach,
// copy all of it or selected content, optionally shifting dates. The
// copy runs in the background; the migration history below polls it.
export default function CourseCopyPanel({ courseId }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const [sourceId, setSourceId] = useState('');
  const [mode, setMode] = useState('all');
  const [selection, setSelection] = useState(emptySelection());
  const [dateShift, setDateShift] = useState(defaultDateShift);
  const [notice, setNotice] = useState(null);

  const sources = useCopySources(courseId, search);
  const selectable = useSelectableContent(courseId, mode === 'select' ? sourceId : '');
  const create = useCreateMigration(courseId, {
    onSuccess: () => {
      setNotice({ ok: true, text: t('contentMigration.copy.queued', 'Course copy started. Its progress is shown in the history below.') });
      setSelection(emptySelection());
    },
    onError: (err) => setNotice({ ok: false, text: err.message || t('contentMigration.copy.failed', 'Could not start the copy') }),
  });

  const dsProblem = dateShiftProblem(dateShift);
  const canSubmit = !!sourceId && !create.isPending && !dsProblem && (mode === 'all' || selectionCount(selection) > 0);

  const submit = (e) => {
    e.preventDefault();
    setNotice(null);
    create.mutate(buildCourseCopyRequest({ sourceCourseId: sourceId, mode, selection, dateShift }));
  };

  const pickSource = (id) => {
    setSourceId(id);
    setSelection(emptySelection());
    const chosen = (sources.data || []).find((c) => String(c.id) === String(id));
    if (chosen && dateShift.mode === 'shift' && !dateShift.oldStart && chosen.start_at) {
      setDateShift({ ...dateShift, oldStart: chosen.start_at.slice(0, 10), oldEnd: chosen.end_at ? chosen.end_at.slice(0, 10) : '' });
    }
  };

  return (
    <form onSubmit={submit} className="space-y-5" aria-label={t('contentMigration.copy.title', 'Copy a course')}>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div>
          <label htmlFor="copy-search" className="block text-sm font-medium text-text-secondary mb-1">
            {t('contentMigration.copy.search', 'Search your courses')}
          </label>
          <input
            id="copy-search"
            type="search"
            className={inputCls}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('contentMigration.copy.searchPlaceholder', 'Course name or code')}
          />
        </div>
        <div>
          <label htmlFor="copy-source" className="block text-sm font-medium text-text-secondary mb-1">
            {t('contentMigration.copy.source', 'Copy from')}
          </label>
          <select id="copy-source" className={inputCls} value={sourceId} onChange={(e) => pickSource(e.target.value)}>
            <option value="">
              {sources.isLoading ? t('common.loading', 'Loading...') : t('contentMigration.copy.choose', 'Select a course')}
            </option>
            {(sources.data || []).map((c) => (
              <option key={c.id} value={c.id}>{c.label || c.name}</option>
            ))}
          </select>
          {sources.isError && <p className="mt-1 text-xs text-accent-danger">{sources.error?.message}</p>}
          {sources.isSuccess && (sources.data || []).length === 0 && (
            <p className="mt-1 text-xs text-text-tertiary">{t('contentMigration.copy.noSources', 'No other courses you teach match.')}</p>
          )}
        </div>
      </div>

      <fieldset>
        <legend className="text-sm font-medium text-text-primary mb-2">{t('contentMigration.copy.content', 'Content')}</legend>
        <div className="flex flex-wrap gap-4 text-sm text-text-secondary">
          <label className="inline-flex items-center gap-2">
            <input type="radio" name="copy-mode" value="all" checked={mode === 'all'} onChange={() => setMode('all')} />
            {t('contentMigration.copy.all', 'All content')}
          </label>
          <label className="inline-flex items-center gap-2">
            <input type="radio" name="copy-mode" value="select" checked={mode === 'select'} onChange={() => setMode('select')} />
            {t('contentMigration.copy.selectSpecific', 'Select specific content')}
          </label>
        </div>
        {mode === 'select' && (
          <div className="mt-3">
            {!sourceId && <p className="text-sm text-text-tertiary">{t('contentMigration.copy.pickFirst', 'Choose a course to see its content.')}</p>}
            {sourceId && selectable.isLoading && (
              <p className="text-sm text-text-tertiary inline-flex items-center gap-2"><Loader2 className="w-4 h-4 animate-spin" />{t('common.loading', 'Loading...')}</p>
            )}
            {sourceId && selectable.isError && <p className="text-sm text-accent-danger">{selectable.error?.message}</p>}
            {sourceId && selectable.isSuccess && (
              <ContentSelectionPicker types={selectable.data} selection={selection} onChange={setSelection} />
            )}
          </div>
        )}
      </fieldset>

      <DateShiftFields value={dateShift} onChange={setDateShift} idPrefix="copy-ds" />

      {notice && (
        <div
          role={notice.ok ? 'status' : 'alert'}
          className={`p-3 rounded-md text-sm flex items-center gap-2 ${notice.ok ? 'bg-accent-success/10 text-accent-success' : 'bg-accent-danger/10 text-accent-danger'}`}
        >
          {notice.ok ? <CheckCircle className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
          {notice.text}
        </div>
      )}

      <button
        type="submit"
        disabled={!canSubmit}
        className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md text-sm font-medium hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {create.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Copy className="w-4 h-4" />}
        {t('contentMigration.copy.submit', 'Copy course content')}
      </button>
    </form>
  );
}
