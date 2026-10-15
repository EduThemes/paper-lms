import React from 'react';
import { Plus, Trash2, AlertCircle } from 'lucide-react';
import { makeId, MATCH_POOL } from './types';

/**
 * Matching editor.
 *
 * Stored shape (what the grader, redaction and player read):
 *   pairs:       { id, left, right, right_id, weight: 100 }
 *   distractors: { id, text, blank_id: "_match_options_" }  (extra right-hand choices)
 *
 * Canvas imports store pairs as { id, text, match_id } plus the right-hand
 * pool as "_match_options_" rows; `toMatchingModel` reads both shapes.
 */
export function toMatchingModel(answers) {
  const list = Array.isArray(answers) ? answers : [];
  const poolText = {};
  for (const a of list) {
    if (a && a.blank_id === MATCH_POOL && a.id != null) poolText[a.id] = a.text ?? '';
  }
  const pairs = [];
  const used = new Set();
  for (const a of list) {
    if (!a || a.blank_id === MATCH_POOL) continue;
    const rightId = a.right_id || a.match_id || makeId('r');
    used.add(rightId);
    pairs.push({
      id: a.id || makeId('m'),
      left: a.left ?? a.text ?? '',
      right: a.right ?? poolText[rightId] ?? '',
      right_id: rightId,
    });
  }
  const distractors = Object.entries(poolText)
    .filter(([id]) => !used.has(id))
    .map(([id, text]) => ({ id, text }));
  return { pairs, distractors };
}

export function fromMatchingModel({ pairs, distractors }) {
  return [
    ...pairs.map(p => ({ id: p.id, left: p.left, right: p.right, right_id: p.right_id, weight: 100 })),
    ...distractors.map(d => ({ id: d.id, text: d.text, blank_id: MATCH_POOL })),
  ];
}

// Left labels identify a pair in a student's answer, so they must be unique.
export function duplicateLefts(pairs) {
  const seen = new Set();
  const dups = new Set();
  for (const p of pairs) {
    const key = (p.left || '').trim();
    if (!key) continue;
    if (seen.has(key)) dups.add(key);
    seen.add(key);
  }
  return [...dups];
}

const MatchingEditor = ({ answers, onChange }) => {
  const model = toMatchingModel(answers);
  const emit = (next) => onChange(fromMatchingModel(next));

  const setPair = (i, patch) => emit({ ...model, pairs: model.pairs.map((p, idx) => (idx === i ? { ...p, ...patch } : p)) });
  const addPair = () => emit({ ...model, pairs: [...model.pairs, { id: makeId('m'), left: '', right: '', right_id: makeId('r') }] });
  const removePair = (i) => emit({ ...model, pairs: model.pairs.filter((_, idx) => idx !== i) });
  const setDistractor = (i, text) => emit({ ...model, distractors: model.distractors.map((d, idx) => (idx === i ? { ...d, text } : d)) });
  const addDistractor = () => emit({ ...model, distractors: [...model.distractors, { id: makeId('r'), text: '' }] });
  const removeDistractor = (i) => emit({ ...model, distractors: model.distractors.filter((_, idx) => idx !== i) });

  const dups = duplicateLefts(model.pairs);

  return (
    <div className="space-y-3">
      <div>
        <div className="grid grid-cols-12 gap-2 text-[10px] uppercase tracking-wide text-text-tertiary px-1 mb-1">
          <div className="col-span-5">Prompt (left)</div>
          <div className="col-span-6">Correct match (right)</div>
          <div className="col-span-1" />
        </div>
        <div className="space-y-2">
          {model.pairs.map((p, i) => (
            <div key={p.id} className="grid grid-cols-12 gap-2 items-center">
              <input
                type="text"
                value={p.left}
                onChange={(e) => setPair(i, { left: e.target.value })}
                className="col-span-5 border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 text-text-primary"
                placeholder={`Prompt ${i + 1}`}
                aria-label={`Prompt ${i + 1}`}
              />
              <input
                type="text"
                value={p.right}
                onChange={(e) => setPair(i, { right: e.target.value })}
                className="col-span-6 border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 text-text-primary"
                placeholder={`Match ${i + 1}`}
                aria-label={`Match for prompt ${i + 1}`}
              />
              <button
                type="button"
                onClick={() => removePair(i)}
                disabled={model.pairs.length <= 1}
                className="col-span-1 p-1 text-text-tertiary hover:text-accent-danger disabled:opacity-30 flex justify-center"
                aria-label={`Remove pair ${i + 1}`}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
        <button type="button" onClick={addPair} className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
          <Plus className="w-3 h-3" /> Add Pair
        </button>
        {dups.length > 0 && (
          <p className="mt-2 text-xs text-accent-warning flex items-center gap-1" role="alert">
            <AlertCircle className="w-3.5 h-3.5" /> Each prompt must be unique: {dups.join(', ')}
          </p>
        )}
      </div>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">
          Distractors <span className="text-text-tertiary">(extra wrong matches shown in every dropdown)</span>
        </label>
        <div className="space-y-2">
          {model.distractors.map((d, i) => (
            <div key={d.id} className="flex items-center gap-2">
              <input
                type="text"
                value={d.text}
                onChange={(e) => setDistractor(i, e.target.value)}
                className="flex-1 border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 text-text-primary"
                placeholder="Distractor"
                aria-label={`Distractor ${i + 1}`}
              />
              <button type="button" onClick={() => removeDistractor(i)} className="p-1 text-text-tertiary hover:text-accent-danger" aria-label={`Remove distractor ${i + 1}`}>
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
        <button type="button" onClick={addDistractor} className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
          <Plus className="w-3 h-3" /> Add Distractor
        </button>
      </div>
    </div>
  );
};

export default MatchingEditor;
