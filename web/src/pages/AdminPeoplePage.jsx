// Reference migration #2 — "list + per-row mutation" shape.
//
// Before: ~35 lines of useState/useEffect + a manual debounce
// `useEffect`, plus an optimistic-but-not-really role-update flow.
//
// After: `useUsersList(search)` flips its query key when search changes
// — react-query keeps both the "everyone" and "filtered" result sets
// in cache so toggling the input doesn't re-fetch. `useUpdateUserRole`
// invalidates the list on success.
//
// Notable: we still keep a small `useEffect` for the 250ms search
// debounce — react-query has no opinion on input timing, so this
// stays exactly where it was.

import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Search, UserPlus } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { useUsersList, useUpdateUserRole } from '../services/apiQueries';
import { useBulkSuspend } from '../services/adminAccountsQueries';
import AddUserForm from '../components/admin/AddUserForm';

// Roles an admin can assign from this page. super_admin is never
// assignable here, but a super_admin row must still render as such (it
// used to fall through to the 'user' option, so any change demoted them).
const ROLES = ['user', 'observer', 'teacher', 'admin'];

const ROLE_LABELS = {
  user:        'User',
  observer:    'Observer',
  teacher:     'Teacher',
  admin:       'Admin',
  super_admin: 'Super admin',
};

const ROLE_BADGE = {
  super_admin: 'bg-brand-50 text-brand-700 border-brand-200',
  admin:    'bg-brand-50 text-brand-700 border-brand-200',
  teacher:  'bg-accent-success/10 text-accent-success border-accent-success/30',
  observer: 'bg-accent-warning/10 text-accent-warning border-accent-warning/30',
  user:     'bg-surface-1 text-text-secondary border-border-default',
};

const AdminPeoplePage = () => {
  const { user: currentUser } = useAuth();
  const [search, setSearch] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [statusMsg, setStatusMsg] = useState('');

  // 250ms debounce so we don't fire a query on every keystroke.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 250);
    return () => clearTimeout(t);
  }, [search]);

  const query = useUsersList(debouncedSearch, 1, 100);
  const updateRole = useUpdateUserRole({
    onSuccess: () => {
      setStatusMsg('Role updated');
      setTimeout(() => setStatusMsg(''), 1500);
    },
  });

  const handleRoleChange = (userId, role) => {
    updateRole.mutate({ userId, role });
  };

  // G114: add users, open a user's detail page, bulk suspend.
  const [showAdd, setShowAdd] = useState(false);
  const [selected, setSelected] = useState(() => new Set());
  const bulk = useBulkSuspend({
    onSuccess: (res) => {
      setSelected(new Set());
      setStatusMsg(`${res?.count ?? 0} user(s) updated`);
      setTimeout(() => setStatusMsg(''), 2000);
    },
  });
  const toggleSelected = (id) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <Page
      query={query}
      title="People"
      empty={(result) => (result?.data?.length ?? 0) === 0}
      emptyMessage="No users match."
    >
      {(result) => {
        const users = result?.data || [];
        return (
          <Layout>
            <div className="p-8">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h1 className="text-2xl font-bold text-text-primary">People</h1>
                  <p className="text-sm text-text-secondary mt-1">
                    {users.length} user{users.length === 1 ? '' : 's'}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  {statusMsg && (
                    <span className="text-xs text-accent-success" role="status">{statusMsg}</span>
                  )}
                  <button
                    type="button"
                    onClick={() => setShowAdd((v) => !v)}
                    className="inline-flex items-center gap-1.5 rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700"
                  >
                    <UserPlus className="w-4 h-4" /> Add user
                  </button>
                </div>
              </div>

              {showAdd && <AddUserForm onClose={() => setShowAdd(false)} />}

              {selected.size > 0 && (
                <div className="mb-4 flex items-center gap-2 rounded-md border border-border-default bg-surface-1 px-3 py-2 text-sm">
                  <span>{selected.size} selected</span>
                  <button
                    type="button"
                    disabled={bulk.isPending}
                    onClick={() => bulk.mutate({ userIds: [...selected], suspended: true })}
                    className="rounded-md border border-accent-danger/40 px-2 py-1 text-xs text-accent-danger"
                  >
                    Suspend
                  </button>
                  <button
                    type="button"
                    disabled={bulk.isPending}
                    onClick={() => bulk.mutate({ userIds: [...selected], suspended: false })}
                    className="rounded-md border border-border-strong px-2 py-1 text-xs"
                  >
                    Unsuspend
                  </button>
                </div>
              )}

              <div className="mb-4 relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
                <input
                  type="search"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="Search by name or email"
                  className="w-full max-w-md rounded-md border border-border-strong bg-surface-0 pl-9 pr-3 py-2 text-sm"
                />
              </div>

              {updateRole.isError && (
                <div
                  className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/5 p-3 text-sm text-accent-danger"
                  role="alert"
                >
                  {updateRole.error?.message || 'Could not update role.'}
                </div>
              )}

              <div className="overflow-hidden rounded-lg border border-border-default bg-surface-0">
                <table className="w-full text-sm">
                  <thead className="bg-surface-1 text-text-secondary">
                    <tr>
                      <th className="px-4 py-2 text-left font-medium"><span className="sr-only">Select</span></th>
                      <th className="px-4 py-2 text-left font-medium">Name</th>
                      <th className="px-4 py-2 text-left font-medium">Email</th>
                      <th className="px-4 py-2 text-left font-medium">Role</th>
                      <th className="px-4 py-2 text-left font-medium">Change role</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((u) => {
                      const role = u.role || 'user';
                      const isSelf = currentUser?.id === u.id;
                      const isSaving = updateRole.isPending && updateRole.variables?.userId === u.id;
                      // Only a super_admin may change a super_admin (the API
                      // enforces the same rule).
                      const isProtected = role === 'super_admin' && currentUser?.role !== 'super_admin';
                      const options = role === 'super_admin' ? ['super_admin', ...ROLES] : ROLES;
                      return (
                        <tr key={u.id} className="border-t border-border-default hover:bg-surface-1">
                          <td className="px-4 py-2">
                            <input
                              type="checkbox"
                              checked={selected.has(u.id)}
                              disabled={isSelf || isProtected}
                              onChange={() => toggleSelected(u.id)}
                              aria-label={`Select ${u.name || 'user'}`}
                            />
                          </td>
                          <td className="px-4 py-2 font-medium text-text-primary">
                            <Link to={`/admin/users/${u.id}`} className="text-brand-600 hover:underline">{u.name || '(unnamed)'}</Link>
                            {u.suspended && (
                              <span className="ml-2 rounded-full border border-accent-danger/30 bg-accent-danger/10 px-2 py-0.5 text-xs text-accent-danger">suspended</span>
                            )}
                          </td>
                          <td className="px-4 py-2 text-text-secondary">{u.email || '—'}</td>
                          <td className="px-4 py-2">
                            <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${ROLE_BADGE[role] || ROLE_BADGE.user}`}>
                              {ROLE_LABELS[role] || role}
                            </span>
                          </td>
                          <td className="px-4 py-2">
                            <select
                              value={role}
                              disabled={isSelf || isSaving || isProtected}
                              onChange={(e) => {
                                if (e.target.value !== 'super_admin') handleRoleChange(u.id, e.target.value);
                              }}
                              className="rounded-md border border-border-strong bg-surface-0 px-2 py-1 text-xs disabled:opacity-50"
                              aria-label={`Change role for ${u.name || 'user'}`}
                              title={
                                isSelf
                                  ? 'You cannot change your own role here'
                                  : isProtected
                                    ? 'Only a super admin can change a super admin'
                                    : ''
                              }
                            >
                              {options.map((r) => (
                                <option key={r} value={r} disabled={r === 'super_admin'}>{ROLE_LABELS[r]}</option>
                              ))}
                            </select>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          </Layout>
        );
      }}
    </Page>
  );
};

export default AdminPeoplePage;
