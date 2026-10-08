import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, ChevronRight } from 'lucide-react';
import {
  COPY_TYPES,
  copyTypeLabel,
  isItemSelected,
  toggleItem,
  toggleType,
} from '../../lib/contentMigration';

// ContentSelectionPicker lists a source course's content by type (from
// GET .../content_migrations/selectable) with a checkbox per type and,
// expanded, per item — Canvas's "Select specific content" dialog.
export default function ContentSelectionPicker({ types, selection, onChange }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState({});
  const order = COPY_TYPES.map((c) => c.key);
  const sorted = [...(types || [])].sort((a, b) => order.indexOf(a.type) - order.indexOf(b.type));

  if (sorted.length === 0) {
    return <p className="text-sm text-text-tertiary">{t('contentMigration.select.empty', 'This course has no content to copy.')}</p>;
  }

  return (
    <ul className="divide-y divide-border-default rounded-md border border-border-default" aria-label={t('contentMigration.select.label', 'Content to copy')}>
      {sorted.map((ty) => {
        const allIds = (ty.items || []).map((i) => i.id);
        const state = selection[ty.type];
        const checked = !!state && state.all;
        const partial = !!state && !state.all && state.ids.length > 0;
        const expandable = (ty.items || []).length > 0;
        const id = `sel-${ty.type}`;
        return (
          <li key={ty.type} className="p-2">
            <div className="flex items-center gap-2">
              {expandable ? (
                <button
                  type="button"
                  aria-expanded={!!open[ty.type]}
                  aria-label={t('contentMigration.select.toggle', 'Show items')}
                  className="p-0.5 text-text-tertiary hover:text-text-primary"
                  onClick={() => setOpen({ ...open, [ty.type]: !open[ty.type] })}
                >
                  {open[ty.type] ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                </button>
              ) : (
                <span className="w-5" />
              )}
              <input
                id={id}
                type="checkbox"
                checked={checked}
                ref={(el) => { if (el) el.indeterminate = partial; }}
                onChange={(e) => onChange(toggleType(selection, ty.type, e.target.checked))}
              />
              <label htmlFor={id} className="text-sm text-text-primary">
                {copyTypeLabel(ty.type)}
                {ty.count > 0 && expandable && <span className="ml-1 text-text-tertiary">({ty.count})</span>}
              </label>
            </div>
            {expandable && open[ty.type] && (
              <ul className="mt-2 ml-11 space-y-1 max-h-56 overflow-y-auto">
                {ty.items.map((item) => {
                  const itemId = `sel-${ty.type}-${item.id}`;
                  return (
                    <li key={item.id} className="flex items-center gap-2">
                      <input
                        id={itemId}
                        type="checkbox"
                        checked={isItemSelected(selection, ty.type, item.id)}
                        onChange={(e) => onChange(toggleItem(selection, ty.type, item.id, e.target.checked, allIds))}
                      />
                      <label htmlFor={itemId} className="text-sm text-text-secondary">{item.title || `#${item.id}`}</label>
                    </li>
                  );
                })}
              </ul>
            )}
          </li>
        );
      })}
    </ul>
  );
}
