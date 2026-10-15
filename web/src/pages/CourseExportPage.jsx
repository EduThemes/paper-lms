import React, { useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Download, Package, Loader2, CheckCircle, AlertCircle, Clock } from 'lucide-react';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import ContentSelectionPicker from '../components/contentMigration/ContentSelectionPicker';
import {
  contentMigrationApi,
  useContentExports,
  useCreateExport,
} from '../services/api_content_migration';
import {
  emptySelection,
  exportCounts,
  formatBytes,
  isJobActive,
  selectionCount,
  summarizeCounts,
  toSelectSettings,
} from '../lib/contentMigration';
import { useQuery } from '@tanstack/react-query';

function ExportStatus({ exp }) {
  const { t } = useTranslation();
  if (exp.workflow_state === 'exported') {
    return <span className="inline-flex items-center gap-1 text-xs font-medium text-accent-success"><CheckCircle className="w-3 h-3" />{t('contentMigration.export.ready', 'Ready')}</span>;
  }
  if (exp.workflow_state === 'failed') {
    return <span className="inline-flex items-center gap-1 text-xs font-medium text-accent-danger"><AlertCircle className="w-3 h-3" />{t('contentImport.statusFailed')}</span>;
  }
  if (exp.workflow_state === 'exporting') {
    return <span className="inline-flex items-center gap-1 text-xs font-medium text-brand-600"><Loader2 className="w-3 h-3 animate-spin" />{t('contentMigration.export.exporting', 'Exporting')} {exp.progress ? `${exp.progress}%` : ''}</span>;
  }
  return <span className="inline-flex items-center gap-1 text-xs font-medium text-text-tertiary"><Clock className="w-3 h-3" />{t('contentImport.statusQueued')}</span>;
}

// CourseExportPage is "Export Course Content" (G066/G142): build an IMS
// Common Cartridge (.imscc) of the whole course or of selected content,
// and download finished packages. Exports run in the background; the
// list polls until they finish.
const CourseExportPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const exports = useContentExports(isTeacher ? courseId : null);
  const [mode, setMode] = useState('all');
  const [selection, setSelection] = useState(emptySelection());
  const [error, setError] = useState(null);
  const selectable = useQuery({
    queryKey: ['contentExports', String(courseId), 'selectable'],
    queryFn: () => contentMigrationApi.selectableContent(courseId, courseId),
    enabled: !!isTeacher && mode === 'select',
    staleTime: 30_000,
  });
  const create = useCreateExport(courseId, {
    onSuccess: () => { setError(null); setSelection(emptySelection()); },
    onError: (err) => setError(err.message || t('contentMigration.export.failed', 'Could not start the export')),
  });

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) {
    return <Layout><div className="py-12 text-center text-text-tertiary">{t('common.loading')}</div></Layout>;
  }

  const submit = (e) => {
    e.preventDefault();
    create.mutate(mode === 'select' ? toSelectSettings(selection) : {});
  };
  const busy = create.isPending || (exports.data || []).some(isJobActive);

  return (
    <Page query={exports} title={t('contentMigration.export.title', 'Export Course Content')}>
      {(list) => (
        <Layout>
          <CourseNav />
          <div className="mb-6">
            <Link to={`/courses/${courseId}/content_import`} className="text-brand-600 hover:underline text-sm">
              {t('contentMigration.export.backToImport', '← Import course content')}
            </Link>
            <h2 className="text-2xl font-bold text-text-primary mt-2">{t('contentMigration.export.title', 'Export Course Content')}</h2>
            <p className="text-text-tertiary text-sm mt-1">
              {t('contentMigration.export.description', 'Download this course as an IMS Common Cartridge (.imscc) package that Paper, Canvas and other LMSs can import.')}
            </p>
          </div>

          <form onSubmit={submit} className="bg-surface-0 rounded-lg shadow p-6 mb-6 space-y-4">
            <fieldset>
              <legend className="text-sm font-medium text-text-primary mb-2">{t('contentMigration.copy.content', 'Content')}</legend>
              <div className="flex flex-wrap gap-4 text-sm text-text-secondary">
                <label className="inline-flex items-center gap-2">
                  <input type="radio" name="export-mode" value="all" checked={mode === 'all'} onChange={() => setMode('all')} />
                  {t('contentMigration.export.all', 'The whole course')}
                </label>
                <label className="inline-flex items-center gap-2">
                  <input type="radio" name="export-mode" value="select" checked={mode === 'select'} onChange={() => setMode('select')} />
                  {t('contentMigration.copy.selectSpecific', 'Select specific content')}
                </label>
              </div>
            </fieldset>
            {mode === 'select' && (
              selectable.isLoading
                ? <p className="text-sm text-text-tertiary">{t('common.loading')}</p>
                : selectable.isError
                  ? <p className="text-sm text-accent-danger">{selectable.error?.message}</p>
                  : <ContentSelectionPicker types={selectable.data} selection={selection} onChange={setSelection} />
            )}
            {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
            <button
              type="submit"
              disabled={create.isPending || (mode === 'select' && selectionCount(selection) === 0)}
              className="inline-flex items-center gap-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md text-sm font-medium hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : <Package className="w-4 h-4" />}
              {t('contentMigration.export.submit', 'Create export')}
            </button>
          </form>

          <div className="bg-surface-0 rounded-lg shadow">
            <div className="p-4 border-b border-border-default">
              <h3 className="font-semibold text-text-primary">{t('contentMigration.export.history', 'Exports')}</h3>
            </div>
            {list.length === 0 ? (
              <div className="p-6 text-center text-text-tertiary">{t('contentMigration.export.none', 'No exports yet.')}</div>
            ) : (
              <ul className="divide-y divide-border-default">
                {list.map((exp) => (
                  <li key={exp.id} className="p-4 flex items-center justify-between gap-4">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-text-primary truncate">
                        {exp.attachment?.display_name || t('contentMigration.export.item', 'Course export #{{id}}', { id: exp.id })}
                      </div>
                      <div className="text-xs text-text-tertiary">
                        {new Date(exp.created_at).toLocaleString()}
                        {exp.attachment?.size ? ` · ${formatBytes(exp.attachment.size)}` : ''}
                      </div>
                      {exp.results && <div className="text-xs text-text-secondary mt-1">{summarizeCounts(exportCounts(exp.results))}</div>}
                      {exp.workflow_state === 'failed' && exp.error_message && (
                        <div className="text-xs text-accent-danger mt-1">{exp.error_message}</div>
                      )}
                    </div>
                    <div className="flex items-center gap-3 shrink-0">
                      <ExportStatus exp={exp} />
                      {exp.workflow_state === 'exported' && (
                        <a
                          href={contentMigrationApi.exportDownloadUrl(courseId, exp.id)}
                          className="inline-flex items-center gap-1 text-sm font-medium text-brand-600 hover:underline"
                          download
                        >
                          <Download className="w-4 h-4" />
                          {t('contentMigration.export.download', 'Download')}
                        </a>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </Layout>
      )}
    </Page>
  );
};

export default CourseExportPage;
