import React, { useState, useEffect, useCallback, useRef } from 'react';
import { RefreshCw, Plus, Trash2, Edit3, AlertTriangle, X, Check, Clock, Loader2, ChevronDown, ChevronRight, Database } from 'lucide-react';
import Layout from '../components/Layout';
import { getCSRFToken } from '../services/api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';
const getHeaders = () => ({
  'Content-Type': 'application/json',
  'X-CSRF-Token': getCSRFToken(),
});

const ACCOUNT_ID = 1;

const EMPTY_FORM = {
  name: '',
  base_url: '',
  client_id: '',
  client_secret: '',
  token_url: '',
  scope: 'https://purl.imsglobal.org/spec/or/v1p1/scope/roster-core.readonly',
  auto_sync: false,
  auto_sync_interval: 24,
};

const STATUS_BADGES = {
  idle: { bg: 'bg-surface-2', text: 'text-text-secondary', label: 'Idle' },
  syncing: { bg: 'bg-brand-100', text: 'text-brand-700', label: 'Syncing' },
  completed: { bg: 'bg-accent-success/20', text: 'text-accent-success', label: 'Completed' },
  error: { bg: 'bg-accent-danger/20', text: 'text-accent-danger', label: 'Error' },
};

const SYNC_LOG_STATUS = {
  running: { bg: 'bg-brand-100', text: 'text-brand-700', label: 'Running' },
  completed: { bg: 'bg-accent-success/20', text: 'text-accent-success', label: 'Completed' },
  failed: { bg: 'bg-accent-danger/20', text: 'text-accent-danger', label: 'Failed' },
};

const OneRosterPage = () => {
  const [connections, setConnections] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ ...EMPTY_FORM });
  const [submitting, setSubmitting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [testingId, setTestingId] = useState(null);
  const [syncingId, setSyncingId] = useState(null);
  const [selectedConnection, setSelectedConnection] = useState(null);
  const [syncLogs, setSyncLogs] = useState([]);
  const [syncLogsLoading, setSyncLogsLoading] = useState(false);
  const [expandedErrors, setExpandedErrors] = useState({});
  const pollRef = useRef(null);

  const fetchConnections = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections?per_page=100`, {
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to fetch OneRoster connections');
      const data = await res.json();
      setConnections(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchSyncLogs = useCallback(async (connectionId) => {
    setSyncLogsLoading(true);
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${connectionId}/sync_logs?per_page=20`, {
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to fetch sync logs');
      const data = await res.json();
      setSyncLogs(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setSyncLogsLoading(false);
    }
  }, []);

  const pollSyncStatus = useCallback(async (connectionId) => {
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${connectionId}/sync_logs?per_page=1`, {
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) return;
      const data = await res.json();
      if (Array.isArray(data) && data.length > 0 && data[0].status !== 'running') {
        // Sync finished, stop polling
        if (pollRef.current) {
          clearInterval(pollRef.current);
          pollRef.current = null;
        }
        setSyncingId(null);
        fetchConnections();
        if (selectedConnection && selectedConnection.id === connectionId) {
          fetchSyncLogs(connectionId);
        }
        setSuccess('Sync completed.');
      }
    } catch {
      // Ignore polling errors
    }
  }, [fetchConnections, fetchSyncLogs, selectedConnection]);

  useEffect(() => {
    fetchConnections();
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
      }
    };
  }, [fetchConnections]);

  const resetForm = () => {
    setFormData({ ...EMPTY_FORM });
    setEditingId(null);
    setShowForm(false);
  };

  const handleEdit = (conn) => {
    setFormData({
      name: conn.name || '',
      base_url: conn.base_url || '',
      client_id: conn.client_id || '',
      client_secret: '', // Don't prefill secret
      token_url: conn.token_url || '',
      scope: conn.scope || '',
      auto_sync: conn.auto_sync || false,
      auto_sync_interval: conn.auto_sync_interval || 24,
    });
    setEditingId(conn.id);
    setShowForm(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(null);

    const body = { ...formData };
    body.auto_sync_interval = parseInt(body.auto_sync_interval, 10) || 24;

    try {
      let res;
      if (editingId) {
        // Don't send empty client_secret on update (keep existing)
        const updateBody = { ...body };
        if (!updateBody.client_secret) {
          delete updateBody.client_secret;
        }
        res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${editingId}`, {
          method: 'PUT',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify(updateBody),
        });
      } else {
        res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections`, {
          method: 'POST',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify(body),
        });
      }

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.errors?.[0]?.message || 'Failed to save connection');
      }

      setSuccess(editingId ? 'Connection updated successfully.' : 'Connection created successfully.');
      resetForm();
      fetchConnections();
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id) => {
    setDeleting(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${id}`, {
        method: 'DELETE',
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to delete connection');
      setDeleteConfirm(null);
      setSuccess('Connection deleted successfully.');
      if (selectedConnection && selectedConnection.id === id) {
        setSelectedConnection(null);
        setSyncLogs([]);
      }
      fetchConnections();
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
  };

  const handleTestConnection = async (id) => {
    setTestingId(id);
    setError(null);
    setSuccess(null);
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${id}/test`, {
        method: 'POST',
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to test connection');
      const data = await res.json();
      if (data.success) {
        setSuccess(data.message);
      } else {
        setError(data.message);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setTestingId(null);
    }
  };

  const handleSync = async (id, type) => {
    setSyncingId(id);
    setError(null);
    setSuccess(null);
    try {
      const endpoint = type === 'incremental' ? 'sync_incremental' : 'sync';
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/oneroster_connections/${id}/${endpoint}`, {
        method: 'POST',
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.errors?.[0]?.message || 'Failed to start sync');
      }
      setSuccess(`${type === 'incremental' ? 'Incremental' : 'Full'} sync started.`);
      fetchConnections();

      // Start polling for sync status
      if (pollRef.current) clearInterval(pollRef.current);
      pollRef.current = setInterval(() => pollSyncStatus(id), 3000);
    } catch (err) {
      setError(err.message);
      setSyncingId(null);
    }
  };

  const handleSelectConnection = (conn) => {
    if (selectedConnection && selectedConnection.id === conn.id) {
      setSelectedConnection(null);
      setSyncLogs([]);
    } else {
      setSelectedConnection(conn);
      fetchSyncLogs(conn.id);
    }
  };

  const toggleErrorDetails = (logId) => {
    setExpandedErrors((prev) => ({
      ...prev,
      [logId]: !prev[logId],
    }));
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatDuration = (start, end) => {
    if (!start || !end) return '-';
    const ms = new Date(end) - new Date(start);
    if (ms < 1000) return `${ms}ms`;
    const secs = Math.floor(ms / 1000);
    if (secs < 60) return `${secs}s`;
    const mins = Math.floor(secs / 60);
    const remainSecs = secs % 60;
    return `${mins}m ${remainSecs}s`;
  };

  const parseErrorDetails = (details) => {
    if (!details) return [];
    try {
      return JSON.parse(details);
    } catch {
      return [details];
    }
  };

  const getStatusBadge = (status, map) => {
    const badge = map[status] || { bg: 'bg-surface-2', text: 'text-text-secondary', label: status };
    return (
      <span className={`text-xs px-2 py-1 rounded-full font-medium ${badge.bg} ${badge.text}`}>
        {badge.label}
      </span>
    );
  };

  return (
    <Layout>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-text-primary">OneRoster Connections</h2>
          <p className="text-text-secondary mt-1">
            Sync roster data from SIS systems via OneRoster 1.1 REST API (Clever, ClassLink, PowerSchool, Infinite Campus).
          </p>
        </div>
        <button
          onClick={() => {
            if (showForm && !editingId) {
              resetForm();
            } else {
              setFormData({ ...EMPTY_FORM });
              setEditingId(null);
              setShowForm(true);
            }
          }}
          className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
        >
          <Plus className="w-4 h-4" />
          Add Connection
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

      {success && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <Check className="w-4 h-4 flex-shrink-0" />
          {success}
          <button onClick={() => setSuccess(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Create / Edit Form */}
      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">
            {editingId ? 'Edit OneRoster Connection' : 'Add OneRoster Connection'}
          </h3>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Connection Name <span className="text-accent-danger">*</span>
              </label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="District SIS Connection"
                required
              />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  OneRoster Base URL <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="url"
                  value={formData.base_url}
                  onChange={(e) => setFormData({ ...formData, base_url: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="https://sis.district.edu/api"
                  required
                />
                <p className="text-xs text-text-tertiary mt-1">The base URL of the OneRoster 1.1 API (without /ims/oneroster/v1p1).</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Token URL (OAuth2) <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="url"
                  value={formData.token_url}
                  onChange={(e) => setFormData({ ...formData, token_url: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="https://sis.district.edu/oauth/token"
                  required
                />
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Client ID <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="text"
                  value={formData.client_id}
                  onChange={(e) => setFormData({ ...formData, client_id: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="your-client-id"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Client Secret <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="password"
                  value={formData.client_secret}
                  onChange={(e) => setFormData({ ...formData, client_secret: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder={editingId ? '(leave blank to keep current)' : 'your-client-secret'}
                  required={!editingId}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">OAuth2 Scope</label>
              <input
                type="text"
                value={formData.scope}
                onChange={(e) => setFormData({ ...formData, scope: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://purl.imsglobal.org/spec/or/v1p1/scope/roster-core.readonly"
              />
              <p className="text-xs text-text-tertiary mt-1">The OAuth2 scope to request. Leave default for most SIS providers.</p>
            </div>

            <hr className="my-4" />

            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="auto_sync"
                  checked={formData.auto_sync}
                  onChange={(e) => setFormData({ ...formData, auto_sync: e.target.checked })}
                  className="rounded border-border-strong text-brand-600 focus:ring-brand-500"
                />
                <label htmlFor="auto_sync" className="text-sm font-medium text-text-secondary">
                  Enable Auto-Sync
                </label>
              </div>
              {formData.auto_sync && (
                <div className="flex items-center gap-2">
                  <label className="text-sm text-text-secondary">Every</label>
                  <input
                    type="number"
                    value={formData.auto_sync_interval}
                    onChange={(e) => setFormData({ ...formData, auto_sync_interval: e.target.value })}
                    className="w-20 rounded-md border border-border-strong px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                    min="1"
                    max="168"
                  />
                  <label className="text-sm text-text-secondary">hours</label>
                </div>
              )}
            </div>

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting}
                className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                {submitting ? 'Saving...' : editingId ? 'Update Connection' : 'Create Connection'}
              </button>
              <button
                type="button"
                onClick={resetForm}
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
            <h3 className="font-semibold text-text-primary mb-2">Delete OneRoster Connection</h3>
            <p className="text-sm text-text-secondary mb-4">
              Are you sure you want to delete this connection? Sync history will be preserved but no further syncs will occur.
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
                {deleting ? 'Deleting...' : 'Delete Connection'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Connections List */}
      {loading ? (
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading OneRoster connections...
</div>
      ) : connections.length === 0 && !showForm ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Database className="w-12 h-12 text-text-disabled mx-auto mb-4" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No OneRoster Connections</h3>
          <p className="text-text-tertiary text-sm mb-4">
            Connect your Student Information System (SIS) via OneRoster 1.1 to automatically sync rosters, users, and enrollments.
          </p>
          <button
            onClick={() => setShowForm(true)}
            className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
          >
            <Plus className="w-4 h-4" />
            Add Your First Connection
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {connections.map((conn) => (
            <div key={conn.id} className="bg-surface-0 rounded-lg shadow overflow-hidden">
              {/* Connection header row */}
              <div
                className="px-6 py-4 flex items-center justify-between cursor-pointer hover:bg-surface-1"
                onClick={() => handleSelectConnection(conn)}
              >
                <div className="flex items-center gap-4 flex-1 min-w-0">
                  <div className="flex-shrink-0">
                    {selectedConnection && selectedConnection.id === conn.id ? (
                      <ChevronDown className="w-5 h-5 text-text-disabled" />
                    ) : (
                      <ChevronRight className="w-5 h-5 text-text-disabled" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-3">
                      <h4 className="text-sm font-semibold text-text-primary truncate">{conn.name}</h4>
                      {getStatusBadge(conn.sync_status, STATUS_BADGES)}
                      {conn.auto_sync && (
                        <span className="text-xs px-2 py-1 rounded-full bg-indigo-100 text-indigo-700 font-medium">
                          Auto every {conn.auto_sync_interval}h
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-4 mt-1">
                      <p className="text-xs text-text-tertiary truncate">{conn.base_url}</p>
                      {conn.last_sync_at && (
                        <p className="text-xs text-text-disabled flex items-center gap-1 flex-shrink-0">
                          <Clock className="w-3 h-3" />
                          Last sync: {formatDate(conn.last_sync_at)}
                        </p>
                      )}
                    </div>
                    {conn.sync_status === 'error' && conn.last_sync_error && (
                      <p className="text-xs text-accent-danger mt-1 truncate">{conn.last_sync_error}</p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 ms-4 flex-shrink-0" onClick={(e) => e.stopPropagation()}>
                  <button
                    onClick={() => handleTestConnection(conn.id)}
                    disabled={testingId === conn.id || syncingId === conn.id}
                    className="text-xs bg-surface-2 text-text-secondary px-3 py-1.5 rounded hover:bg-border-default disabled:opacity-50"
                    title="Test Connection"
                  >
                    {testingId === conn.id ? 'Testing...' : 'Test'}
                  </button>
                  <button
                    onClick={() => handleSync(conn.id, 'full')}
                    disabled={syncingId === conn.id || conn.sync_status === 'syncing'}
                    className="text-xs bg-brand-100 text-brand-700 px-3 py-1.5 rounded hover:bg-brand-100 disabled:opacity-50 flex items-center gap-1"
                    title="Full Sync"
                  >
                    {(syncingId === conn.id || conn.sync_status === 'syncing') ? (
                      <><Loader2 className="w-3 h-3 animate-spin" /> Syncing</>
                    ) : (
                      <><RefreshCw className="w-3 h-3" /> Sync</>
                    )}
                  </button>
                  <button
                    onClick={() => handleSync(conn.id, 'incremental')}
                    disabled={syncingId === conn.id || conn.sync_status === 'syncing' || !conn.last_sync_at}
                    className="text-xs bg-accent-success/20 text-accent-success px-3 py-1.5 rounded hover:bg-accent-success/30 disabled:opacity-50"
                    title={!conn.last_sync_at ? 'Run a full sync first' : 'Incremental Sync (changes since last sync)'}
                  >
                    Incremental
                  </button>
                  <button
                    onClick={() => handleEdit(conn)}
                    className="text-text-tertiary hover:text-text-secondary p-1.5"
                    title="Edit connection"
                  >
                    <Edit3 className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => setDeleteConfirm(conn.id)}
                    className="text-accent-danger hover:text-accent-danger p-1.5"
                    title="Delete connection"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Sync History (expanded) */}
              {selectedConnection && selectedConnection.id === conn.id && (
                <div className="border-t border-border-default bg-surface-1 px-6 py-4">
                  <h4 className="text-sm font-semibold text-text-secondary mb-3">Sync History</h4>

                  {syncLogsLoading ? (
                    <div className="text-center py-4 text-text-disabled text-sm">Loading sync history...</div>
                  ) : syncLogs.length === 0 ? (
                    <div className="text-center py-4 text-text-disabled text-sm">No sync history yet. Run a sync to see results here.</div>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="min-w-full text-sm">
                        <thead>
                          <tr className="text-start text-xs text-text-tertiary uppercase tracking-wider">
                            <th className="pb-2 pe-4">Type</th>
                            <th className="pb-2 pe-4">Status</th>
                            <th className="pb-2 pe-4">Orgs</th>
                            <th className="pb-2 pe-4">Users</th>
                            <th className="pb-2 pe-4">Classes</th>
                            <th className="pb-2 pe-4">Enrollments</th>
                            <th className="pb-2 pe-4">Errors</th>
                            <th className="pb-2 pe-4">Started</th>
                            <th className="pb-2 pe-4">Duration</th>
                            <th className="pb-2"></th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-border-default">
                          {syncLogs.map((log) => {
                            const errors = parseErrorDetails(log.error_details);
                            return (
                              <React.Fragment key={log.id}>
                                <tr className="hover:bg-surface-0">
                                  <td className="py-2 pe-4">
                                    <span className={`text-xs px-2 py-0.5 rounded font-medium ${
                                      log.sync_type === 'full' ? 'bg-brand-50 text-brand-700' : 'bg-accent-success/10 text-accent-success'
                                    }`}>
                                      {log.sync_type === 'full' ? 'Full' : 'Incremental'}
                                    </span>
                                  </td>
                                  <td className="py-2 pe-4">{getStatusBadge(log.status, SYNC_LOG_STATUS)}</td>
                                  <td className="py-2 pe-4 text-text-secondary">
                                    {log.orgs_created > 0 && <span className="text-accent-success">+{log.orgs_created}</span>}
                                    {log.orgs_created > 0 && log.orgs_updated > 0 && ' / '}
                                    {log.orgs_updated > 0 && <span className="text-brand-600">{log.orgs_updated} upd</span>}
                                    {log.orgs_created === 0 && log.orgs_updated === 0 && '-'}
                                  </td>
                                  <td className="py-2 pe-4 text-text-secondary">
                                    {log.users_created > 0 && <span className="text-accent-success">+{log.users_created}</span>}
                                    {log.users_created > 0 && log.users_updated > 0 && ' / '}
                                    {log.users_updated > 0 && <span className="text-brand-600">{log.users_updated} upd</span>}
                                    {log.users_created === 0 && log.users_updated === 0 && '-'}
                                  </td>
                                  <td className="py-2 pe-4 text-text-secondary">
                                    {log.classes_created > 0 && <span className="text-accent-success">+{log.classes_created}</span>}
                                    {log.classes_created > 0 && log.classes_updated > 0 && ' / '}
                                    {log.classes_updated > 0 && <span className="text-brand-600">{log.classes_updated} upd</span>}
                                    {log.classes_created === 0 && log.classes_updated === 0 && '-'}
                                  </td>
                                  <td className="py-2 pe-4 text-text-secondary">
                                    {log.enrollments_created > 0 && <span className="text-accent-success">+{log.enrollments_created}</span>}
                                    {log.enrollments_created > 0 && log.enrollments_updated > 0 && ' / '}
                                    {log.enrollments_updated > 0 && <span className="text-brand-600">{log.enrollments_updated} upd</span>}
                                    {log.enrollments_created === 0 && log.enrollments_updated === 0 && '-'}
                                  </td>
                                  <td className="py-2 pe-4">
                                    {log.errors > 0 ? (
                                      <span className="text-accent-danger font-medium">{log.errors}</span>
                                    ) : (
                                      <span className="text-text-disabled">0</span>
                                    )}
                                  </td>
                                  <td className="py-2 pe-4 text-text-tertiary text-xs whitespace-nowrap">
                                    {formatDate(log.started_at)}
                                  </td>
                                  <td className="py-2 pe-4 text-text-tertiary text-xs whitespace-nowrap">
                                    {formatDuration(log.started_at, log.completed_at)}
                                  </td>
                                  <td className="py-2">
                                    {errors.length > 0 && (
                                      <button
                                        onClick={() => toggleErrorDetails(log.id)}
                                        className="text-xs text-accent-danger hover:text-accent-danger underline"
                                      >
                                        {expandedErrors[log.id] ? 'Hide' : 'Details'}
                                      </button>
                                    )}
                                  </td>
                                </tr>
                                {expandedErrors[log.id] && errors.length > 0 && (
                                  <tr>
                                    <td colSpan="10" className="py-2 px-4">
                                      <div className="bg-accent-danger/10 rounded p-3 text-xs text-accent-danger max-h-48 overflow-y-auto">
                                        <ul className="list-disc list-inside space-y-1">
                                          {errors.map((errMsg, idx) => (
                                            <li key={idx}>{errMsg}</li>
                                          ))}
                                        </ul>
                                      </div>
                                    </td>
                                  </tr>
                                )}
                              </React.Fragment>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </Layout>
  );
};

export default OneRosterPage;
