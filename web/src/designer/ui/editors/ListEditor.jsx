/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useDesignerT } from '../i18n';

function SmallIcon({ label, onClick, disabled, children }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={label}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
    >
      {children}
      <span className="sr-only">{label}</span>
    </button>
  );
}

/**
 * Edits an ordered list of items (links, objectives, weeks…). Each item gets a small card with move up,
 * move down and remove. `onChange(items, key)` passes a coalesce key for edits to one field and null for
 * structural changes, so typing undoes as one step.
 */
export function ListEditor({ items, onChange, title, renderItem, newItem, addLabel, itemKey, min = 0 }) {
  const { t } = useDesignerT();
  const move = (i, d) => {
    const next = [...items];
    const [x] = next.splice(i, 1);
    next.splice(i + d, 0, x);
    onChange(next, null);
  };
  return (
    <div className="space-y-2.5">
      {items.map((item, i) => {
        const name = title(item, i);
        const key = itemKey(item, i);
        return (
          <div key={key} role="group" aria-label={name} className="space-y-2.5 rounded-md border border-border bg-surface-1 p-2.5">
            <div className="flex items-center justify-between gap-2">
              <span className="min-w-0 truncate text-sm font-semibold text-text-primary">{name}</span>
              <span className="flex shrink-0">
                <SmallIcon label={`${t('field.moveUp')}: ${name}`} disabled={i === 0} onClick={() => move(i, -1)}>
                  <ArrowUp className="h-4 w-4" aria-hidden="true" />
                </SmallIcon>
                <SmallIcon label={`${t('field.moveDown')}: ${name}`} disabled={i === items.length - 1} onClick={() => move(i, 1)}>
                  <ArrowDown className="h-4 w-4" aria-hidden="true" />
                </SmallIcon>
                <SmallIcon label={t('field.removeItem', { name })} disabled={items.length <= min} onClick={() => onChange(items.filter((_, j) => j !== i), null)}>
                  <Trash2 className="h-4 w-4" aria-hidden="true" />
                </SmallIcon>
              </span>
            </div>
            {renderItem(item, (patch, field) => onChange(items.map((x, j) => (j === i ? { ...x, ...patch } : x)), `${key}.${field}`), i)}
          </div>
        );
      })}
      <Button type="button" size="sm" variant="outline" onClick={() => onChange([...items, newItem()], null)}>
        <Plus aria-hidden="true" />
        {addLabel}
      </Button>
    </div>
  );
}
