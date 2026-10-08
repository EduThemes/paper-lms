import React, { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, FileBarChart, History, Loader2, Play, Trash2 } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { api } from '../services/api';
import {
  accountReportFileUrl,
  deleteAccountReportRun,
  isReportRunning,
  listAccountReportRuns,
  listAccountReports,
  startAccountReport,
} from '../services/api_api';

// Account Reports (G138): Canvas's /api/v1/accounts/:id/reports. Each
// report runs in the background on the server; the page polls while a
// run is in progress and offers the finished CSV / ZIP for download.

const ACCOUNT = 'self';
const reportsKey = ['accountReports', ACCOUNT];
const runsKey = (report) => ['accountReports', ACCOUNT, report, 'runs'];

const STATUS_STYLE = {
  complete: 'bg-accent-success/10 text-accent-success border-accent-success/30',
  error: 'bg-accent-danger/10 text-accent-danger border-accent-danger/30',
  aborted: 'bg-surface-1 text-text-secondary border-border-default',
  running: 'bg-brand-50 text-brand-700 border-brand-200',
  created: 'bg-brand-50 text-brand-700 border-brand-200',
};

const BOOLEAN_PARAMS = ['users', 'courses', 'sections', 'enrollments'];

function formatWhen(value) {
  if (!value) return '—';
  return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function StatusBadge({ run }) {
  if (!run) return <span className="text-xs text-text-tertiary">Never run</span>;
  const label = run.status === 'created' ? 'queued' : run.status;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${STATUS_STYLE[run.status] || STATUS_STYLE.aborted}`}>
      {isReportRunning(run) && <Loader2 className="w-3 h-3 animate-spin" aria-hidden="true" />}
      {label}
    </span>
  );
}

function RunHistory({ report }) {
  const queryClient = useQueryClient();
  const runs = useQuery({
    queryKey: runsKey(report),
    queryFn: () => listAccountReportRuns(ACCOUNT, report),
    refetchInterval: (q) => ((q.state.data || []).some(isReportRunning) ? 3000 : false),
  });
  const remove = useMutation({
    mutationFn: (id) => deleteAccountReportRun(ACCOUNT, report, id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: runsKey(report) });
      queryClient.invalidateQueries({ queryKey: reportsKey });
    },
  });

  if (runs.isLoading) return <p className="text-xs text-text-tertiary mt-3">Loading history…</p>;
  if (runs.isError) return <p className="text-xs text-accent-danger mt-3">Could not load the history.</p>;
  const list = runs.data || [];
  if (list.length === 0) return <p className="text-xs text-text-tertiary mt-3">No runs yet.</p>;
  return (
    <table className="mt-3 w-full text-xs">
      <thead className="text-text-tertiary">
        <tr>
          <th className="py-1 text-left font-medium">Started</th>
          <th className="py-1 text-left font-medium">Status</th>
          <th className="py-1 text-left font-medium">File</th>
          <th className="py-1" />
        </tr>
      </thead>
      <tbody>
        {list.map((run) => (
          <tr key={run.id} className="border-t border-border-default">
            <td className="py-1.5 text-text-secondary">{formatWhen(run.created_at)}</td>
            <td className="py-1.5"><StatusBadge run={run} /></td>
            <td className="py-1.5">
              {run.status === 'complete' && run.attachment ? (
                <a className="text-brand-600 hover:text-brand-800" href={accountReportFileUrl(ACCOUNT, report, run.id)}>
                  {run.attachment.filename}
                </a>
              ) : (
                <span className="text-text-tertiary">{run.message || '—'}</span>
              )}
            </td>
            <td className="py-1.5 text-right">
              <button
                type="button"
                onClick={() => remove.mutate(run.id)}
                disabled={remove.isPending}
                className="text-text-tertiary hover:text-accent-danger disabled:opacity-50"
                aria-label={`Delete run from ${formatWhen(run.created_at)}`}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ReportCard({ report, terms }) {
  const queryClient = useQueryClient();
  const paramNames = Object.keys(report.parameters || {});
  const booleanParams = paramNames.filter((p) => BOOLEAN_PARAMS.includes(p));
  const takesTerm = paramNames.includes('enrollment_term_id') && booleanParams.length === 0;
  const [values, setValues] = useState(() => Object.fromEntries(booleanParams.map((p) => [p, true])));
  const [termId, setTermId] = useState('');
  const [showHistory, setShowHistory] = useState(false);

  const run = useMutation({
    mutationFn: () => {
      const parameters = { ...values };
      if (takesTerm && termId) parameters.enrollment_term_id = termId;
      return startAccountReport(ACCOUNT, report.report, parameters);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reportsKey });
      queryClient.invalidateQueries({ queryKey: runsKey(report.report) });
    },
  });

  const last = report.last_run;
  const noneChosen = booleanParams.length > 0 && !booleanParams.some((p) => values[p]);

  return (
    <section className="rounded-lg border border-border-default bg-surface-0 p-5" aria-labelledby={`report-${report.report}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 id={`report-${report.report}`} className="font-semibold text-text-primary">{report.title}</h2>
          <p className="text-sm text-text-secondary mt-0.5">{report.description}</p>
        </div>
        <StatusBadge run={last} />
      </div>

      {booleanParams.length > 0 && (
        <fieldset className="mt-3 flex flex-wrap gap-4">
          <legend className="sr-only">Include</legend>
          {booleanParams.map((p) => (
            <label key={p} className="inline-flex items-center gap-1.5 text-sm text-text-secondary capitalize">
              <input
                type="checkbox"
                checked={!!values[p]}
                onChange={(e) => setValues((v) => ({ ...v, [p]: e.target.checked }))}
              />
              {p}
            </label>
          ))}
        </fieldset>
      )}

      {takesTerm && (
        <div className="mt-3">
          <label htmlFor={`term-${report.report}`} className="block text-xs font-medium text-text-secondary mb-1">Term</label>
          <select
            id={`term-${report.report}`}
            value={termId}
            onChange={(e) => setTermId(e.target.value)}
            className="rounded-md border border-border-strong bg-surface-0 px-2 py-1 text-sm"
          >
            <option value="">All terms</option>
            {terms.map((t) => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </select>
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={() => run.mutate()}
          disabled={run.isPending || isReportRunning(last) || noneChosen}
          className="inline-flex items-center gap-1.5 rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          <Play className="w-3.5 h-3.5" />
          Run report
        </button>
        {last?.status === 'complete' && last.attachment && (
          <a
            href={accountReportFileUrl(ACCOUNT, report.report, last.id)}
            className="inline-flex items-center gap-1.5 text-sm text-brand-600 hover:text-brand-800"
          >
            <Download className="w-3.5 h-3.5" />
            Download {last.attachment.filename}
          </a>
        )}
        {last && <span className="text-xs text-text-tertiary">Last run {formatWhen(last.created_at)}</span>}
        <button
          type="button"
          onClick={() => setShowHistory((v) => !v)}
          className="ml-auto inline-flex items-center gap-1 text-xs text-text-secondary hover:text-text-primary"
          aria-expanded={showHistory}
        >
          <History className="w-3.5 h-3.5" />
          History
        </button>
      </div>
      {run.isError && (
        <p role="alert" className="mt-2 text-sm text-accent-danger">{run.error?.message || 'Could not start the report.'}</p>
      )}
      {showHistory && <RunHistory report={report.report} />}
    </section>
  );
}

const AccountReportsPage = () => {
  const query = useQuery({
    queryKey: reportsKey,
    queryFn: () => listAccountReports(ACCOUNT),
    refetchInterval: (q) => ((q.state.data || []).some((r) => isReportRunning(r.last_run)) ? 3000 : false),
  });
  const terms = useQuery({
    queryKey: ['accountReports', ACCOUNT, 'terms'],
    queryFn: async () => {
      const res = await api.getEnrollmentTerms(ACCOUNT, 1, 100);
      const data = res?.data;
      if (Array.isArray(data)) return data;
      return Array.isArray(data?.enrollment_terms) ? data.enrollment_terms : [];
    },
  });

  return (
    <Page query={query} title="Account Reports" empty={(list) => (list?.length ?? 0) === 0} emptyMessage="No reports are available.">
      {(reports) => (
        <Layout>
          <div className="p-8 max-w-4xl">
            <div className="flex items-center gap-3 mb-2">
              <FileBarChart className="w-6 h-6 text-brand-600" aria-hidden="true" />
              <h1 className="text-2xl font-bold text-text-primary">Account Reports</h1>
            </div>
            <p className="text-sm text-text-secondary mb-6">
              Reports run in the background. They are also available to scripts through the Canvas-compatible
              <code className="mx-1 rounded bg-surface-2 px-1 text-xs">/api/v1/accounts/self/reports</code>
              API.
            </p>
            <div className="space-y-4">
              {reports.map((r) => (
                <ReportCard key={r.report} report={r} terms={terms.data || []} />
              ))}
            </div>
          </div>
        </Layout>
      )}
    </Page>
  );
};

export default AccountReportsPage;
