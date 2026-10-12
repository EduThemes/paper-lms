import React, { useState } from 'react';
import { Lock, Unlock, Loader2, RotateCcw } from 'lucide-react';
import { useBlueprintContent, useRestrictItem } from '../../services/api_content_migration';
import { blueprintTypeLabel, blueprintTypeParam, describeRestrictions } from '../../lib/contentMigration';
import RestrictionCheckboxes from './RestrictionCheckboxes';

// BlueprintLocksPanel lists the blueprint course's lockable content
// (assignments, quizzes, pages, discussions) with a lock toggle per item
// and, expanded, which attributes are locked (G069). Locked items cannot
// be changed in associated courses and are re-pushed on every sync.
export default function BlueprintLocksPanel({ courseId }) {
  const content = useBlueprintContent(courseId);
  const restrict = useRestrictItem(courseId);
  const [open, setOpen] = useState(null);
  const [error, setError] = useState(null);

  const save = (item, body) => {
    setError(null);
    restrict.mutate(
      { content_type: blueprintTypeParam(item.content_type), content_id: item.content_id, ...body },
      { onError: (err) => setError(err.message || 'Could not update the lock') },
    );
  };

  if (content.isLoading) {
    return <div className="py-8 text-center text-text-tertiary"><Loader2 className="w-5 h-5 animate-spin inline" /></div>;
  }
  if (content.isError) {
    return <div role="alert" className="py-4 text-accent-danger text-sm">{content.error?.message}</div>;
  }
  const items = content.data?.items || [];

  return (
    <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
      <div className="px-6 py-4 border-b border-border-default">
        <h3 className="text-lg font-semibold text-text-primary">Locked Content</h3>
        <p className="text-sm text-text-tertiary mt-1">
          Locked items cannot be edited or deleted in associated courses, and every sync overwrites local changes to them.
          Items without their own setting follow the default restrictions.
        </p>
      </div>
      {error && <div role="alert" className="mx-6 mt-4 p-3 rounded-md bg-accent-danger/10 text-accent-danger text-sm">{error}</div>}
      {items.length === 0 ? (
        <div className="py-10 text-center text-text-tertiary text-sm">This course has no assignments, quizzes, pages or discussions yet.</div>
      ) : (
        <ul className="divide-y divide-border-default">
          {items.map((item) => {
            const key = `${item.content_type}-${item.content_id}`;
            const pending = restrict.isPending && restrict.variables?.content_id === item.content_id && restrict.variables?.content_type === blueprintTypeParam(item.content_type);
            return (
              <li key={key} className="px-6 py-3">
                <div className="flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-text-primary truncate">{item.title || `#${item.content_id}`}</div>
                    <div className="text-xs text-text-tertiary">
                      {blueprintTypeLabel(item.content_type)} · {describeRestrictions(item.restrictions)}
                      {!item.explicit && ' (default)'}
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {pending && <Loader2 className="w-4 h-4 animate-spin text-text-tertiary" />}
                    {item.explicit && (
                      <button
                        type="button"
                        title="Use the default restrictions"
                        aria-label={`Use the default restrictions for ${item.title}`}
                        className="p-1.5 rounded text-text-tertiary hover:text-text-primary"
                        onClick={() => save(item, { inherit: true })}
                      >
                        <RotateCcw className="w-4 h-4" />
                      </button>
                    )}
                    <button
                      type="button"
                      aria-pressed={item.locked}
                      aria-label={`${item.locked ? 'Unlock' : 'Lock'} ${item.title}`}
                      className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-medium ${
                        item.locked ? 'bg-brand-100 text-brand-800' : 'bg-surface-2 text-text-secondary'
                      }`}
                      onClick={() => save(item, { restricted: !item.locked })}
                    >
                      {item.locked ? <Lock className="w-3.5 h-3.5" /> : <Unlock className="w-3.5 h-3.5" />}
                      {item.locked ? 'Locked' : 'Unlocked'}
                    </button>
                    <button
                      type="button"
                      aria-expanded={open === key}
                      className="text-xs text-brand-600 hover:underline"
                      onClick={() => setOpen(open === key ? null : key)}
                    >
                      Attributes
                    </button>
                  </div>
                </div>
                {open === key && (
                  <div className="mt-3 ml-1">
                    <RestrictionCheckboxes
                      idPrefix={`lock-${key}`}
                      compact
                      value={item.restrictions}
                      disabled={restrict.isPending}
                      onChange={(r) => save(item, { restrictions: r })}
                    />
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
