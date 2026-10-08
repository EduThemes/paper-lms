import React, { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Key, Plus, Trash2, Copy, Check, AlertTriangle, X, RefreshCw, Search, UserCog } from 'lucide-react';
import { api } from '../services/api';
import {
  deleteUserToken,
  expiryToISO,
  listAccountUsers,
  listUserTokens,
  tokenStatus,
  updateAccessToken,
} from '../services/api_api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

// Personal access tokens (G143): create with an optional expiry, see when
// each token expires, regenerate a token's secret, delete. Admins can also
// look up a user of their account and revoke that user's tokens.

const tokensKey = (userId) => ['accessTokens', String(userId)];

const formatDate = (dateStr) => {
  if (!dateStr) return '-';
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const tomorrow = () => {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  return d.toISOString().slice(0, 10);
};

const STATUS_BADGE = {
  expired: { cls: 'bg-accent-danger/10 text-accent-danger', label: 'Expired' },
  expiring: { cls: 'bg-accent-warning/10 text-accent-warning', label: 'Expires soon' },
  integration: { cls: 'bg-surface-2 text-text-secondary', label: 'App' },
  active: null,
};

function ExpiryCell({ token }) {
  const status = tokenStatus(token);
  const badge = STATUS_BADGE[status];
  return (
    <div className="flex items-center gap-2">
      <span>{token.kind === 'oauth' ? '-' : token.expires_at ? formatDate(token.expires_at) : 'Never'}</span>
      {badge && <span className={`text-xs px-2 py-0.5 rounded-full ${badge.cls}`}>{badge.label}</span>}
    </div>
  );
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Fallback for non-HTTPS contexts
    const textArea = document.createElement('textarea');
    textArea.value = text;
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand('copy');
    document.body.removeChild(textArea);
  }
}

function NewTokenBanner({ token, onClose }) {
  const [copied, setCopied] = useState(false);
  const value = token?.visible_token || token?.full_token || token?.token;
  if (!value) return null;
  return (
    <div className="bg-accent-success/10 border border-green-300 rounded-lg p-6 mb-6">
      <div className="flex items-start gap-3">
        <AlertTriangle className="w-5 h-5 text-accent-warning mt-0.5 flex-shrink-0" />
        <div className="flex-1">
          <h3 className="font-semibold text-text-primary mb-1">Token Generated Successfully</h3>
          <p className="text-sm text-text-secondary mb-3">
            Copy your personal access token now. You will not be able to see it again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-surface-0 border border-green-300 rounded px-3 py-2 text-sm font-mono text-text-primary break-all">
              {value}
            </code>
            <button
              onClick={async () => {
                await copyText(value);
                setCopied(true);
                setTimeout(() => setCopied(false), 2000);
              }}
              className="flex items-center gap-1 bg-surface-0 border border-border-strong px-3 py-2 rounded-md hover:bg-surface-1 text-sm flex-shrink-0"
            >
              {copied ? (
                <>
                  <Check className="w-4 h-4 text-accent-success" />
                  <span className="text-accent-success">Copied</span>
                </>
              ) : (
                <>
                  <Copy className="w-4 h-4" />
                  <span>Copy</span>
                </>
              )}
            </button>
          </div>
        </div>
        <button onClick={onClose} className="text-text-disabled hover:text-text-secondary" aria-label="Dismiss">
          <X className="w-5 h-5" />
        </button>
      </div>
    </div>
  );
}

function TokenTable({ tokens, onDelete, onRegenerate, emptyText }) {
  if (tokens.length === 0) {
    return (
      <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
        <Key className="w-12 h-12 text-gray-300 mx-auto mb-4" />
        <h3 className="text-lg font-medium text-text-primary mb-1">No Access Tokens</h3>
        <p className="text-text-tertiary text-sm">{emptyText}</p>
      </div>
    );
  }
  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
      <table className="min-w-full divide-y divide-border-default">
        <thead className="bg-surface-1">
          <tr>
            {['Purpose', 'Token', 'Scopes', 'Expires', 'Last Used'].map((h) => (
              <th key={h} className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">{h}</th>
            ))}
            <th className="px-6 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">Actions</th>
          </tr>
        </thead>
        <tbody className="bg-surface-0 divide-y divide-border-default">
          {tokens.map((token) => (
            <tr key={token.id} className="hover:bg-surface-1">
              <td className="px-6 py-4 whitespace-nowrap">
                <div className="flex items-center gap-2">
                  <Key className="w-4 h-4 text-text-disabled flex-shrink-0" />
                  <span className="text-sm font-medium text-text-primary">
                    {token.app_name || token.purpose || 'Unnamed token'}
                  </span>
                </div>
              </td>
              <td className="px-6 py-4 whitespace-nowrap">
                <code className="text-sm text-text-tertiary bg-surface-2 px-2 py-0.5 rounded">...{token.token_hint || '****'}</code>
              </td>
              <td className="px-6 py-4">
                <div className="flex flex-wrap gap-1">
                  {token.scopes && token.scopes.length > 0 ? (
                    token.scopes.map((scope) => (
                      <span key={scope} className="inline-block bg-brand-100 text-brand-700 text-xs px-2 py-0.5 rounded-full">{scope}</span>
                    ))
                  ) : (
                    <span className="text-xs text-text-disabled">Full access</span>
                  )}
                </div>
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary"><ExpiryCell token={token} /></td>
              <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">{formatDate(token.last_used_at)}</td>
              <td className="px-6 py-4 whitespace-nowrap text-right">
                {onRegenerate && token.can_manually_regenerate && (
                  <button
                    onClick={() => onRegenerate(token)}
                    className="text-text-secondary hover:text-brand-700 p-1"
                    title="Regenerate token"
                    aria-label={`Regenerate ${token.purpose || 'token'}`}
                  >
                    <RefreshCw className="w-4 h-4" />
                  </button>
                )}
                <button
                  onClick={() => onDelete(token)}
                  className="text-accent-danger hover:text-accent-danger p-1"
                  title="Delete token"
                  aria-label={`Delete ${token.app_name || token.purpose || 'token'}`}
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ConfirmDialog({ title, children, confirmLabel, busy, confirmDisabled = false, onCancel, onConfirm, danger = true }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div role="dialog" aria-modal="true" aria-label={title} className="bg-surface-0 rounded-lg shadow-xl p-6 max-w-sm mx-4">
        <h3 className="font-semibold text-text-primary mb-2">{title}</h3>
        <div className="text-sm text-text-secondary mb-4">{children}</div>
        <div className="flex gap-3 justify-end">
          <button onClick={onCancel} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary" disabled={busy}>
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={busy || confirmDisabled}
            className={`${danger ? 'bg-accent-danger hover:bg-accent-danger/90' : 'bg-brand-600 hover:bg-brand-700'} text-white px-4 py-2 rounded-md text-sm disabled:opacity-50`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}

// AdminTokenManager lets an account admin find a user of their account
// (GET /accounts/self/users) and revoke that user's tokens.
function AdminTokenManager() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [term, setTerm] = useState('');
  const [selected, setSelected] = useState(null);
  const [confirm, setConfirm] = useState(null);

  useEffect(() => {
    const t = setTimeout(() => setTerm(search.trim()), 250);
    return () => clearTimeout(t);
  }, [search]);

  const users = useQuery({
    queryKey: ['accountUsers', 'self', term],
    queryFn: () => listAccountUsers('self', { search_term: term, include: ['email'] }),
    enabled: term.length >= 2,
  });
  const tokens = useQuery({
    queryKey: tokensKey(selected?.id ?? 'none'),
    queryFn: () => listUserTokens(selected.id),
    enabled: !!selected,
  });
  const revoke = useMutation({
    mutationFn: (token) => deleteUserToken(selected.id, token.id),
    onSuccess: () => {
      setConfirm(null);
      queryClient.invalidateQueries({ queryKey: tokensKey(selected.id) });
    },
  });

  return (
    <section className="mt-10" aria-labelledby="admin-tokens-heading">
      <div className="flex items-center gap-2 mb-2">
        <UserCog className="w-5 h-5 text-brand-600" aria-hidden="true" />
        <h3 id="admin-tokens-heading" className="text-lg font-semibold text-text-primary">Manage a user&apos;s tokens</h3>
      </div>
      <p className="text-sm text-text-secondary mb-4">
        As an administrator you can review and revoke the tokens of people in your account. Only the owner can create or
        regenerate a token.
      </p>
      <div className="relative max-w-md mb-3">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" />
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search people by name, login or SIS ID"
          aria-label="Search people"
          className="w-full rounded-md border border-border-strong bg-surface-0 pl-9 pr-3 py-2 text-sm"
        />
      </div>
      {term.length >= 2 && users.data && (
        <ul className="mb-4 max-w-md divide-y divide-border-default rounded-md border border-border-default bg-surface-0">
          {users.data.length === 0 && <li className="px-3 py-2 text-sm text-text-tertiary">No people match.</li>}
          {users.data.map((u) => (
            <li key={u.id}>
              <button
                type="button"
                onClick={() => setSelected(u)}
                className={`w-full px-3 py-2 text-left text-sm hover:bg-surface-1 ${selected?.id === u.id ? 'bg-brand-50' : ''}`}
              >
                <span className="font-medium text-text-primary">{u.name}</span>
                <span className="ml-2 text-text-tertiary">{u.login_id}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {selected && (
        <div>
          <h4 className="text-sm font-medium text-text-primary mb-2">Tokens of {selected.name}</h4>
          {tokens.isLoading ? (
            <p className="text-sm text-text-tertiary">Loading tokens…</p>
          ) : tokens.isError ? (
            <p role="alert" className="text-sm text-accent-danger">{tokens.error?.message || 'Could not load tokens.'}</p>
          ) : (
            <TokenTable tokens={tokens.data || []} onDelete={setConfirm} emptyText="This person has no access tokens." />
          )}
        </div>
      )}
      {confirm && (
        <ConfirmDialog
          title="Revoke Token"
          confirmLabel={revoke.isPending ? 'Revoking...' : 'Revoke Token'}
          busy={revoke.isPending}
          onCancel={() => setConfirm(null)}
          onConfirm={() => revoke.mutate(confirm)}
        >
          Revoke {selected?.name}&apos;s token &ldquo;{confirm.app_name || confirm.purpose}&rdquo;? Anything using it loses
          access immediately.
        </ConfirmDialog>
      )}
    </section>
  );
}

const AccessTokensPage = () => {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const userId = user?.id;
  const [showForm, setShowForm] = useState(false);
  const [newToken, setNewToken] = useState(null);
  const [deleteConfirm, setDeleteConfirm] = useState(null);
  const [regenerate, setRegenerate] = useState(null);
  const [regenExpiry, setRegenExpiry] = useState('');
  const [error, setError] = useState(null);
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';

  const tokensQuery = useQuery({
    queryKey: tokensKey(userId),
    queryFn: async () => {
      const { data } = await api.getAccessTokens(userId, 1, 100);
      return Array.isArray(data) ? data : [];
    },
    enabled: !!userId,
  });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: tokensKey(userId) });

  const create = useMutation({
    mutationFn: (token) => api.createAccessToken(userId, token),
    onSuccess: (data) => {
      setNewToken(data);
      setShowForm(false);
      invalidate();
    },
    onError: (err) => setError(err.message),
  });
  const remove = useMutation({
    mutationFn: (tokenId) => api.deleteAccessToken(userId, tokenId),
    onSuccess: () => {
      setDeleteConfirm(null);
      invalidate();
    },
    onError: (err) => setError(err.message),
  });
  const regen = useMutation({
    mutationFn: ({ token, expiresAt }) =>
      updateAccessToken(userId, token.id, { regenerate: true, ...(expiresAt ? { expires_at: expiresAt } : {}) }),
    onSuccess: (data) => {
      setNewToken(data);
      setRegenerate(null);
      setRegenExpiry('');
      invalidate();
    },
    onError: (err) => setError(err.message),
  });

  const handleCreate = (e) => {
    e.preventDefault();
    setError(null);
    const formData = new FormData(e.target);
    const purpose = formData.get('purpose');
    const scopesRaw = formData.get('scopes');
    const scopes = scopesRaw ? scopesRaw.split(',').map((s) => s.trim()).filter(Boolean) : undefined;
    const expiresAt = expiryToISO(formData.get('expires_at'));
    create.mutate({
      purpose,
      ...(scopes && scopes.length > 0 ? { scopes } : {}),
      ...(expiresAt ? { expires_at: expiresAt } : {}),
    });
  };

  const regenNeedsExpiry = regenerate && tokenStatus(regenerate) === 'expired';
  const tokens = tokensQuery.data || [];

  return (
    <Layout>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-text-primary">Personal Access Tokens</h2>
          <p className="text-text-secondary mt-1">
            Manage tokens for API access. Tokens allow third-party applications to access the API on your behalf.
          </p>
        </div>
        <button
          onClick={() => {
            setShowForm(!showForm);
            setNewToken(null);
          }}
          className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
        >
          <Plus className="w-4 h-4" />
          Generate New Token
        </button>
      </div>

      {(error || tokensQuery.isError) && (
        <div role="alert" className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error || tokensQuery.error?.message}
          <button onClick={() => setError(null)} className="ml-auto" aria-label="Dismiss error">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {newToken && <NewTokenBanner token={newToken} onClose={() => setNewToken(null)} />}

      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">Generate New Token</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label htmlFor="token-purpose" className="block text-sm font-medium text-text-secondary mb-1">
                Purpose <span className="text-accent-danger">*</span>
              </label>
              <input
                id="token-purpose"
                name="purpose"
                required
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="e.g., CI/CD pipeline, Mobile app, Script"
              />
              <p className="text-xs text-text-tertiary mt-1">A description of what this token will be used for.</p>
            </div>
            <div>
              <label htmlFor="token-expires" className="block text-sm font-medium text-text-secondary mb-1">Expires</label>
              <input
                id="token-expires"
                name="expires_at"
                type="date"
                min={tomorrow()}
                className="rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
              />
              <p className="text-xs text-text-tertiary mt-1">Leave blank for a token that never expires.</p>
            </div>
            <div>
              <label htmlFor="token-scopes" className="block text-sm font-medium text-text-secondary mb-1">Scopes</label>
              <textarea
                id="token-scopes"
                name="scopes"
                rows={2}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="e.g., url:GET|/api/v1/courses, url:GET|/api/v1/courses/:course_id/assignments"
              />
              <p className="text-xs text-text-tertiary mt-1">
                Comma-separated list of endpoint scopes (url:METHOD|/api/v1/path). The token can only call
                the endpoints listed. Leave blank for full access.
              </p>
            </div>
            <div className="flex gap-3">
              <button
                type="submit"
                disabled={create.isPending}
                className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                {create.isPending ? 'Generating...' : 'Generate Token'}
              </button>
              <button type="button" onClick={() => setShowForm(false)} className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm">
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {deleteConfirm && (
        <ConfirmDialog
          title="Delete Token"
          confirmLabel={remove.isPending ? 'Deleting...' : 'Delete Token'}
          busy={remove.isPending}
          onCancel={() => setDeleteConfirm(null)}
          onConfirm={() => remove.mutate(deleteConfirm.id)}
        >
          Are you sure you want to delete this token? Any applications using it will lose access immediately.
        </ConfirmDialog>
      )}

      {regenerate && (
        <ConfirmDialog
          title="Regenerate Token"
          danger={false}
          confirmLabel={regen.isPending ? 'Regenerating...' : 'Regenerate'}
          busy={regen.isPending}
          confirmDisabled={regenNeedsExpiry && !regenExpiry}
          onCancel={() => {
            setRegenerate(null);
            setRegenExpiry('');
          }}
          onConfirm={() => regen.mutate({ token: regenerate, expiresAt: expiryToISO(regenExpiry) })}
        >
          <p className="mb-3">
            The current secret of &ldquo;{regenerate.purpose}&rdquo; stops working at once and a new one is shown to you
            a single time.
          </p>
          <label htmlFor="regen-expires" className="block text-xs font-medium text-text-secondary mb-1">
            {regenNeedsExpiry ? 'New expiry date (required for an expired token)' : 'New expiry date (optional)'}
          </label>
          <input
            id="regen-expires"
            type="date"
            min={tomorrow()}
            value={regenExpiry}
            onChange={(e) => setRegenExpiry(e.target.value)}
            className="rounded-md border border-border-strong px-3 py-1.5 text-sm"
          />
        </ConfirmDialog>
      )}

      {tokensQuery.isLoading ? (
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">Loading tokens...</div>
      ) : (
        <TokenTable
          tokens={tokens}
          onDelete={setDeleteConfirm}
          onRegenerate={setRegenerate}
          emptyText="You have not generated any personal access tokens yet."
        />
      )}

      {isAdmin && <AdminTokenManager />}
    </Layout>
  );
};

export default AccessTokensPage;
