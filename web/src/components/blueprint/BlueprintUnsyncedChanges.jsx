import React from 'react';
import { RefreshCw } from 'lucide-react';
import { useUnsyncedChanges } from '../../services/api_content_migration';

const ASSET_LABELS = {
  assignment: 'Assignment',
  wiki_page: 'Page',
  quiz: 'Quiz',
  discussion_topic: 'Discussion',
  context_module: 'Module',
};

// BlueprintUnsyncedChanges lists what the next sync will push: content
// created or edited since the last sync, and content deleted from the
// blueprint (whose copies the sync removes).
export default function BlueprintUnsyncedChanges({ courseId }) {
  const changes = useUnsyncedChanges(courseId);
  const list = changes.data || [];
  return (
    <div className="bg-surface-0 rounded-lg shadow mb-6">
      <div className="px-6 py-4 border-b border-border-default flex items-center justify-between">
        <h3 className="text-lg font-semibold text-text-primary">Unsynced Changes</h3>
        <button type="button" onClick={() => changes.refetch()} className="text-text-tertiary hover:text-text-secondary p-1" title="Refresh" aria-label="Refresh unsynced changes">
          <RefreshCw className={`w-4 h-4 ${changes.isFetching ? 'animate-spin' : ''}`} />
        </button>
      </div>
      {changes.isError && <div role="alert" className="px-6 py-3 text-sm text-accent-danger">{changes.error?.message}</div>}
      {changes.isSuccess && list.length === 0 && (
        <div className="px-6 py-6 text-sm text-text-tertiary">Everything is in sync.</div>
      )}
      {list.length > 0 && (
        <ul className="divide-y divide-border-default">
          {list.map((ch) => (
            <li key={`${ch.change_type}-${ch.asset_type}-${ch.asset_id}`} className="px-6 py-2 flex items-center justify-between text-sm">
              <span className="text-text-primary">
                <span className="text-text-tertiary">{ASSET_LABELS[ch.asset_type] || ch.asset_type}:</span>{' '}
                {ch.asset_name || `#${ch.asset_id}`}
              </span>
              <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${ch.change_type === 'deleted' ? 'bg-accent-danger/10 text-accent-danger' : 'bg-surface-2 text-text-secondary'}`}>
                {ch.change_type === 'deleted' ? 'Deleted' : 'Changed'}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
