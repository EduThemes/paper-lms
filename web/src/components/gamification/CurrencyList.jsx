import React from 'react';
import { api } from '../../services/api';
import { CurrencyIcon } from './currencyIcon';
import CurrencyEditor from './CurrencyEditor';
import ScopedEntityList from '../scoped/ScopedEntityList';

// CurrencyList — per-scope currency table. Thin config wrapper around
// the shared <ScopedEntityList>; the bulk of the table/CRUD wiring
// lives there (same shape used by BadgesList).
export default function CurrencyList({ courseId }) {
  return (
    <ScopedEntityList
      entityName="currency"
      entityNamePlural="currencies"
      pageTitle="Currencies"
      pageSubtitle={(scope) =>
        scope === 'course'
          ? 'Course-scoped currencies. Only visible inside this course.'
          : 'Site-wide currencies. Available to every course.'
      }
      emptyStateCopy={(scope) =>
        scope === 'course'
          ? 'No course-scoped currencies yet. Click "New currency" to create one.'
          : 'No currencies — the seeder should have created the four system rows. Check the server logs.'
      }
      courseId={courseId}
      apiCalls={{
        list: () => api.gamification.listCurrencies(),
        create: (body, opts) => api.gamification.createCurrency(body, opts),
        update: (id, body, opts) => api.gamification.updateCurrency(id, body, opts),
        delete: (id, opts) => api.gamification.deleteCurrency(id, opts),
      }}
      resultsKey="currencies"
      sortRows={(rows) =>
        [...rows].sort((a, b) => (a.display_order ?? 99) - (b.display_order ?? 99))
      }
      deleteConfirmMessage={(row) =>
        `Delete "${row.display_label}"? Existing wallet balances will keep their currency_type_id but this currency will no longer be addressable by name.`
      }
      columns={[
        {
          header: 'Code / Label',
          render: (row, { Lock }) => (
            <div className="flex items-center gap-2">
              <CurrencyIcon icon={row.icon} color={row.color} className="w-4 h-4" />
              <div className="min-w-0">
                <div className="text-text-primary truncate flex items-center gap-1.5">
                  {row.display_label}
                  {row.system_owned && (
                    <Lock className="w-3 h-3 text-text-tertiary" aria-label="System currency" />
                  )}
                </div>
                <code className="text-xs text-text-tertiary">{row.code}</code>
              </div>
            </div>
          ),
        },
        {
          header: 'Behavior',
          render: (row) => (
            <span className="text-text-secondary text-xs">
              {[
                row.spendable ? 'spendable' : null,
                row.monotonic ? 'monotonic' : null,
                !row.visible_to_student ? 'instructor-only' : null,
              ]
                .filter(Boolean)
                .join(' · ') || '—'}
            </span>
          ),
        },
        {
          header: 'Topbar',
          render: (row) =>
            row.visible_in_topbar ? (
              <span className="text-accent-success text-xs">Visible</span>
            ) : (
              <span className="text-text-tertiary text-xs">Hidden</span>
            ),
        },
      ]}
      renderEditor={({ open, onOpenChange, row, onSave, saving, saveError }) => (
        <CurrencyEditor
          open={open}
          onOpenChange={onOpenChange}
          currency={row}
          onSave={onSave}
          saving={saving}
          saveError={saveError}
        />
      )}
      // Tell any mounted CurrencyPills to re-fetch so the new currency
      // shows immediately for currently-signed-in users.
      onAfterMutate={() => window.dispatchEvent(new Event('wallet:refresh'))}
    />
  );
}
