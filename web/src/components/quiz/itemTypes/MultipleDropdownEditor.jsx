import React, { useMemo } from 'react';
import { Plus, Trash2, Check } from 'lucide-react';
import { makeId } from './types';

/**
 * Multi-dropdown editor.
 *
 * Author writes [blank_id] placeholders in the question text; this editor
 * shows one option-set per blank. Each option carries `blank_id`, `text`,
 * and `weight` (100 for the correct value).
 *
 * We extract blank ids from the question text on the fly and let the author
 * add options for each. Orphaned options (whose blank_id is no longer in the
 * text) are surfaced so the author can clean up.
 */
const BLANK_RE = /\[([a-zA-Z][\w-]*)\]/g;

export function extractBlankIds(text) {
  if (!text) return [];
  const ids = [];
  const seen = new Set();
  // Strip HTML tags first so we only match the rendered text.
  const plain = String(text).replace(/<[^>]*>/g, ' ');
  let m;
  while ((m = BLANK_RE.exec(plain)) !== null) {
    if (!seen.has(m[1])) {
      ids.push(m[1]);
      seen.add(m[1]);
    }
  }
  return ids;
}

const MultipleDropdownEditor = ({ answers, onChange, questionText }) => {
  const list = Array.isArray(answers) ? answers : [];
  const blankIds = useMemo(() => extractBlankIds(questionText), [questionText]);

  const updateOption = (idx, patch) => {
    onChange(list.map((a, i) => i === idx ? { ...a, ...patch } : a));
  };

  const addOption = (blankId) => {
    const hasCorrect = list.some(a => a.blank_id === blankId && a.weight > 0);
    onChange([
      ...list,
      { id: makeId('a'), blank_id: blankId, text: '', weight: hasCorrect ? 0 : 100 },
    ]);
  };

  const removeOption = (idx) => {
    const target = list[idx];
    const remaining = list.filter((_, i) => i !== idx);
    // If we removed the only correct answer for this blank, promote the first
    // remaining option for that blank so submissions stay gradable.
    const blankOpts = remaining.filter(a => a.blank_id === target.blank_id);
    if (target.weight > 0 && blankOpts.length > 0 && !blankOpts.some(a => a.weight > 0)) {
      blankOpts[0].weight = 100;
    }
    onChange(remaining);
  };

  const setCorrect = (blankId, optionIdx) => {
    onChange(list.map((a) => {
      if (a.blank_id !== blankId) return a;
      return { ...a, weight: list.indexOf(a) === optionIdx ? 100 : 0 };
    }));
  };

  // Group options by blank_id; preserve insertion order.
  const grouped = blankIds.map(bid => ({
    blank_id: bid,
    options: list
      .map((a, i) => ({ ...a, _idx: i }))
      .filter(a => a.blank_id === bid),
  }));

  const orphaned = list
    .map((a, i) => ({ ...a, _idx: i }))
    .filter(a => !blankIds.includes(a.blank_id));

  return (
    <div className="space-y-3">
      <p className="text-xs text-text-tertiary italic">
        Use <code className="font-mono bg-surface-1 px-1 rounded">[blank_id]</code> placeholders in the question text
        (e.g. <code className="font-mono bg-surface-1 px-1 rounded">[color]</code>) — one option set will appear per blank.
      </p>
      {blankIds.length === 0 && (
        <div className="text-xs text-accent-warning bg-accent-warning/10 border border-accent-warning/30 rounded p-2">
          No blank placeholders detected yet. Add e.g. <code>[blank1]</code> to your question text.
        </div>
      )}
      {grouped.map(({ blank_id, options }) => (
        <div key={blank_id} className="border border-border-default rounded p-3 bg-surface-1">
          <div className="text-xs font-medium text-text-secondary mb-2">
            Blank: <code className="font-mono">[{blank_id}]</code>
            <span className="text-text-disabled font-normal ml-2">(click circle to mark correct)</span>
          </div>
          <div className="space-y-2">
            {options.map((opt) => {
              const correct = opt.weight > 0;
              return (
                <div key={opt.id} className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setCorrect(blank_id, opt._idx)}
                    className={`w-6 h-6 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${
                      correct
                        ? 'border-accent-success bg-accent-success text-white'
                        : 'border-border-strong hover:border-accent-success/60'
                    }`}
                    aria-pressed={correct}
                  >
                    {correct && <Check className="w-3 h-3" />}
                  </button>
                  <input
                    type="text"
                    value={opt.text}
                    onChange={(e) => updateOption(opt._idx, { text: e.target.value })}
                    className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
                    placeholder="Option text"
                  />
                  <button
                    onClick={() => removeOption(opt._idx)}
                    className="p-1 text-text-disabled hover:text-accent-danger"
                    aria-label="Remove option"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              );
            })}
          </div>
          <button
            onClick={() => addOption(blank_id)}
            className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
            type="button"
          >
            <Plus className="w-3 h-3" /> Add Option
          </button>
        </div>
      ))}
      {orphaned.length > 0 && (
        <div className="border border-accent-warning/30 bg-accent-warning/10 rounded p-3">
          <div className="text-xs font-medium text-accent-warning mb-2">
            Orphaned options (blank no longer in question text):
          </div>
          {orphaned.map(opt => (
            <div key={opt.id} className="flex items-center gap-2 text-xs">
              <code className="font-mono">[{opt.blank_id}]</code>
              <span className="text-text-secondary truncate">{opt.text || '(empty)'}</span>
              <button
                onClick={() => removeOption(opt._idx)}
                className="text-accent-danger hover:underline"
              >
                Remove
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default MultipleDropdownEditor;
