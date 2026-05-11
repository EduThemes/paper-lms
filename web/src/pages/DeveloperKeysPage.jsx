import React, { useState, useEffect, useCallback } from 'react';
import { KeyRound, Plus, Trash2, Copy, Check, AlertTriangle, X, ToggleLeft, ToggleRight, Eye, EyeOff } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';

const ACCOUNT_ID = 1;

const DeveloperKeysPage = () => {
  const [keys, setKeys] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newKeyData, setNewKeyData] = useState(null);
  const [copiedField, setCopiedField] = useState(null);
  const [deleteConfirm, setDeleteConfirm] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [toggling, setToggling] = useState(null);

  const fetchKeys = useCallback(async () => {
    try {
      const { data } = await api.getDeveloperKeys(ACCOUNT_ID, 1, 100);
      setKeys(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchKeys();
  }, [fetchKeys]);

  const handleCreate = async (e) => {
    e.preventDefault();
    setCreating(true);
    setError(null);
    const formData = new FormData(e.target);

    const redirectUrisRaw = formData.get('redirect_uris');
    const redirectUris = redirectUrisRaw
      ? redirectUrisRaw.split('\n').map((s) => s.trim()).filter(Boolean)
      : undefined;

    const scopesRaw = formData.get('scopes');
    const scopes = scopesRaw
      ? scopesRaw.split('\n').map((s) => s.trim()).filter(Boolean)
      : undefined;

    const developerKey = {
      name: formData.get('name'),
      email: formData.get('email') || undefined,
      redirect_uri: formData.get('redirect_uri') || undefined,
      redirect_uris: redirectUris,
      notes: formData.get('notes') || undefined,
      scopes: scopes,
    };

    // Remove undefined fields
    Object.keys(developerKey).forEach(
      (k) => developerKey[k] === undefined && delete developerKey[k]
    );

    try {
      const data = await api.createDeveloperKey(ACCOUNT_ID, developerKey);
      setNewKeyData(data);
      setShowForm(false);
      fetchKeys();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleCopy = async (text, field) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
    }
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const handleToggle = async (key) => {
    setToggling(key.id);
    setError(null);
    const newState = key.workflow_state === 'active' ? 'inactive' : 'active';
    try {
      await api.updateDeveloperKey(ACCOUNT_ID, key.id, { workflow_state: newState });
      fetchKeys();
    } catch (err) {
      setError(err.message);
    } finally {
      setToggling(null);
    }
  };

  const handleDelete = async (keyId) => {
    setDeleting(true);
    setError(null);
    try {
      await api.deleteDeveloperKey(ACCOUNT_ID, keyId);
      setDeleteConfirm(null);
      fetchKeys();
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  return (
    <Layout>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-text-primary">Developer Keys</h2>
          <p className="text-text-secondary mt-1">
            Manage OAuth2 client credentials for third-party application integrations.
          </p>
        </div>
        <button
          onClick={() => {
            setShowForm(!showForm);
            setNewKeyData(null);
          }}
          className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
        >
          <Plus className="w-4 h-4" />
          Add Developer Key
        </button>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* New Key Credentials Display */}
      {newKeyData && (
        <div className="bg-accent-success/10 border border-accent-success/30 rounded-lg p-6 mb-6">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-accent-warning mt-0.5 flex-shrink-0" />
            <div className="flex-1">
              <h3 className="font-semibold text-text-primary mb-1">Developer Key Created</h3>
              <p className="text-sm text-text-secondary mb-4">
                Save these credentials now. The client secret will not be shown again.
              </p>
              <div className="space-y-3">
                <div>
                  <label className="block text-xs font-medium text-text-tertiary mb-1">Client ID</label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 bg-surface-0 border border-accent-success/30 rounded px-3 py-2 text-sm font-mono text-text-primary break-all">
                      {newKeyData.api_key || newKeyData.client_id || newKeyData.id}
                    </code>
                    <button
                      onClick={() => handleCopy(String(newKeyData.api_key || newKeyData.client_id || newKeyData.id), 'client_id')}
                      className="flex items-center gap-1 bg-surface-0 border border-border-strong px-3 py-2 rounded-md hover:bg-surface-1 text-sm flex-shrink-0"
                    >
                      {copiedField === 'client_id' ? (
                        <Check className="w-4 h-4 text-accent-success" />
                      ) : (
                        <Copy className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-tertiary mb-1">Client Secret</label>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 bg-surface-0 border border-accent-success/30 rounded px-3 py-2 text-sm font-mono text-text-primary break-all">
                      {newKeyData.api_secret || newKeyData.client_secret || 'N/A'}
                    </code>
                    <button
                      onClick={() => handleCopy(String(newKeyData.api_secret || newKeyData.client_secret || ''), 'client_secret')}
                      className="flex items-center gap-1 bg-surface-0 border border-border-strong px-3 py-2 rounded-md hover:bg-surface-1 text-sm flex-shrink-0"
                    >
                      {copiedField === 'client_secret' ? (
                        <Check className="w-4 h-4 text-accent-success" />
                      ) : (
                        <Copy className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <button onClick={() => setNewKeyData(null)} className="text-text-disabled hover:text-text-secondary">
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>
      )}

      {/* Create Form */}
      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">Add Developer Key</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Name <span className="text-accent-danger">*</span>
                </label>
                <input
                  name="name"
                  required
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="My Application"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Owner Email
                </label>
                <input
                  name="email"
                  type="email"
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="developer@example.com"
                />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Redirect URI
              </label>
              <input
                name="redirect_uri"
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://example.com/oauth2/callback"
              />
              <p className="text-xs text-text-tertiary mt-1">Primary redirect URI for OAuth2 flow.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Additional Redirect URIs
              </label>
              <textarea
                name="redirect_uris"
                rows={3}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder={"https://example.com/callback1\nhttps://example.com/callback2"}
              />
              <p className="text-xs text-text-tertiary mt-1">One URI per line.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Notes</label>
              <textarea
                name="notes"
                rows={2}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="Internal notes about this developer key..."
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Scopes</label>
              <textarea
                name="scopes"
                rows={3}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder={"url:GET|/api/v1/courses\nurl:POST|/api/v1/courses/:id/assignments"}
              />
              <p className="text-xs text-text-tertiary mt-1">
                One scope per line. Leave blank to allow all scopes.
              </p>
            </div>
            <div className="flex gap-3">
              <button
                type="submit"
                disabled={creating}
                className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                {creating ? 'Creating...' : 'Create Key'}
              </button>
              <button
                type="button"
                onClick={() => setShowForm(false)}
                className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Delete Confirmation */}
      {deleteConfirm !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-surface-0 rounded-lg shadow-xl p-6 max-w-sm mx-4">
            <h3 className="font-semibold text-text-primary mb-2">Delete Developer Key</h3>
            <p className="text-sm text-text-secondary mb-4">
              Are you sure you want to delete this developer key? All applications using these credentials will stop working.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setDeleteConfirm(null)}
                className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
                disabled={deleting}
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteConfirm)}
                disabled={deleting}
                className="bg-accent-danger text-white px-4 py-2 rounded-md hover:bg-accent-danger/90 text-sm disabled:opacity-50"
              >
                {deleting ? 'Deleting...' : 'Delete Key'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Developer Keys List */}
      {loading ? (
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading developer keys...
</div>
      ) : keys.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <KeyRound className="w-12 h-12 text-text-disabled mx-auto mb-4" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No Developer Keys</h3>
          <p className="text-text-tertiary text-sm">
            No developer keys have been created yet. Create one to enable OAuth2 integrations.
          </p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-border-default">
            <thead className="bg-surface-1">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Client ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Redirect URI
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  State
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Created
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-surface-0 divide-y divide-border-default">
              {keys.map((key) => (
                <tr key={key.id} className="hover:bg-surface-1">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      <KeyRound className="w-4 h-4 text-text-disabled flex-shrink-0" />
                      <div>
                        <span className="text-sm font-medium text-text-primary">{key.name || 'Unnamed'}</span>
                        {key.email && (
                          <p className="text-xs text-text-tertiary">{key.email}</p>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <code className="text-sm text-text-secondary bg-surface-2 px-2 py-0.5 rounded font-mono">
                      {key.api_key || key.client_id || key.id}
                    </code>
                  </td>
                  <td className="px-6 py-4">
                    <span className="text-sm text-text-secondary break-all max-w-xs block truncate" title={key.redirect_uri}>
                      {key.redirect_uri || '-'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`text-xs px-2 py-1 rounded-full ${
                        key.workflow_state === 'active'
                          ? 'bg-accent-success/20 text-accent-success'
                          : key.workflow_state === 'inactive'
                          ? 'bg-surface-2 text-text-secondary'
                          : 'bg-accent-warning/20 text-accent-warning'
                      }`}
                    >
                      {key.workflow_state || 'active'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    {key.is_lti_key ? (
                      <span className="text-xs px-2 py-1 rounded-full bg-purple-100 text-purple-800">
                        LTI
                      </span>
                    ) : (
                      <span className="text-xs px-2 py-1 rounded-full bg-brand-100 text-brand-700">
                        OAuth2
                      </span>
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                    {formatDate(key.created_at)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => handleToggle(key)}
                        disabled={toggling === key.id}
                        className="text-text-tertiary hover:text-text-secondary p-1 disabled:opacity-50"
                        title={key.workflow_state === 'active' ? 'Deactivate' : 'Activate'}
                      >
                        {key.workflow_state === 'active' ? (
                          <ToggleRight className="w-5 h-5 text-accent-success" />
                        ) : (
                          <ToggleLeft className="w-5 h-5 text-text-disabled" />
                        )}
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(key.id)}
                        className="text-accent-danger hover:text-accent-danger p-1"
                        title="Delete key"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Layout>
  );
};

export default DeveloperKeysPage;
