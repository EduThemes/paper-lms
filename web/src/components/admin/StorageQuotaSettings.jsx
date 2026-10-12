import React, { useEffect, useState } from 'react';
import { adminAccountsApi } from '../../services/api_admin_accounts';
import { useQuotaDefaults, useUpdateQuotaDefaults } from '../../services/adminAccountsQueries';
import { Section, Field, Button, ErrorNote, StatusNote, inputClass } from './AdminUI';
import { formatBytes } from '../../utils/adminAccounts';

const TYPES = [
  ['default_course_quota_mb', 'Course quota (MB)'],
  ['default_user_quota_mb', 'User quota (MB)'],
  ['default_group_quota_mb', 'Group quota (MB)'],
];

// Storage quotas (G120): account defaults plus a per-course / user /
// group override. Course and group uploads past the quota are refused.
export default function StorageQuotaSettings() {
  const query = useQuotaDefaults();
  const [form, setForm] = useState({});
  const [status, setStatus] = useState('');
  const save = useUpdateQuotaDefaults({
    onSuccess: () => {
      setStatus('Saved');
      setTimeout(() => setStatus(''), 1500);
    },
  });

  const [ctx, setCtx] = useState({ type: 'courses', id: '' });
  const [info, setInfo] = useState(null);
  const [override, setOverride] = useState('');
  const [ctxError, setCtxError] = useState(null);

  useEffect(() => {
    if (query.data) setForm(query.data);
  }, [query.data]);

  const lookup = async (e) => {
    e?.preventDefault();
    setCtxError(null);
    setInfo(null);
    try {
      const res = await adminAccountsApi.getContextQuota(ctx.type, ctx.id);
      setInfo(res);
      setOverride(res.overridden ? String(res.quota_mb) : '');
    } catch (err) {
      setCtxError(err);
    }
  };

  const applyOverride = async (clear) => {
    setCtxError(null);
    try {
      const res = await adminAccountsApi.setContextQuota(ctx.type, ctx.id, clear || override === '' ? null : Number(override));
      setInfo(res);
      setOverride(res.overridden ? String(res.quota_mb) : '');
    } catch (err) {
      setCtxError(err);
    }
  };

  return (
    <Section title="Storage quotas" description="Course and group file uploads that would exceed the quota are refused. Submissions never count.">
      <ErrorNote error={query.error || save.error} />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        {TYPES.map(([k, label]) => (
          <Field key={k} label={label} id={`quota-${k}`}>
            <input
              id={`quota-${k}`}
              type="number"
              min={0}
              className={inputClass}
              value={form[k] ?? ''}
              onChange={(e) => setForm({ ...form, [k]: e.target.value })}
            />
          </Field>
        ))}
      </div>
      <div className="mt-3 flex items-center gap-3">
        <Button
          disabled={save.isPending}
          onClick={() => save.mutate(Object.fromEntries(TYPES.map(([k]) => [k, Number(form[k]) || 0])))}
        >
          Save defaults
        </Button>
        <StatusNote message={status} />
      </div>

      <form onSubmit={lookup} className="mt-6 border-t border-border-default pt-4 grid grid-cols-1 md:grid-cols-[10rem_10rem_auto] gap-3 items-end">
        <Field label="Override for a" id="quota-ctx-type">
          <select id="quota-ctx-type" className={inputClass} value={ctx.type} onChange={(e) => setCtx({ ...ctx, type: e.target.value })}>
            <option value="courses">Course</option>
            <option value="users">User</option>
            <option value="groups">Group</option>
          </select>
        </Field>
        <Field label="ID" id="quota-ctx-id">
          <input id="quota-ctx-id" className={inputClass} inputMode="numeric" value={ctx.id} onChange={(e) => setCtx({ ...ctx, id: e.target.value.replace(/\D/g, '') })} />
        </Field>
        <Button type="submit" variant="secondary" disabled={!ctx.id}>Look up</Button>
      </form>
      <ErrorNote error={ctxError} />
      {info && (
        <div className="mt-3 text-sm space-y-2">
          <p>
            Using <strong>{formatBytes(info.quota_used)}</strong> of <strong>{info.quota_mb} MB</strong>
            {info.overridden ? ' (override)' : ' (account default)'}.
          </p>
          <div className="flex items-end gap-2">
            <Field label="Quota (MB)" id="quota-override">
              <input id="quota-override" type="number" min={0} className={inputClass} value={override} onChange={(e) => setOverride(e.target.value)} />
            </Field>
            <Button onClick={() => applyOverride(false)}>Set</Button>
            {info.overridden && <Button variant="secondary" onClick={() => applyOverride(true)}>Use default</Button>}
          </div>
        </div>
      )}
    </Section>
  );
}
