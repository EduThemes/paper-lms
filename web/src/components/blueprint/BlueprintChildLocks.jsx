import React from 'react';
import { Lock } from 'lucide-react';
import { useBlueprintSubscriptions, useChildLockedContent } from '../../services/api_content_migration';
import { blueprintTypeLabel, describeRestrictions } from '../../lib/contentMigration';

// BlueprintChildLocks is shown in a course associated with a blueprint:
// it names the blueprint and lists the locked content the course cannot
// change (G069). Renders nothing for a course with no blueprint.
export default function BlueprintChildLocks({ courseId }) {
  const subs = useBlueprintSubscriptions(courseId);
  const active = (subs.data || []).filter((s) => s.workflow_state === 'active');
  const locked = useChildLockedContent(courseId, active.length > 0);
  if (active.length === 0) return null;
  const items = locked.data || [];

  return (
    <section aria-label="Blueprint" className="bg-brand-50 border border-brand-200 rounded-lg p-4 mb-6">
      <h3 className="text-sm font-semibold text-text-primary flex items-center gap-2">
        <Lock className="w-4 h-4" />
        This course receives content from a blueprint course
      </h3>
      <p className="text-xs text-text-secondary mt-1">
        Blueprint syncs update the content they created here. Locked items below cannot be changed or deleted in this course.
      </p>
      {items.length > 0 ? (
        <ul className="mt-3 space-y-1">
          {items.map((it) => (
            <li key={`${it.content_type}-${it.content_id}`} className="text-sm text-text-primary">
              <span className="text-text-tertiary">{blueprintTypeLabel(it.content_type)}:</span> {it.title || `#${it.content_id}`}
              <span className="ml-2 text-xs text-text-tertiary">{describeRestrictions(it.restrictions)}</span>
            </li>
          ))}
        </ul>
      ) : (
        locked.isSuccess && <p className="mt-2 text-xs text-text-tertiary">No blueprint content is locked.</p>
      )}
    </section>
  );
}
