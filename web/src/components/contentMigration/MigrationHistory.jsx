import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CheckCircle, AlertCircle, Clock, Loader2 } from 'lucide-react';
import { isMigrationActive, summarizeCounts } from '../../lib/contentMigration';

const TYPE_LABEL_KEYS = {
  common_cartridge: 'contentImport.typeCommonCartridge',
  canvas_cartridge: 'contentImport.typeCanvasExport',
  qti_converter: 'contentImport.typeQtiQuiz',
  course_copy: 'contentImport.typeCourseCopy',
};

function StatusBadge({ state }) {
  const { t } = useTranslation();
  const map = {
    queued: [Clock, 'text-text-tertiary bg-surface-2', t('contentImport.statusQueued')],
    created: [Clock, 'text-text-tertiary bg-surface-2', t('contentImport.statusQueued')],
    pre_processing: [Loader2, 'text-brand-600 bg-brand-50', t('contentImport.statusProcessing')],
    running: [Loader2, 'text-brand-600 bg-brand-50', t('contentImport.statusProcessing')],
    completed: [CheckCircle, 'text-accent-success bg-accent-success/10', t('contentImport.statusCompleted')],
    failed: [AlertCircle, 'text-accent-danger bg-accent-danger/10', t('contentImport.statusFailed')],
  };
  const [Icon, cls, label] = map[state] || [Clock, 'text-text-tertiary bg-surface-2', state];
  return (
    <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-1 rounded-full ${cls}`}>
      <Icon className={`w-3 h-3 ${state === 'running' ? 'animate-spin' : ''}`} />
      {label}
    </span>
  );
}

function formatDate(s) {
  if (!s) return '-';
  return new Date(s).toLocaleString(undefined, { month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit' });
}

// MigrationHistory lists a course's content migrations (course copies
// and package imports) with live progress, result counts and issues.
export default function MigrationHistory({ migrations }) {
  const { t } = useTranslation();
  const [openIssues, setOpenIssues] = useState({});
  if (!migrations || migrations.length === 0) {
    return <div className="p-6 text-center text-text-tertiary">{t('contentImport.noImportsYet')}</div>;
  }
  return (
    <ul className="divide-y divide-border-default">
      {migrations.map((m) => {
        const results = m.results || {};
        const warnings = results.warnings || [];
        const summary = summarizeCounts(results.counts);
        return (
          <li key={m.id} className="p-4">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="text-sm font-medium text-text-primary">
                  {TYPE_LABEL_KEYS[m.migration_type] ? t(TYPE_LABEL_KEYS[m.migration_type]) : m.migration_type || t('contentImport.typeFallback')}
                  {m.migration_type === 'course_copy' && m.source_course_id && (
                    <span className="ml-1 text-text-tertiary font-normal">
                      {t('contentMigration.history.fromCourse', 'from course #{{id}}', { id: m.source_course_id })}
                    </span>
                  )}
                </div>
                <div className="text-xs text-text-tertiary">{formatDate(m.created_at)}</div>
                {summary && <div className="text-xs text-text-secondary mt-1">{summary}</div>}
                {m.workflow_state === 'failed' && m.error_message && (
                  <div className="text-xs text-accent-danger mt-1">{m.error_message}</div>
                )}
                {warnings.length > 0 && (
                  <button
                    type="button"
                    className="text-xs text-brand-600 hover:underline mt-1"
                    aria-expanded={!!openIssues[m.id]}
                    onClick={() => setOpenIssues({ ...openIssues, [m.id]: !openIssues[m.id] })}
                  >
                    {t('contentMigration.history.issues', '{{count}} issue(s)', { count: warnings.length })}
                  </button>
                )}
                {openIssues[m.id] && (
                  <ul className="mt-1 list-disc ml-5 text-xs text-text-secondary space-y-0.5">
                    {warnings.map((w, i) => <li key={i}>{w}</li>)}
                  </ul>
                )}
              </div>
              <div className="text-right shrink-0">
                <StatusBadge state={m.workflow_state} />
                {isMigrationActive(m) && (
                  <div
                    className="mt-2 w-32 bg-border-default rounded-full h-1.5"
                    role="progressbar"
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={m.progress || 0}
                    aria-label={t('contentMigration.history.progress', 'Progress')}
                  >
                    <div className="bg-brand-600 h-1.5 rounded-full transition-all" style={{ width: `${m.progress || 0}%` }} />
                  </div>
                )}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
