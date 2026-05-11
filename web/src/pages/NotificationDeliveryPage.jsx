import React, { useState, useEffect, useCallback } from 'react';
import { Mail, CheckCircle, Clock, AlertTriangle, XCircle, RefreshCw, Plus, Trash2 } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

async function apiFetch(path, options = {}) {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.errors?.[0]?.message || `Request failed: ${response.status}`);
  }
  return response.json();
}

const STATUS_CONFIG = {
  delivered: { label: 'Delivered', color: 'bg-accent-success/20 text-accent-success', icon: CheckCircle },
  sent:      { label: 'Sent',      color: 'bg-accent-success/10 text-accent-success',  icon: CheckCircle },
  pending:   { label: 'Pending',   color: 'bg-accent-warning/20 text-accent-warning', icon: Clock },
  queued:    { label: 'Queued',    color: 'bg-brand-100 text-brand-800',   icon: Clock },
  failed:    { label: 'Failed',    color: 'bg-accent-danger/20 text-accent-danger',     icon: XCircle },
  bounced:   { label: 'Bounced',   color: 'bg-surface-2 text-text-primary',   icon: AlertTriangle },
};

const StatusBadge = ({ status }) => {
  const config = STATUS_CONFIG[status] || { label: status, color: 'bg-surface-2 text-text-secondary', icon: Clock };
  const Icon = config.icon;
  return (
    <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium ${config.color}`}>
      <Icon className="w-3 h-3" />
      {config.label}
    </span>
  );
};

const CHANNEL_ICONS = {
  email: Mail,
  webhook: RefreshCw,
  push: CheckCircle,
};

const NotificationDeliveryPage = () => {
  const { user } = useAuth();
  const isAdmin = user?.role === 'admin';

  const [deliveries, setDeliveries] = useState([]);
  const [channels, setChannels] = useState([]);
  const [stats, setStats] = useState(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [retrying, setRetrying] = useState(false);

  // New channel form state
  const [showAddChannel, setShowAddChannel] = useState(false);
  const [newChannelType, setNewChannelType] = useState('email');
  const [newChannelAddress, setNewChannelAddress] = useState('');
  const [addingChannel, setAddingChannel] = useState(false);

  const perPage = 20;

  const fetchDeliveries = useCallback(async () => {
    try {
      const statusParam = statusFilter ? `&status=${statusFilter}` : '';
      const data = await apiFetch(`/users/self/notification_deliveries?page=${page}&per_page=${perPage}${statusParam}`);
      setDeliveries(data);
    } catch (err) {
      setError(err.message);
    }
  }, [page, statusFilter]);

  const fetchChannels = useCallback(async () => {
    try {
      const data = await apiFetch('/users/self/communication_channels');
      setChannels(data);
    } catch (err) {
      // Channels may not be set up yet; ignore
    }
  }, []);

  const fetchStats = useCallback(async () => {
    if (!isAdmin) return;
    try {
      const data = await apiFetch('/admin/notification_stats');
      setStats(data.delivery_stats);
    } catch (err) {
      // Non-critical
    }
  }, [isAdmin]);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchDeliveries(), fetchChannels(), fetchStats()]).finally(() => setLoading(false));
  }, [fetchDeliveries, fetchChannels, fetchStats]);

  const handleRetryFailed = async () => {
    setRetrying(true);
    setError(null);
    try {
      const data = await apiFetch('/admin/notification_deliveries/retry', { method: 'POST' });
      setSuccess(`${data.retried} failed deliveries queued for retry.`);
      setTimeout(() => setSuccess(null), 4000);
      fetchDeliveries();
      fetchStats();
    } catch (err) {
      setError(err.message);
    } finally {
      setRetrying(false);
    }
  };

  const handleAddChannel = async (e) => {
    e.preventDefault();
    setAddingChannel(true);
    setError(null);
    try {
      await apiFetch('/users/self/communication_channels', {
        method: 'POST',
        body: JSON.stringify({
          communication_channel: {
            channel_type: newChannelType,
            address: newChannelAddress,
          },
        }),
      });
      setNewChannelAddress('');
      setShowAddChannel(false);
      setSuccess('Communication channel added successfully.');
      setTimeout(() => setSuccess(null), 3000);
      fetchChannels();
    } catch (err) {
      setError(err.message);
    } finally {
      setAddingChannel(false);
    }
  };

  const handleDeleteChannel = async (channelId) => {
    if (!window.confirm('Are you sure you want to remove this communication channel?')) return;
    setError(null);
    try {
      await apiFetch(`/users/self/communication_channels/${channelId}`, { method: 'DELETE' });
      setSuccess('Communication channel removed.');
      setTimeout(() => setSuccess(null), 3000);
      fetchChannels();
    } catch (err) {
      setError(err.message);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '--';
    return new Date(dateStr).toLocaleString();
  };

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading notification deliveries...
</div>
      </Layout>
    );
  }

  return (
    <Layout>
    <div className="max-w-6xl mx-auto">
      {/* Page Header */}
      <div className="flex items-center gap-3 mb-6">
        <div className="bg-brand-100 p-2 rounded-lg">
          <Mail className="w-6 h-6 text-brand-600" />
        </div>
        <div>
          <h2 className="text-2xl font-bold text-text-primary">Notification Deliveries</h2>
          <p className="text-text-secondary mt-0.5 text-sm">
            View delivery status, manage communication channels, and track notification history.
          </p>
        </div>
      </div>

      {/* Alerts */}
      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          <span>{error}</span>
          <button onClick={() => setError(null)} className="ml-auto text-accent-danger hover:text-accent-danger">
            <XCircle className="w-4 h-4" />
          </button>
        </div>
      )}
      {success && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <CheckCircle className="w-4 h-4 flex-shrink-0" />
          <span>{success}</span>
        </div>
      )}

      {/* Admin Stats Dashboard */}
      {isAdmin && stats && (
        <div className="mb-8">
          <h3 className="text-lg font-semibold text-text-primary mb-3">Delivery Statistics</h3>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
            {['pending', 'queued', 'sent', 'delivered', 'failed', 'bounced'].map((status) => {
              const config = STATUS_CONFIG[status];
              const Icon = config.icon;
              return (
                <div key={status} className="bg-surface-0 rounded-lg shadow p-4 text-center">
                  <Icon className="w-5 h-5 mx-auto mb-1 text-text-disabled" />
                  <p className="text-2xl font-bold text-text-primary">{stats[status] || 0}</p>
                  <p className="text-xs text-text-tertiary capitalize">{config.label}</p>
                </div>
              );
            })}
          </div>
          {(stats.failed || 0) > 0 && (
            <div className="mt-4">
              <button
                onClick={handleRetryFailed}
                disabled={retrying}
                className="inline-flex items-center gap-2 bg-accent-danger text-white px-4 py-2 rounded-md hover:bg-accent-danger/90 text-sm font-medium disabled:opacity-50 transition-colors"
              >
                <RefreshCw className={`w-4 h-4 ${retrying ? 'animate-spin' : ''}`} />
                {retrying ? 'Retrying...' : 'Retry Failed Deliveries'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Communication Channels */}
      <div className="mb-8">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-lg font-semibold text-text-primary">Communication Channels</h3>
          <button
            onClick={() => setShowAddChannel(!showAddChannel)}
            className="inline-flex items-center gap-1.5 text-sm text-brand-600 hover:text-brand-800 font-medium"
          >
            <Plus className="w-4 h-4" />
            Add Channel
          </button>
        </div>

        {showAddChannel && (
          <form onSubmit={handleAddChannel} className="bg-surface-0 rounded-lg shadow p-4 mb-4">
            <div className="flex flex-col sm:flex-row gap-3">
              <div>
                <label htmlFor="channel-type" className="block text-xs font-medium text-text-secondary mb-1">Type</label>
                <select
                  id="channel-type"
                  value={newChannelType}
                  onChange={(e) => setNewChannelType(e.target.value)}
                  className="rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="email">Email</option>
                  <option value="webhook">Webhook</option>
                </select>
              </div>
              <div className="flex-1">
                <label htmlFor="channel-address" className="block text-xs font-medium text-text-secondary mb-1">
                  {newChannelType === 'email' ? 'Email Address' : 'Webhook URL'}
                </label>
                <input
                  id="channel-address"
                  type={newChannelType === 'email' ? 'email' : 'url'}
                  value={newChannelAddress}
                  onChange={(e) => setNewChannelAddress(e.target.value)}
                  placeholder={newChannelType === 'email' ? 'you@example.com' : 'https://hooks.example.com/notify'}
                  required
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
              </div>
              <div className="flex items-end">
                <button
                  type="submit"
                  disabled={addingChannel}
                  className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50 transition-colors"
                >
                  {addingChannel ? 'Adding...' : 'Add'}
                </button>
              </div>
            </div>
          </form>
        )}

        {channels.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary text-sm">
            No communication channels configured. Notifications will be sent to your account email.
          </div>
        ) : (
          <div className="bg-surface-0 rounded-lg shadow divide-y divide-gray-100">
            {channels.map((ch) => {
              const Icon = CHANNEL_ICONS[ch.channel_type] || Mail;
              return (
                <div key={ch.id} className="flex items-center justify-between px-4 py-3">
                  <div className="flex items-center gap-3">
                    <Icon className="w-5 h-5 text-text-disabled" />
                    <div>
                      <p className="text-sm font-medium text-text-primary">{ch.address}</p>
                      <p className="text-xs text-text-tertiary capitalize">{ch.channel_type} &middot; Position {ch.position}</p>
                    </div>
                    {ch.confirmed && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-accent-success/20 text-accent-success">
                        <CheckCircle className="w-3 h-3" />
                        Confirmed
                      </span>
                    )}
                  </div>
                  <button
                    onClick={() => handleDeleteChannel(ch.id)}
                    className="text-text-disabled hover:text-accent-danger transition-colors p-1"
                    title="Remove channel"
                    aria-label={`Remove ${ch.channel_type} channel ${ch.address}`}
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Delivery Log */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-lg font-semibold text-text-primary">Delivery Log</h3>
          <div className="flex items-center gap-2">
            <label htmlFor="status-filter" className="text-sm text-text-secondary">Status:</label>
            <select
              id="status-filter"
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
              className="rounded-md border border-border-strong px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="">All</option>
              <option value="pending">Pending</option>
              <option value="queued">Queued</option>
              <option value="sent">Sent</option>
              <option value="delivered">Delivered</option>
              <option value="failed">Failed</option>
              <option value="bounced">Bounced</option>
            </select>
          </div>
        </div>

        {deliveries.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary text-sm">
            No notification deliveries found{statusFilter ? ` with status "${statusFilter}"` : ''}.
          </div>
        ) : (
          <>
            <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-border-default">
                  <thead className="bg-surface-1">
                    <tr>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Date</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Subject</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Channel</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Digest</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Retries</th>
                    </tr>
                  </thead>
                  <tbody className="bg-surface-0 divide-y divide-gray-100">
                    {deliveries.map((d) => {
                      const ChIcon = CHANNEL_ICONS[d.channel_type] || Mail;
                      return (
                        <tr key={d.id} className="hover:bg-surface-1">
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">
                            {formatDate(d.created_at)}
                          </td>
                          <td className="px-4 py-3 text-sm text-text-primary max-w-xs truncate" title={d.subject}>
                            {d.subject}
                          </td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">
                            <span className="inline-flex items-center gap-1.5">
                              <ChIcon className="w-3.5 h-3.5" />
                              <span className="capitalize">{d.channel_type}</span>
                            </span>
                          </td>
                          <td className="px-4 py-3 whitespace-nowrap">
                            <StatusBadge status={d.delivery_status} />
                          </td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary capitalize">
                            {d.digest_type || '--'}
                          </td>
                          <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary">
                            {d.retry_count > 0 ? (
                              <span className="text-accent-danger" title={d.last_error || ''}>
                                {d.retry_count}/{d.max_retries}
                              </span>
                            ) : (
                              '--'
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Pagination */}
            <div className="flex items-center justify-between mt-4">
              <button
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
                className="px-3 py-1.5 text-sm border border-border-strong rounded-md text-text-secondary hover:bg-surface-1 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Previous
              </button>
              <span className="text-sm text-text-tertiary">Page {page}</span>
              <button
                onClick={() => setPage((p) => p + 1)}
                disabled={deliveries.length < perPage}
                className="px-3 py-1.5 text-sm border border-border-strong rounded-md text-text-secondary hover:bg-surface-1 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next
              </button>
            </div>
          </>
        )}
      </div>
    </div>
    </Layout>
  );
};

export default NotificationDeliveryPage;
