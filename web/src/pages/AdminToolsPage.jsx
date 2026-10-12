// Admin tools (G119): restore deleted courses, look up a user's sign-in
// history / activity / grade changes, and browse the account audit log.

import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { RotateCcw, Search } from 'lucide-react';
import Layout from '../components/Layout';
import { api } from '../services/api';
import {
  useDeletedCourses, useRestoreCourse, useAdminUserHistory, useAccountAuditLog,
} from '../services/adminAccountsQueries';
import { Section, Field, Button, ErrorNote, StatusNote, inputClass, formatDateTime } from '../components/admin/AdminUI';

function useDebounced(value, ms = 300) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

function RestoreCourses() {
  const [term, setTerm] = useState('');
  const debounced = useDebounced(term.trim());
  const query = useDeletedCourses(debounced);
  const [status, setStatus] = useState('');
  const restore = useRestoreCourse({
    onSuccess: (c) => {
      setStatus(`Restored "${c.name}" as unpublished.`);
      setTimeout(() => setStatus(''), 3000);
    },
  });
  const courses = query.data || [];
  return (
    <Section title="Restore deleted courses" description="A restored course comes back unpublished, with its content, enrollments and grades.">
      <ErrorNote error={query.error || restore.error} />
      <Field label="Find a deleted course" id="deleted-course-search">
        <input id="deleted-course-search" className={inputClass} placeholder="Name, code, SIS ID or course ID" value={term} onChange={(e) => setTerm(e.target.value)} />
      </Field>
      <StatusNote message={status} />
      <table className="w-full text-sm mt-3">
        <thead className="text-left text-text-secondary"><tr><th className="py-1">Course</th><th>SIS ID</th><th>Deleted</th><th /></tr></thead>
        <tbody>
          {courses.length === 0 && !query.isLoading && (
            <tr><td colSpan={4} className="py-3 text-text-tertiary">No deleted courses match.</td></tr>
          )}
          {courses.map((c) => (
            <tr key={c.id} className="border-t border-border-default">
              <td className="py-2">{c.name} <span className="text-text-tertiary">{c.course_code} · #{c.id}</span></td>
              <td>{c.sis_course_id || '—'}</td>
              <td>{formatDateTime(c.deleted_at)}</td>
              <td className="text-right">
                <Button variant="secondary" disabled={restore.isPending} onClick={() => restore.mutate(c.id)}>
                  <RotateCcw className="w-4 h-4" /> Restore
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Section>
  );
}

function UserLogs() {
  const [term, setTerm] = useState('');
  const [results, setResults] = useState([]);
  const [selected, setSelected] = useState(null);
  const debounced = useDebounced(term.trim());
  const history = useAdminUserHistory(selected?.id);

  useEffect(() => {
    let alive = true;
    if (debounced.length < 2) {
      setResults([]);
      return undefined;
    }
    api.searchUsers(debounced, 1, 10)
      .then((res) => { if (alive) setResults(res?.data || []); })
      .catch(() => { if (alive) setResults([]); });
    return () => { alive = false; };
  }, [debounced]);

  const data = history.data || { logins: [], activity: [], grade_changes: [] };
  return (
    <Section title="Logs by user" description="Sign-ins, audited actions (including actions taken while someone acted as the user) and grade changes.">
      <Field label="Find a user" id="logs-user-search">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
          <input id="logs-user-search" className={`${inputClass} pl-9`} placeholder="Name or email" value={term} onChange={(e) => setTerm(e.target.value)} />
        </div>
      </Field>
      {results.length > 0 && !selected && (
        <ul className="mt-2 rounded-md border border-border-default">
          {results.map((u) => (
            <li key={u.id}>
              <button type="button" className="w-full text-left px-3 py-1.5 text-sm hover:bg-surface-1" onClick={() => { setSelected(u); setTerm(''); }}>
                {u.name} <span className="text-text-tertiary">{u.email}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {selected && (
        <div className="mt-4">
          <div className="flex items-center gap-3 mb-3">
            <h3 className="font-semibold">{selected.name}</h3>
            <Link to={`/admin/users/${selected.id}`} className="text-sm text-brand-600 hover:underline">Open user</Link>
            <Button variant="secondary" onClick={() => setSelected(null)}>Clear</Button>
          </div>
          <ErrorNote error={history.error} />
          <h4 className="text-sm font-medium text-text-secondary">Sign-ins</h4>
          <ul className="text-sm mb-3">
            {data.logins.length === 0 && <li className="text-text-tertiary">None recorded.</li>}
            {data.logins.map((ev) => (
              <li key={ev.id} className="border-t border-border-default py-1">{formatDateTime(ev.created_at)} · {ev.event_type.replace('auth.', '')} · {ev.ip_address || '—'}</li>
            ))}
          </ul>
          <h4 className="text-sm font-medium text-text-secondary">Activity</h4>
          <ul className="text-sm mb-3">
            {data.activity.length === 0 && <li className="text-text-tertiary">None recorded.</li>}
            {data.activity.map((ev) => (
              <li key={ev.id} className="border-t border-border-default py-1">{formatDateTime(ev.created_at)} · {ev.action || ev.event_type}{ev.real_user_id ? ` (by user #${ev.real_user_id})` : ''}</li>
            ))}
          </ul>
          <h4 className="text-sm font-medium text-text-secondary">Grade changes</h4>
          <ul className="text-sm">
            {data.grade_changes.length === 0 && <li className="text-text-tertiary">None recorded.</li>}
            {data.grade_changes.map((gc) => (
              <li key={gc.id} className="border-t border-border-default py-1">{formatDateTime(gc.created_at)} · course #{gc.course_id} · assignment #{gc.assignment_id} · {gc.old_grade || '—'} → {gc.new_grade || '—'}</li>
            ))}
          </ul>
        </div>
      )}
    </Section>
  );
}

function AccountActivity() {
  const [page, setPage] = useState(1);
  const [eventType, setEventType] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const filters = { event_type: eventType, date_from: dateFrom };
  const query = useAccountAuditLog(page, filters);
  const rows = query.data?.data || [];
  const hasNext = Boolean(query.data?.pagination?.next);
  return (
    <Section title="Account activity" description="Every audited change in the account, newest first.">
      <ErrorNote error={query.error} />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-3">
        <Field label="Event type" id="audit-event-type">
          <input id="audit-event-type" className={inputClass} placeholder="e.g. http.write, masquerade_start" value={eventType} onChange={(e) => { setEventType(e.target.value.trim()); setPage(1); }} />
        </Field>
        <Field label="From" id="audit-date-from">
          <input id="audit-date-from" type="date" className={inputClass} value={dateFrom} onChange={(e) => { setDateFrom(e.target.value); setPage(1); }} />
        </Field>
      </div>
      <table className="w-full text-sm">
        <thead className="text-left text-text-secondary"><tr><th className="py-1">When</th><th>Event</th><th>User</th><th>Action</th></tr></thead>
        <tbody>
          {rows.map((ev) => (
            <tr key={ev.id} className="border-t border-border-default">
              <td className="py-1.5 whitespace-nowrap">{formatDateTime(ev.created_at)}</td>
              <td>{ev.event_type}</td>
              <td>{ev.user_id ? <Link to={`/admin/users/${ev.user_id}`} className="text-brand-600 hover:underline">#{ev.user_id}</Link> : '—'}{ev.real_user_id ? ` (as #${ev.real_user_id})` : ''}</td>
              <td className="text-text-secondary">{ev.action}</td>
            </tr>
          ))}
          {rows.length === 0 && !query.isLoading && <tr><td colSpan={4} className="py-3 text-text-tertiary">No activity.</td></tr>}
        </tbody>
      </table>
      <div className="mt-3 flex items-center gap-2">
        <Button variant="secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>Previous</Button>
        <span className="text-sm text-text-secondary">Page {page}</span>
        <Button variant="secondary" disabled={!hasNext} onClick={() => setPage(page + 1)}>Next</Button>
      </div>
    </Section>
  );
}

const TABS = [['restore', 'Restore courses'], ['logs', 'Logs by user'], ['activity', 'Account activity']];

const AdminToolsPage = () => {
  const [tab, setTab] = useState('restore');
  return (
    <Layout>
      <div className="p-8 max-w-5xl">
        <h1 className="text-2xl font-bold text-text-primary">Admin tools</h1>
        <div className="flex gap-2 my-6" role="tablist">
          {TABS.map(([k, label]) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={tab === k}
              onClick={() => setTab(k)}
              className={`px-3 py-1.5 rounded-full text-sm font-medium ${tab === k ? 'bg-brand-600 text-white' : 'bg-surface-1 text-text-secondary'}`}
            >
              {label}
            </button>
          ))}
        </div>
        {tab === 'restore' && <RestoreCourses />}
        {tab === 'logs' && <UserLogs />}
        {tab === 'activity' && <AccountActivity />}
      </div>
    </Layout>
  );
};

export default AdminToolsPage;
