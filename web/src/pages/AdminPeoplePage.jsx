import React, { useState, useEffect, useCallback } from 'react';
import { Search, Users } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const ROLES = ['user', 'observer', 'teacher', 'admin'];

const ROLE_LABELS = {
  user:     'User',
  observer: 'Observer',
  teacher:  'Teacher',
  admin:    'Admin',
};

const ROLE_BADGE = {
  admin:    'bg-brand-50 text-brand-700 border-brand-200',
  teacher:  'bg-accent-success/10 text-accent-success border-accent-success/30',
  observer: 'bg-accent-warning/10 text-accent-warning border-accent-warning/30',
  user:     'bg-surface-1 text-text-secondary border-border-default',
};

const AdminPeoplePage = () => {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [search, setSearch] = useState('');
  const [savingId, setSavingId] = useState(null);
  const [statusMsg, setStatusMsg] = useState('');

  const fetchUsers = useCallback(async (term = '') => {
    setLoading(true);
    setError(null);
    try {
      const result = term
        ? await api.searchUsers(term, 1, 100)
        : await api.listUsers(1, 100);
      setUsers(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchUsers(); }, [fetchUsers]);

  // debounce search
  useEffect(() => {
    const t = setTimeout(() => { fetchUsers(search.trim()); }, 250);
    return () => clearTimeout(t);
  }, [search, fetchUsers]);

  const handleRoleChange = async (userId, role) => {
    setSavingId(userId);
    setStatusMsg('');
    try {
      await api.updateUserRole(userId, role);
      setUsers((prev) => prev.map((u) => (u.id === userId ? { ...u, role } : u)));
      setStatusMsg('Role updated');
      setTimeout(() => setStatusMsg(''), 1500);
    } catch (err) {
      setError(err.message);
    } finally {
      setSavingId(null);
    }
  };

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-text-primary">People</h1>
            <p className="text-sm text-text-secondary mt-1">
              {loading ? '—' : `${users.length} user${users.length === 1 ? '' : 's'}`}
            </p>
          </div>
          {statusMsg && (
            <span className="text-xs text-accent-success" role="status">{statusMsg}</span>
          )}
        </div>

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

        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        ) : error ? (
          <div className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-4 text-center">
            <p className="text-sm text-accent-danger mb-2">{error}</p>
            <button onClick={() => fetchUsers(search.trim())} className="text-sm font-medium text-brand-600 hover:text-brand-800">Try Again</button>
          </div>
        ) : users.length === 0 ? (
          <div className="rounded-lg border border-border-default bg-surface-0 p-10 text-center">
            <Users className="mx-auto w-8 h-8 text-text-tertiary mb-2" />
            <p className="text-sm text-text-secondary">No users match.</p>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border-default bg-surface-0">
            <table className="w-full text-sm">
              <thead className="bg-surface-1 text-text-secondary">
                <tr>
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
                  return (
                    <tr key={u.id} className="border-t border-border-default hover:bg-surface-1">
                      <td className="px-4 py-2 font-medium text-text-primary">{u.name || '(unnamed)'}</td>
                      <td className="px-4 py-2 text-text-secondary">{u.email || '—'}</td>
                      <td className="px-4 py-2">
                        <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${ROLE_BADGE[role] || ROLE_BADGE.user}`}>
                          {ROLE_LABELS[role] || role}
                        </span>
                      </td>
                      <td className="px-4 py-2">
                        <select
                          value={role}
                          disabled={isSelf || savingId === u.id}
                          onChange={(e) => handleRoleChange(u.id, e.target.value)}
                          className="rounded-md border border-border-strong bg-surface-0 px-2 py-1 text-xs disabled:opacity-50"
                          title={isSelf ? 'You cannot change your own role here' : ''}
                        >
                          {ROLES.map((r) => (
                            <option key={r} value={r}>{ROLE_LABELS[r]}</option>
                          ))}
                        </select>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default AdminPeoplePage;
