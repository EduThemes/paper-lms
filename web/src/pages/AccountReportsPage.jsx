// Account reports (G117): run provisioning, SIS export, grade export,
// last user access, unpublished / deleted courses and zero-activity
// reports, watch their progress and download the result.

import React, { useEffect, useState } from 'react';
import { Download, Play, Trash2 } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { api } from '../services/api';
import { adminUrls } from '../services/api_admin_accounts';
import { useAccountReports, useStartReport, useDeleteReportRun } from '../services/adminAccountsQueries';
import { Section, Field, Button, ErrorNote, Badge, inputClass, formatDateTime } from '../components/admin/AdminUI';
import { reportRunning, extractTerms } from '../utils/adminAccounts';

const ENTITIES = [
  ['users', 'Users'], ['logins', 'Logins'], ['terms', 'Terms'], ['courses', 'Courses'],
  ['sections', 'Sections'], ['enrollments', 'Enrollments'], ['groups', 'Groups'],
  ['group_memberships', 'Group memberships'], ['user_observers', 'Observers'],
];

const STATUS_TONE = { complete: 'good', error: 'bad', running: 'warn', created: 'warn' };

function ReportCard({ def, terms, onStart, onDelete, starting }) {
  const [open, setOpen] = useState(false);
  const [termId, setTermId] = useState('');
  const [includeDeleted, setIncludeDeleted] = useState(false);
  const [entities, setEntities] = useState(['users', 'courses', 'sections', 'enrollments']);
  const run = def.last_run;
  const params = def.parameters || [];

  const start = () => {
    const parameters = {};
    if (termId) parameters.enrollment_term_id = Number(termId);
    if (includeDeleted) parameters.include_deleted = true;
    if (params.includes('entities')) parameters.entities = entities;
    onStart(def.report, parameters);
    setOpen(false);
  };

  return (
    <div className="rounded-lg border border-border-default bg-surface-0 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="font-semibold text-text-primary">{def.title}</h3>
          <p className="text-sm text-text-secondary mt-0.5">{def.description}</p>
        </div>
        <Button onClick={() => (params.length ? setOpen((v) => !v) : start())} disabled={starting || reportRunning(run)}>
          <Play className="w-4 h-4" /> Run
        </Button>
      </div>
      {open && (
        <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3 border-t border-border-default pt-3">
          {params.includes('enrollment_term_id') && (
            <Field label="Term" id={`${def.report}-term`}>
              <select id={`${def.report}-term`} className={inputClass} value={termId} onChange={(e) => setTermId(e.target.value)}>
                <option value="">All terms</option>
                {terms.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
              </select>
            </Field>
          )}
          {params.includes('include_deleted') && (
            <label className="flex items-center gap-2 text-sm mt-6">
              <input type="checkbox" checked={includeDeleted} onChange={(e) => setIncludeDeleted(e.target.checked)} />
              Include deleted objects
            </label>
          )}
          {params.includes('entities') && (
            <fieldset className="md:col-span-2">
              <legend className="text-sm font-medium text-text-secondary mb-1">Include</legend>
              <div className="flex flex-wrap gap-3">
                {ENTITIES.map(([k, label]) => (
                  <label key={k} className="flex items-center gap-1.5 text-sm">
                    <input
                      type="checkbox"
                      checked={entities.includes(k)}
                      onChange={(e) => setEntities(e.target.checked ? [...entities, k] : entities.filter((x) => x !== k))}
                    />
                    {label}
                  </label>
                ))}
              </div>
            </fieldset>
          )}
          <div className="md:col-span-2">
            <Button onClick={start} disabled={starting || (params.includes('entities') && entities.length === 0)}>Run report</Button>
          </div>
        </div>
      )}
      {run && (
        <div className="mt-3 flex flex-wrap items-center gap-3 text-sm border-t border-border-default pt-3">
          <Badge tone={STATUS_TONE[run.status] || 'neutral'}>{run.status}</Badge>
          {reportRunning(run) && <span className="text-text-secondary">{run.progress}%</span>}
          <span className="text-text-tertiary">Last run {formatDateTime(run.created_at)}</span>
          {run.message && <span className="text-text-secondary">{run.message}</span>}
          {run.downloadable && (
            <a className="inline-flex items-center gap-1 text-brand-600 hover:underline" href={adminUrls.reportDownload(def.report, run.id)}>
              <Download className="w-4 h-4" /> {run.file_name}
            </a>
          )}
          {!reportRunning(run) && (
            <button type="button" className="ml-auto text-text-tertiary hover:text-accent-danger" aria-label={`Delete the last ${def.title} run`} onClick={() => onDelete(def.report, run.id)}>
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>
      )}
    </div>
  );
}

const AccountReportsPage = () => {
  const [polling, setPolling] = useState(false);
  const query = useAccountReports({ refetchInterval: polling ? 2000 : false });
  const start = useStartReport();
  const del = useDeleteReportRun();
  const [terms, setTerms] = useState([]);

  useEffect(() => {
    const running = (query.data || []).some((d) => reportRunning(d.last_run));
    setPolling(running);
  }, [query.data]);

  useEffect(() => {
    let alive = true;
    if (typeof api.getEnrollmentTerms === 'function') {
      Promise.resolve(api.getEnrollmentTerms(undefined, 1, 100))
        .then((res) => { if (alive) setTerms(extractTerms(res)); })
        .catch(() => {});
    }
    return () => { alive = false; };
  }, []);

  return (
    <Page query={query} title="Reports">
      {(defs) => (
        <Layout>
          <div className="p-8 max-w-5xl">
            <h1 className="text-2xl font-bold text-text-primary">Reports</h1>
            <p className="text-sm text-text-secondary mt-1 mb-6">Account-wide CSV reports. They run in the background; this page updates when they finish.</p>
            <ErrorNote error={start.error || del.error} />
            <Section title="Available reports">
              <div className="grid grid-cols-1 gap-4">
                {defs.map((d) => (
                  <ReportCard
                    key={d.report}
                    def={d}
                    terms={terms}
                    starting={start.isPending}
                    onStart={(report, parameters) => start.mutate({ report, parameters }, { onSuccess: () => setPolling(true) })}
                    onDelete={(report, id) => del.mutate({ report, id })}
                  />
                ))}
              </div>
            </Section>
          </div>
        </Layout>
      )}
    </Page>
  );
};

export default AccountReportsPage;
