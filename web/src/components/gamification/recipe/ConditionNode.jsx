import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import PredicateEditor, { PREDICATE_KINDS, emptyPredicate } from './PredicateEditor';

// ConditionNode — recursive renderer for a `condition_set` subtree.
//
// The tree shape mirrors what `predicates.DecodePredicate` accepts:
//
//   ConditionSet  := { kind:"ConditionSet", op:"AND"|"OR"|"N_OF_M",
//                      threshold?:int, children: Predicate[] }
//   Predicate     := ConditionSet | <one of 7 atomic predicate kinds>
//
// The root call is always a ConditionSet — RecipeEditor seeds it as
// `{ kind:"ConditionSet", op:"AND", children:[] }`, matching the
// Uncanny Automator pattern of "an empty rule fires on the trigger
// alone, no conditions." Nested ConditionSets render with extra
// left-padding so the AND/OR nesting is visually obvious without
// a graph layout.

export default function ConditionNode({ value, vocab, onChange, depth = 0 }) {
  const node = value || { kind: 'ConditionSet', op: 'AND', children: [] };

  const updateChild = (idx, next) => {
    const nextChildren = node.children.slice();
    nextChildren[idx] = next;
    onChange({ ...node, children: nextChildren });
  };

  const removeChild = (idx) => {
    const nextChildren = node.children.filter((_, i) => i !== idx);
    onChange({ ...node, children: nextChildren });
  };

  const addAtomic = (kind) => {
    onChange({ ...node, children: [...node.children, emptyPredicate(kind)] });
  };

  const addGroup = () => {
    onChange({
      ...node,
      children: [...node.children, { kind: 'ConditionSet', op: 'AND', children: [] }],
    });
  };

  const setOp = (op) => {
    const next = { ...node, op };
    // N_OF_M requires a positive threshold; seed it sensibly on
    // first switch so the form is in a valid state without the user
    // having to discover the hidden field.
    if (op === 'N_OF_M' && (!node.threshold || node.threshold <= 0)) {
      next.threshold = Math.max(1, node.children.length || 1);
    }
    if (op !== 'N_OF_M') {
      // Strip threshold for AND/OR — server-side validator ignores it,
      // but a clean JSON shape is easier to diff in audit logs.
      delete next.threshold;
    }
    onChange(next);
  };

  const setOps = vocab?.set_ops || ['AND', 'OR', 'N_OF_M'];

  return (
    <div
      className={`rounded-md border border-surface-raised bg-surface-1 p-2.5 space-y-2 ${
        depth > 0 ? 'ml-3' : ''
      }`}
    >
      <header className="flex flex-wrap items-center gap-1.5">
        <span className="text-[11px] font-medium uppercase tracking-wide text-text-tertiary">
          Match
        </span>
        <div role="radiogroup" aria-label="Set operator" className="flex gap-1">
          {setOps.map((op) => {
            const active = node.op === op;
            return (
              <button
                type="button"
                key={op}
                role="radio"
                aria-checked={active}
                onClick={() => setOp(op)}
                className={`px-2 py-0.5 rounded-md text-[11px] border ${
                  active
                    ? 'bg-brand-400/10 border-brand-400 text-text-primary'
                    : 'bg-surface-0 border-surface-raised text-text-secondary hover:bg-surface-2'
                }`}
              >
                {op.replace(/_/g, ' ')}
              </button>
            );
          })}
        </div>
        {node.op === 'N_OF_M' && (
          <label className="flex items-center gap-1 text-[11px] text-text-secondary">
            ≥
            <input
              type="number"
              min="1"
              value={node.threshold ?? 1}
              onChange={(e) => onChange({ ...node, threshold: Number(e.target.value) || 1 })}
              className="w-12 px-1.5 py-0.5 rounded border border-surface-raised bg-surface-0 text-xs text-text-primary"
              aria-label="N of M threshold"
            />
            of
          </label>
        )}
        <span className="text-[11px] text-text-tertiary">
          {node.children.length || 'no'} {node.children.length === 1 ? 'condition' : 'conditions'}
        </span>
      </header>

      {node.children.length > 0 && (
        <ul className="space-y-2 list-none m-0 p-0">
          {node.children.map((child, idx) => (
            <li key={idx} className="flex items-start gap-1.5">
              <div className="flex-1 min-w-0">
                {child.kind === 'ConditionSet' ? (
                  <ConditionNode
                    value={child}
                    vocab={vocab}
                    onChange={(next) => updateChild(idx, next)}
                    depth={depth + 1}
                  />
                ) : (
                  <div className="rounded-md border border-surface-raised bg-surface-0 p-2 space-y-1.5">
                    <div className="text-[11px] font-medium text-text-tertiary">{child.kind}</div>
                    <PredicateEditor
                      value={child}
                      vocab={vocab}
                      onChange={(next) => updateChild(idx, next)}
                    />
                  </div>
                )}
              </div>
              <button
                type="button"
                onClick={() => removeChild(idx)}
                aria-label={`Remove condition ${idx + 1}`}
                className="mt-1 p-1 rounded-md text-text-tertiary hover:bg-surface-2 hover:text-accent-danger"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}

      <footer className="flex flex-wrap items-center gap-2 pt-1">
        <AddConditionMenu onAdd={addAtomic} />
        <button
          type="button"
          onClick={addGroup}
          className="inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs text-text-secondary border border-dashed border-surface-raised hover:bg-surface-2"
        >
          <Plus className="w-3 h-3" /> Group
        </button>
      </footer>
    </div>
  );
}

// AddConditionMenu — native <select> rather than Radix dropdown to
// keep the bundle small and the keyboard story painless. The empty
// "Add condition…" placeholder triggers the add on change and then
// resets so the same option can be picked again.
function AddConditionMenu({ onAdd }) {
  return (
    <label className="inline-flex items-center gap-1">
      <span className="sr-only">Add condition</span>
      <select
        value=""
        onChange={(e) => {
          if (e.target.value) {
            onAdd(e.target.value);
            e.target.value = '';
          }
        }}
        className="px-2 py-1 rounded-md text-xs border border-dashed border-surface-raised bg-surface-0 text-text-secondary hover:bg-surface-2"
        aria-label="Add condition"
      >
        <option value="">+ Condition…</option>
        {PREDICATE_KINDS.map((k) => (
          <option key={k} value={k}>{k}</option>
        ))}
      </select>
    </label>
  );
}
