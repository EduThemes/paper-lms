// Per-category x per-channel notification preferences (G091), Canvas's
// notification settings grid: one row per category, one column per
// delivery channel (account email first, then confirmed channels). A cell
// without an explicit choice inherits the global delivery policy above.

import React from 'react';
import { Link } from 'react-router-dom';
import { Grid3x3, Info } from 'lucide-react';
import { useNotificationPolicies, useUpdateNotificationPolicies } from '../services/queries_notify_inbox';
import { policyCell, channelLabel } from '../utils/notifyInbox';

const FREQUENCY_LABELS = {
  immediately: 'Right away',
  daily: 'Daily summary',
  weekly: 'Weekly summary',
  never: 'Off',
};

const NotificationPolicyGrid = () => {
  const query = useNotificationPolicies();
  const mutation = useUpdateNotificationPolicies();

  if (query.isLoading) return <div className="bg-surface-0 rounded-lg shadow p-6 text-sm text-text-tertiary">Loading notification settings...</div>;
  if (query.isError) {
    return (
      <div className="bg-surface-0 rounded-lg shadow p-6 text-sm text-accent-danger" role="alert">
        {query.error?.message || 'Could not load notification settings'}
        <button type="button" onClick={() => query.refetch()} className="ml-2 underline">Try again</button>
      </div>
    );
  }

  const grid = query.data || {};
  const channels = grid.channels || [];
  const categories = grid.categories || [];
  const frequencies = grid.frequencies || Object.keys(FREQUENCY_LABELS);
  const stubTypes = (grid.channel_types || []).filter((t) => !t.available);

  const setCell = (channelId, category, frequency) => {
    mutation.mutate([{ channel_id: channelId, category, frequency }]);
  };

  return (
    <div className="bg-surface-0 rounded-lg shadow p-6">
      <div className="flex items-center gap-2 mb-2">
        <Grid3x3 className="w-5 h-5 text-text-tertiary" aria-hidden="true" />
        <h3 className="text-lg font-semibold text-text-primary">Notifications by type and channel</h3>
      </div>
      <p className="text-sm text-text-secondary mb-4">
        Choose how often each kind of notification reaches each of your channels. Cells marked
        &ldquo;default&rdquo; follow the delivery policy above. Every notification also appears in your
        {' '}<Link to="/notifications" className="text-brand-600 hover:underline">notification feed</Link>.
      </p>
      {mutation.isError && (
        <p className="text-sm text-accent-danger mb-3" role="alert">{mutation.error?.message}</p>
      )}
      {channels.length === 0 ? (
        <p className="text-sm text-text-tertiary">
          You have no delivery channels yet. <Link to="/settings/notification_deliveries" className="text-brand-600 hover:underline">Add one</Link>.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm" aria-label="Notification preferences by type and channel">
            <thead>
              <tr className="border-b border-border-default">
                <th scope="col" className="text-left py-2 pr-4 font-medium text-text-secondary">Notification</th>
                {channels.map((ch) => (
                  <th key={ch.id} scope="col" className="text-left py-2 px-2 font-medium text-text-secondary">
                    <span className="block max-w-[14rem] truncate" title={channelLabel(ch)}>{channelLabel(ch)}</span>
                    {!ch.deliverable && <span className="block text-xs font-normal text-accent-warning">{ch.confirmed ? 'Not deliverable' : 'Unconfirmed'}</span>}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-border-default">
              {categories.map((cat) => (
                <tr key={cat.key}>
                  <th scope="row" className="text-left py-2 pr-4 align-top font-normal">
                    <span className="block font-medium text-text-primary">{cat.name}</span>
                    <span className="block text-xs text-text-tertiary">{cat.description}</span>
                  </th>
                  {channels.map((ch) => {
                    const cell = policyCell(grid, ch.id, cat.key);
                    const value = cell?.explicit ? cell.frequency : 'default';
                    return (
                      <td key={ch.id} className="py-2 px-2 align-top">
                        <select
                          value={value}
                          disabled={mutation.isPending}
                          onChange={(e) => setCell(ch.id, cat.key, e.target.value)}
                          aria-label={`${cat.name} on ${channelLabel(ch)}`}
                          className="border border-border-strong rounded-md px-2 py-1 text-sm bg-surface-0"
                        >
                          <option value="default">
                            {cell?.explicit ? 'Use default' : `Default (${FREQUENCY_LABELS[cell?.frequency] || cell?.frequency || 'Daily summary'})`}
                          </option>
                          {frequencies.map((f) => (
                            <option key={f} value={f}>{FREQUENCY_LABELS[f] || f}</option>
                          ))}
                        </select>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {stubTypes.length > 0 && (
        <p className="mt-4 text-xs text-text-tertiary flex items-start gap-1">
          <Info className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" aria-hidden="true" />
          {stubTypes.map((t) => t.reason).join(' ')}
        </p>
      )}
    </div>
  );
};

export default NotificationPolicyGrid;
