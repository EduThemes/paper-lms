import React from 'react';
import { DndContext, closestCenter, PointerSensor, KeyboardSensor, useSensor, useSensors } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Plus, Trash2 } from 'lucide-react';
import { CurrencyPicker, BadgePicker } from './EntityPickers';

// EffectsPalette — drag-to-reorder list of effects on a recipe.
//
// Effect order is semantically meaningful: the dispatcher fires effects
// in declaration order with stop-on-first-error semantics. The drag
// handle is the only way to express that ordering, so it has to feel
// solid — pointer + keyboard sensors, ARIA-labeled handle, visible
// position number.
//
// Per-effect editor lives in EffectRow below; the switch on `kind`
// follows the same pattern as PredicateEditor so new effect kinds
// register by adding one case here + one entry in EffectCatalog
// server-side.

const EFFECT_KINDS = ['AwardCurrency', 'AwardBadge'];

// Stable per-row identity is required by @dnd-kit. Effects in the
// model don't carry an id (the JSON shape the backend stores is
// position-indexed), so we synthesize one on first render and strip
// it before save. _dragId never leaves this component.
function withDragIds(effects) {
  return effects.map((e, i) => ({ ...e, _dragId: e._dragId || `eff-${i}-${Math.random().toString(36).slice(2, 9)}` }));
}
function stripDragIds(effects) {
  return effects.map(({ _dragId, ...rest }) => rest);
}

export default function EffectsPalette({ value, onChange }) {
  const list = withDragIds(value || []);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const propagate = (next) => onChange(stripDragIds(next));

  const handleDragEnd = (event) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIdx = list.findIndex((e) => e._dragId === active.id);
    const newIdx = list.findIndex((e) => e._dragId === over.id);
    propagate(arrayMove(list, oldIdx, newIdx));
  };

  const updateAt = (idx, next) => {
    const copy = list.slice();
    copy[idx] = { ...next, _dragId: list[idx]._dragId };
    propagate(copy);
  };
  const removeAt = (idx) => propagate(list.filter((_, i) => i !== idx));
  const addEffect = (kind) => propagate([...list, emptyEffect(kind)]);

  return (
    <div className="space-y-2">
      {list.length === 0 ? (
        <div className="rounded-md border border-dashed border-surface-raised bg-surface-1 p-3 text-xs text-text-tertiary">
          No effects yet. Add an effect for this rule to grant anything when it fires.
        </div>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={list.map((e) => e._dragId)} strategy={verticalListSortingStrategy}>
            <ol className="space-y-2 list-none m-0 p-0">
              {list.map((effect, idx) => (
                <SortableEffectRow
                  key={effect._dragId}
                  effect={effect}
                  index={idx}
                  onChange={(next) => updateAt(idx, next)}
                  onRemove={() => removeAt(idx)}
                />
              ))}
            </ol>
          </SortableContext>
        </DndContext>
      )}

      <AddEffectMenu onAdd={addEffect} />
    </div>
  );
}

function SortableEffectRow({ effect, index, onChange, onRemove }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: effect._dragId });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <li
      ref={setNodeRef}
      style={style}
      className="rounded-md border border-surface-raised bg-surface-1 p-2.5 flex items-start gap-2"
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="mt-1 cursor-grab active:cursor-grabbing text-text-tertiary hover:text-text-secondary touch-none"
        aria-label={`Drag handle for effect ${index + 1}`}
      >
        <GripVertical className="w-4 h-4" />
      </button>
      <span className="mt-1 text-[11px] font-mono text-text-tertiary w-5 text-center">{index + 1}.</span>
      <div className="flex-1 min-w-0 space-y-1.5">
        <div className="text-[11px] font-medium text-text-tertiary">{effect.kind}</div>
        <EffectFields effect={effect} onChange={onChange} />
      </div>
      <button
        type="button"
        onClick={onRemove}
        aria-label={`Remove effect ${index + 1}`}
        className="mt-1 p-1 rounded-md text-text-tertiary hover:bg-surface-2 hover:text-accent-danger"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
    </li>
  );
}

// EffectFields — switch on effect kind, render its inline editor.
// Same UX shape as PredicateEditor: one tiny field group per kind.
function EffectFields({ effect, onChange }) {
  switch (effect.kind) {
    case 'AwardCurrency':
      return <AwardCurrencyFields value={effect} onChange={onChange} />;
    case 'AwardBadge':
      return <AwardBadgeFields value={effect} onChange={onChange} />;
    default:
      return (
        <div className="rounded-md border border-accent-warning/40 bg-accent-warning/10 px-2.5 py-1.5 text-xs text-text-secondary">
          Unknown effect kind <code className="font-mono">{effect.kind || '?'}</code>.
        </div>
      );
  }
}

const inputCls =
  'px-2.5 py-1.5 rounded-md border border-surface-raised bg-surface-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60';

function AwardCurrencyFields({ value, onChange }) {
  return (
    <div className="grid grid-cols-3 gap-2">
      <CurrencyPicker
        value={value.code}
        onChange={(code) => onChange({ ...value, code })}
        required
      />
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium text-text-secondary">Amount <span className="text-accent-danger">*</span></span>
        <input
          type="number"
          min="1"
          step="1"
          value={value.amount ?? ''}
          onChange={(e) => {
            const raw = e.target.value;
            onChange({ ...value, amount: raw === '' ? undefined : Number(raw) });
          }}
          className={inputCls}
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium text-text-secondary">Multiplier</span>
        <input
          type="number"
          step="any"
          value={value.multiplier ?? ''}
          placeholder="1.0"
          onChange={(e) => {
            const raw = e.target.value;
            onChange({ ...value, multiplier: raw === '' ? undefined : Number(raw) });
          }}
          className={inputCls}
        />
      </label>
    </div>
  );
}

function AwardBadgeFields({ value, onChange }) {
  return (
    <div className="grid grid-cols-2 gap-2">
      <BadgePicker
        value={value.code}
        onChange={(code) => onChange({ ...value, code })}
        required
      />
      <label className="flex flex-col gap-1">
        <span className="text-xs font-medium text-text-secondary">Evidence (optional)</span>
        <input
          type="text"
          value={value.evidence ?? ''}
          onChange={(e) => onChange({ ...value, evidence: e.target.value })}
          className={inputCls}
        />
      </label>
    </div>
  );
}

function AddEffectMenu({ onAdd }) {
  return (
    <label className="inline-flex items-center gap-1">
      <span className="sr-only">Add effect</span>
      <select
        value=""
        onChange={(e) => {
          if (e.target.value) {
            onAdd(e.target.value);
            e.target.value = '';
          }
        }}
        className="px-2 py-1 rounded-md text-xs border border-dashed border-surface-raised bg-surface-0 text-text-secondary hover:bg-surface-2"
        aria-label="Add effect"
      >
        <option value="">+ Effect…</option>
        {EFFECT_KINDS.map((k) => (
          <option key={k} value={k}>{k}</option>
        ))}
      </select>
    </label>
  );
}

function emptyEffect(kind) {
  // Seed `amount: 1` so AwardCurrency starts with a saveable shape
  // — the server requires amount > 0. AwardBadge has no required
  // numeric defaults.
  if (kind === 'AwardCurrency') return { kind, amount: 1 };
  return { kind };
}
