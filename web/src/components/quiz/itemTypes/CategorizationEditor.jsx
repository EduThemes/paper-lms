import React from 'react';
import { Plus, Trash2, Folder } from 'lucide-react';
import { makeId } from './types';

/**
 * Categorization editor. Two lists: items on the left, buckets on the right.
 * Each bucket has an `item_ids` array — items live in exactly one bucket
 * (or are unassigned). We avoid full DnD complexity here in favor of a
 * dropdown per item; this keeps keyboard accessibility easy.
 *
 * Stored in a single answer entry:
 *   { items: [{id, text}], buckets: [{id, label, item_ids: []}] }
 */
const CategorizationEditor = ({ answers, onChange }) => {
  const cfg = (Array.isArray(answers) && answers[0]) || {
    id: makeId('a'), text: '', weight: 100, items: [], buckets: [],
  };
  const items = cfg.items || [];
  const buckets = cfg.buckets || [];

  const patch = (p) => onChange([{ ...cfg, ...p }]);

  const addItem = () => {
    patch({ items: [...items, { id: makeId('i'), text: '' }] });
  };
  const updateItem = (id, text) => {
    patch({ items: items.map(i => i.id === id ? { ...i, text } : i) });
  };
  const removeItem = (id) => {
    patch({
      items: items.filter(i => i.id !== id),
      buckets: buckets.map(b => ({ ...b, item_ids: (b.item_ids || []).filter(iid => iid !== id) })),
    });
  };

  const addBucket = () => {
    patch({ buckets: [...buckets, { id: makeId('b'), label: '', item_ids: [] }] });
  };
  const updateBucket = (id, p) => {
    patch({ buckets: buckets.map(b => b.id === id ? { ...b, ...p } : b) });
  };
  const removeBucket = (id) => {
    patch({ buckets: buckets.filter(b => b.id !== id) });
  };

  const assignItemToBucket = (itemId, bucketId) => {
    const cleared = buckets.map(b => ({
      ...b,
      item_ids: (b.item_ids || []).filter(iid => iid !== itemId),
    }));
    if (!bucketId) {
      patch({ buckets: cleared });
      return;
    }
    patch({
      buckets: cleared.map(b => b.id === bucketId
        ? { ...b, item_ids: [...(b.item_ids || []), itemId] }
        : b),
    });
  };

  const bucketForItem = (itemId) =>
    buckets.find(b => (b.item_ids || []).includes(itemId))?.id || '';

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Items */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs font-medium text-text-secondary">Items</label>
            <button onClick={addItem} type="button" className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
              <Plus className="w-3 h-3" /> Add Item
            </button>
          </div>
          <div className="space-y-2">
            {items.length === 0 && (
              <p className="text-xs text-text-tertiary italic">No items yet.</p>
            )}
            {items.map(item => (
              <div key={item.id} className="flex items-center gap-2">
                <input
                  type="text"
                  value={item.text}
                  onChange={(e) => updateItem(item.id, e.target.value)}
                  className="flex-1 border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
                  placeholder="Item text"
                />
                <select
                  value={bucketForItem(item.id)}
                  onChange={(e) => assignItemToBucket(item.id, e.target.value)}
                  className="border border-border-strong rounded px-2 py-1 text-xs bg-surface-0 text-text-primary max-w-[110px]"
                >
                  <option value="">(unassigned)</option>
                  {buckets.map(b => (
                    <option key={b.id} value={b.id}>{b.label || '(unnamed)'}</option>
                  ))}
                </select>
                <button onClick={() => removeItem(item.id)} className="p-1 text-text-disabled hover:text-accent-danger" aria-label="Remove item" type="button">
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        </div>

        {/* Buckets */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <label className="text-xs font-medium text-text-secondary flex items-center gap-1">
              <Folder className="w-3.5 h-3.5" /> Buckets
            </label>
            <button onClick={addBucket} type="button" className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
              <Plus className="w-3 h-3" /> Add Bucket
            </button>
          </div>
          <div className="space-y-2">
            {buckets.length === 0 && (
              <p className="text-xs text-text-tertiary italic">No buckets yet.</p>
            )}
            {buckets.map(bucket => {
              const contained = (bucket.item_ids || [])
                .map(id => items.find(i => i.id === id))
                .filter(Boolean);
              return (
                <div key={bucket.id} className="border border-border-default rounded p-2 bg-surface-1">
                  <div className="flex items-center gap-2 mb-1">
                    <input
                      type="text"
                      value={bucket.label}
                      onChange={(e) => updateBucket(bucket.id, { label: e.target.value })}
                      className="flex-1 border border-border-strong rounded px-2 py-1 text-sm font-medium bg-surface-0 text-text-primary"
                      placeholder="Bucket label"
                    />
                    <button onClick={() => removeBucket(bucket.id)} className="p-1 text-text-disabled hover:text-accent-danger" aria-label="Remove bucket" type="button">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  {contained.length === 0 ? (
                    <p className="text-[11px] text-text-disabled italic">No items assigned</p>
                  ) : (
                    <ul className="text-xs text-text-secondary space-y-0.5 list-disc pl-4">
                      {contained.map(i => <li key={i.id} className="truncate">{i.text || '(empty)'}</li>)}
                    </ul>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
};

export default CategorizationEditor;
