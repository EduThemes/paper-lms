// Admin user detail (G113, G114, G115, G119, G121): edit another user's
// profile and primary login, suspend / force a password reset, act as
// the user, manage their additional logins and observer links, and read
// their enrollments, sign-in history, activity and grade changes.

import React, { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, UserCheck, Trash2, Plus } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';
import { adminAccountsApi } from '../services/api_admin_accounts';
import {
  useAdminUser, useAdminUserHistory, useUpdateAdminUser, useUserStatusMutation,
  useLoginMutation, useObserverLinkMutation,
} from '../services/adminAccountsQueries';
import {
  Section, Field, Button, ErrorNote, StatusNote, Badge, inputClass, formatDateTime,
} from '../components/admin/AdminUI';

const PROFILE_FIELDS = [
  ['name', 'Full name'],
  ['short_name', 'Display name'],
  ['sortable_name', 'Sortable name'],
  ['email', 'Email'],
  ['login_id', 'Primary login'],
  ['sis_user_id', 'SIS user ID'],
  ['time_zone', 'Time zone'],
];

function ProfileForm({ user }) {
  const [form, setForm] = useState({});
  const [password, setPassword] = useState('');
  const [status, setStatus] = useState('');
  const update = useUpdateAdminUser(user.id, {
    onSuccess: () => {
      setStatus('Saved');
      setPassword('');
      setTimeout(() => setStatus(''), 1500);
    },
  });

  useEffect(() => {
    const next = {};
    PROFILE_FIELDS.forEach(([k]) => { next[k] = user[k] ?? ''; });
    setForm(next);
  }, [user]);

  const submit = (e) => {
    e.preventDefault();
    const payload = {};
    PROFILE_FIELDS.forEach(([k]) => {
      if ((form[k] ?? '') !== (user[k] ?? '')) payload[k] = form[k];
    });
    if (password) payload.password = password;
    if (Object.keys(payload).length === 0) return;
    update.mutate(payload);
  };

  const disabled = !user.permissions?.can_manage;
  return (
    <form onSubmit={submit}>
      <ErrorNote error={update.error} />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {PROFILE_FIELDS.map(([k, label]) => (
          <Field key={k} label={label} id={`user-${k}`}>
            <input
              id={`user-${k}`}
              className={inputClass}
              value={form[k] ?? ''}
              disabled={disabled}
              onChange={(e) => setForm({ ...form, [k]: e.target.value })}
            />
          </Field>
        ))}
        <Field label="Set a new password" id="user-password" hint="Leave blank to keep the current password.">
          <input
            id="user-password"
            type="password"
            autoComplete="new-password"
            className={inputClass}
            value={password}
            disabled={disabled}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
      </div>
      <div className="mt-4 flex items-center gap-3">
        <Button type="submit" disabled={disabled || update.isPending}>Save profile</Button>
        <StatusNote message={status} />
        {disabled && <span className="text-xs text-text-tertiary">Only a super admin can change this user.</span>}
      </div>
    </form>
  );
}

function StatusActions({ user }) {
  const navigate = useNavigate();
  const { refreshUser } = useAuth();
  const [actError, setActError] = useState(null);
  const statusMutation = useUserStatusMutation(user.id);
  const canManage = user.permissions?.can_manage;

  const actAs = async () => {
    setActError(null);
    try {
      await adminAccountsApi.startMasquerade(user.id);
      await refreshUser();
      navigate('/');
    } catch (err) {
      setActError(err);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <ErrorNote error={statusMutation.error || actError} />
      {user.permissions?.can_masquerade && (
        <Button variant="secondary" onClick={actAs}>
          <UserCheck className="w-4 h-4" /> Act as user
        </Button>
      )}
      {canManage && (
        <>
          <Button
            variant={user.suspended ? 'secondary' : 'danger'}
            disabled={statusMutation.isPending}
            onClick={() => statusMutation.mutate({ action: user.suspended ? 'unsuspend' : 'suspend' })}
          >
            {user.suspended ? 'Unsuspend' : 'Suspend'}
          </Button>
          <Button
            variant="secondary"
            disabled={statusMutation.isPending || user.requires_password_reset}
            onClick={() => statusMutation.mutate({ action: 'reset' })}
          >
            {user.requires_password_reset ? 'Password reset pending' : 'Require password reset'}
          </Button>
        </>
      )}
    </div>
  );
}

function LoginsSection({ user }) {
  const [draft, setDraft] = useState({ unique_id: '', password: '', sis_user_id: '', authentication_provider_id: '' });
  const [providers, setProviders] = useState([]);
  const mutation = useLoginMutation(user.id, {
    onSuccess: () => setDraft({ unique_id: '', password: '', sis_user_id: '', authentication_provider_id: '' }),
  });
  const canManage = user.permissions?.can_manage;

  useEffect(() => {
    let alive = true;
    if (typeof api.getAuthProviders === 'function') {
      Promise.resolve(api.getAuthProviders())
        .then((res) => { if (alive) setProviders(Array.isArray(res) ? res : res?.data || []); })
        .catch(() => {});
    }
    return () => { alive = false; };
  }, []);

  const add = (e) => {
    e.preventDefault();
    if (!draft.unique_id.trim()) return;
    const login = { unique_id: draft.unique_id.trim() };
    if (draft.sis_user_id.trim()) login.sis_user_id = draft.sis_user_id.trim();
    if (draft.authentication_provider_id) login.authentication_provider_id = Number(draft.authentication_provider_id);
    else if (draft.password) login.password = draft.password;
    mutation.mutate({ action: 'create', login });
  };

  return (
    <Section title="Logins" description="Additional ways this person signs in: a second username, or the name an SSO provider sends.">
      <ErrorNote error={mutation.error} />
      <table className="w-full text-sm mb-4">
        <thead className="text-text-secondary text-left">
          <tr><th className="py-1">Login</th><th>SIS ID</th><th>Sign-in</th><th>Status</th><th>Last used</th><th><span className="sr-only">Actions</span></th></tr>
        </thead>
        <tbody>
          <tr className="border-t border-border-default">
            <td className="py-2 font-medium">{user.login_id}</td>
            <td>{user.sis_user_id || '—'}</td>
            <td>Password (primary)</td>
            <td><Badge tone="good">active</Badge></td>
            <td>—</td>
            <td />
          </tr>
          {user.logins.map((l) => (
            <tr key={l.id} className="border-t border-border-default">
              <td className="py-2 font-medium">{l.unique_id}</td>
              <td>{l.sis_user_id || '—'}</td>
              <td>{l.authentication_provider_id ? `SSO provider #${l.authentication_provider_id}` : (l.has_password ? 'Own password' : "User's password")}</td>
              <td><Badge tone={l.workflow_state === 'active' ? 'good' : 'warn'}>{l.workflow_state}</Badge></td>
              <td>{formatDateTime(l.last_login_at)}</td>
              <td className="text-right whitespace-nowrap">
                {canManage && (
                  <>
                    <Button
                      variant="secondary"
                      className="mr-2"
                      aria-label={`${l.workflow_state === 'active' ? 'Suspend' : 'Activate'} login ${l.unique_id}`}
                      onClick={() => mutation.mutate({ action: 'update', loginId: l.id, login: { workflow_state: l.workflow_state === 'active' ? 'suspended' : 'active' } })}
                    >
                      {l.workflow_state === 'active' ? 'Suspend' : 'Activate'}
                    </Button>
                    <Button variant="danger" aria-label={`Delete login ${l.unique_id}`} onClick={() => mutation.mutate({ action: 'delete', loginId: l.id })}>
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {canManage && (
        <form onSubmit={add} className="grid grid-cols-1 md:grid-cols-5 gap-3 items-end">
          <Field label="New login" id="new-login-id">
            <input id="new-login-id" className={inputClass} value={draft.unique_id} onChange={(e) => setDraft({ ...draft, unique_id: e.target.value })} />
          </Field>
          <Field label="Sign-in method" id="new-login-provider">
            <select id="new-login-provider" className={inputClass} value={draft.authentication_provider_id} onChange={(e) => setDraft({ ...draft, authentication_provider_id: e.target.value })}>
              <option value="">Password</option>
              {providers.map((p) => (
                <option key={p.id} value={p.id}>{(p.auth_type || 'sso').toUpperCase()} #{p.id}</option>
              ))}
            </select>
          </Field>
          <Field label="Password" id="new-login-password" hint="Optional; blank uses the user's password.">
            <input id="new-login-password" type="password" autoComplete="new-password" className={inputClass} disabled={!!draft.authentication_provider_id} value={draft.password} onChange={(e) => setDraft({ ...draft, password: e.target.value })} />
          </Field>
          <Field label="SIS ID" id="new-login-sis">
            <input id="new-login-sis" className={inputClass} value={draft.sis_user_id} onChange={(e) => setDraft({ ...draft, sis_user_id: e.target.value })} />
          </Field>
          <Button type="submit" disabled={mutation.isPending}><Plus className="w-4 h-4" /> Add login</Button>
        </form>
      )}
    </Section>
  );
}

function ObserverSection({ user }) {
  const [term, setTerm] = useState('');
  const [results, setResults] = useState([]);
  const mutation = useObserverLinkMutation(user.id);

  useEffect(() => {
    const q = term.trim();
    if (q.length < 2) {
      setResults([]);
      return undefined;
    }
    const t = setTimeout(async () => {
      try {
        const res = await api.searchUsers(q, 1, 8);
        setResults((res?.data || []).filter((u) => u.id !== user.id));
      } catch {
        setResults([]);
      }
    }, 250);
    return () => clearTimeout(t);
  }, [term, user.id]);

  const row = (u, unlink) => (
    <li key={u.id} className="flex items-center justify-between py-1.5 border-t border-border-default">
      <Link to={`/admin/users/${u.id}`} className="text-brand-600 hover:underline">{u.name}</Link>
      <Button variant="secondary" onClick={unlink}>Remove</Button>
    </li>
  );

  return (
    <Section title="Observers" description="Parents and guardians follow a student into every current and future course.">
      <ErrorNote error={mutation.error} />
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <h3 className="text-sm font-medium text-text-secondary mb-1">{user.name} observes</h3>
          <ul>
            {user.observees.length === 0 && <li className="text-sm text-text-tertiary">No students.</li>}
            {user.observees.map((s) => row(s, () => mutation.mutate({ action: 'unlink', observerId: user.id, studentId: s.id })))}
          </ul>
        </div>
        <div>
          <h3 className="text-sm font-medium text-text-secondary mb-1">Observing {user.name}</h3>
          <ul>
            {user.observers.length === 0 && <li className="text-sm text-text-tertiary">No observers.</li>}
            {user.observers.map((o) => row(o, () => mutation.mutate({ action: 'unlink', observerId: o.id, studentId: user.id })))}
          </ul>
        </div>
      </div>
      <div className="mt-4">
        <Field label="Link a student to this observer" id="observee-search">
          <input id="observee-search" className={inputClass} placeholder="Search by name or email" value={term} onChange={(e) => setTerm(e.target.value)} />
        </Field>
        {results.length > 0 && (
          <ul className="mt-2 rounded-md border border-border-default">
            {results.map((s) => (
              <li key={s.id} className="flex items-center justify-between px-3 py-1.5 border-t first:border-t-0 border-border-default text-sm">
                <span>{s.name} <span className="text-text-tertiary">{s.email}</span></span>
                <Button variant="secondary" onClick={() => { mutation.mutate({ action: 'link', observerId: user.id, studentId: s.id }); setTerm(''); }}>
                  Observe
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Section>
  );
}

function EnrollmentsSection({ enrollments }) {
  return (
    <Section title="Enrollments" description="Every course this person is in, in any state.">
      {enrollments.length === 0 ? (
        <p className="text-sm text-text-tertiary">No enrollments.</p>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-text-secondary text-left">
            <tr><th className="py-1">Course</th><th>Section</th><th>Role</th><th>State</th><th>Last activity</th></tr>
          </thead>
          <tbody>
            {enrollments.map((e) => (
              <tr key={e.id} className="border-t border-border-default">
                <td className="py-2"><Link to={`/courses/${e.course_id}`} className="text-brand-600 hover:underline">{e.course_name}</Link> <span className="text-text-tertiary">{e.course_code}</span></td>
                <td>{e.section_name || '—'}</td>
                <td>{e.role && e.role !== e.type ? `${e.role} (${e.type.replace('Enrollment', '')})` : e.type.replace('Enrollment', '')}</td>
                <td><Badge tone={e.enrollment_state === 'active' ? 'good' : 'neutral'}>{e.enrollment_state}</Badge></td>
                <td>{formatDateTime(e.last_activity_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Section>
  );
}

function HistorySection({ userId }) {
  const [tab, setTab] = useState('logins');
  const query = useAdminUserHistory(userId);
  const data = query.data || { logins: [], activity: [], grade_changes: [] };
  const tabs = [['logins', 'Sign-ins'], ['activity', 'Activity'], ['grade_changes', 'Grade changes']];
  return (
    <Section title="History" description="Sign-in history, audited actions and grade changes for this person.">
      <div className="flex gap-2 mb-3" role="tablist">
        {tabs.map(([k, label]) => (
          <button
            key={k}
            type="button"
            role="tab"
            aria-selected={tab === k}
            onClick={() => setTab(k)}
            className={`px-3 py-1 rounded-full text-xs font-medium ${tab === k ? 'bg-brand-600 text-on-brand' : 'bg-surface-1 text-text-secondary'}`}
          >
            {label}
          </button>
        ))}
      </div>
      {query.isLoading && <p className="text-sm text-text-tertiary">Loading…</p>}
      <ErrorNote error={query.error} />
      {tab !== 'grade_changes' ? (
        <table className="w-full text-sm">
          <thead className="text-text-secondary text-left"><tr><th className="py-1">When</th><th>Event</th><th>Detail</th><th>IP</th></tr></thead>
          <tbody>
            {(data[tab] || []).map((ev) => (
              <tr key={ev.id} className="border-t border-border-default">
                <td className="py-1.5 whitespace-nowrap">{formatDateTime(ev.created_at)}</td>
                <td>{ev.event_type}</td>
                <td className="text-text-secondary">{ev.action}{ev.real_user_id ? ` (by user #${ev.real_user_id})` : ''}</td>
                <td className="text-text-tertiary">{ev.ip_address || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <table className="w-full text-sm">
          <thead className="text-text-secondary text-left"><tr><th className="py-1">When</th><th>Course</th><th>Assignment</th><th>Student</th><th>Grader</th><th>Change</th></tr></thead>
          <tbody>
            {(data.grade_changes || []).map((gc) => (
              <tr key={gc.id} className="border-t border-border-default">
                <td className="py-1.5 whitespace-nowrap">{formatDateTime(gc.created_at)}</td>
                <td>#{gc.course_id}</td>
                <td>#{gc.assignment_id}</td>
                <td>#{gc.student_id}</td>
                <td>#{gc.grader_id}</td>
                <td>{gc.old_grade || '—'} → {gc.new_grade || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Section>
  );
}

const AdminUserDetailPage = () => {
  const { userId } = useParams();
  const query = useAdminUser(userId);
  return (
    <Page query={query} title="User">
      {(user) => (
        <Layout>
          <div className="p-8 max-w-6xl">
            <Link to="/admin/people" className="inline-flex items-center gap-1 text-sm text-brand-600 hover:underline mb-4">
              <ArrowLeft className="w-4 h-4" /> People
            </Link>
            <div className="flex flex-wrap items-start justify-between gap-4 mb-6">
              <div>
                <h1 className="text-2xl font-bold text-text-primary">{user.name}</h1>
                <p className="text-sm text-text-secondary mt-1">
                  {user.email} · {user.role}
                  {user.suspended && <> · <Badge tone="bad">suspended</Badge></>}
                  {user.mfa_enrolled && <> · <Badge tone="brand">2FA</Badge></>}
                </p>
              </div>
              <StatusActions user={user} />
            </div>
            <Section title="Profile"><ProfileForm user={user} /></Section>
            <EnrollmentsSection enrollments={user.enrollments} />
            <LoginsSection user={user} />
            <ObserverSection user={user} />
            <HistorySection userId={user.id} />
          </div>
        </Layout>
      )}
    </Page>
  );
};

export default AdminUserDetailPage;
