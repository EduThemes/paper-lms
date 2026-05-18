import React from 'react';
import { api } from '../../services/api';
import { BadgeIcon } from './BadgeIcon';
import BadgeEditor from './BadgeEditor';
import ScopedEntityList from '../scoped/ScopedEntityList';

// BadgesList — per-scope badge table. Thin config wrapper around the
// shared <ScopedEntityList>; same shape as CurrencyList (W2-B) by
// design — admins should not have to relearn the table UI between
// gamification primitives.
export default function BadgesList({ courseId }) {
  return (
    <ScopedEntityList
      entityName="badge"
      entityNamePlural="badges"
      pageTitle="Badges"
      pageSubtitle={(scope) =>
        scope === 'course'
          ? 'Course-scoped badges. Only earnable inside this course.'
          : 'Site-wide badges. Available to every course.'
      }
      emptyStateCopy={() => 'No badges yet. Click "New badge" to create one.'}
      courseId={courseId}
      apiCalls={{
        list: () => api.gamification.listBadges(),
        create: (body, opts) => api.gamification.createBadge(body, opts),
        update: (id, body, opts) => api.gamification.updateBadge(id, body, opts),
        delete: (id, opts) => api.gamification.deleteBadge(id, opts),
      }}
      resultsKey="badges"
      sortRows={(rows) =>
        [...rows].sort((a, b) => (a.name || '').localeCompare(b.name || ''))
      }
      deleteConfirmMessage={(row) =>
        `Delete "${row.name}"? This permanently removes the badge and ALL existing learner awards of it (the database CASCADEs).`
      }
      columns={[
        {
          header: 'Badge',
          render: (row, { Lock }) => (
            <div className="flex items-center gap-3">
              <BadgeIcon badge={row} size="sm" />
              <div className="min-w-0">
                <div className="text-text-primary truncate flex items-center gap-1.5">
                  {row.name}
                  {row.system_owned && (
                    <Lock className="w-3 h-3 text-text-tertiary" aria-label="System badge" />
                  )}
                </div>
                <div className="text-xs text-text-tertiary truncate max-w-md">{row.description}</div>
              </div>
            </div>
          ),
        },
        {
          header: 'Code',
          render: (row) => <code className="text-xs text-text-tertiary">{row.code}</code>,
        },
        {
          header: 'Visibility',
          render: (row) =>
            row.internal_only ? (
              <span className="text-text-secondary text-xs">Internal-only</span>
            ) : (
              <span className="text-accent-warning text-xs">External-eligible</span>
            ),
        },
      ]}
      renderEditor={({ open, onOpenChange, row, onSave, saving, saveError }) => (
        <BadgeEditor
          open={open}
          onOpenChange={onOpenChange}
          badge={row}
          onSave={onSave}
          saving={saving}
          saveError={saveError}
        />
      )}
    />
  );
}
