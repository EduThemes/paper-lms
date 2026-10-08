import React, { useMemo } from 'react';
import { Plus, Trash2, AlertCircle } from 'lucide-react';
import { makeId } from './types';
import { extractBlankIds } from './MultipleDropdownEditor';

/**
 * Fill-in-multiple-blanks editor.
 *
 * The author writes [blank] tokens in the question text; each blank gets a
 * list of accepted answers (matched case-insensitively). Stored in the
 * Canvas array shape the grader reads: [{ id, blank_id, text, weight: 100 }],
 * one entry per accepted answer. The legacy map shape
 * { blank: ["a", "b"] } is read too and rewritten on the first edit.
 */
export function toBlankEntries(answers) {
  if (Array.isArray(answers)) return answers.filter(a => a && a.blank_id);
  if (answers && typeof answers === 'object') {
    const out = [];
    for (const [blank, accepted] of Object.entries(answers)) {
      for (const text of Array.isArray(accepted) ? accepted : []) {
        out.push({ id: makeId('b'), blank_id: blank, text, weight: 100 });
      }
    }
    return out;
  }
  return [];
}

const FillInMultipleBlanksEditor = ({ answers, onChange, questionText }) => {
  const entries = toBlankEntries(answers);
  const blankIds = useMemo(() => extractBlankIds(questionText), [questionText]);
  const orphanIds = [...new Set(entries.map(e => e.blank_id))].filter(id => !blankIds.includes(id));

  const update = (id, text) => onChange(entries.map(e => (e.id === id ? { ...e, text } : e)));
  const add = (blankId) => onChange([...entries, { id: makeId('b'), blank_id: blankId, text: '', weight: 100 }]);
  const remove = (id) => onChange(entries.filter(e => e.id !== id));
  const dropBlank = (blankId) => onChange(entries.filter(e => e.blank_id !== blankId));

  const renderBlank = (blankId, orphan = false) => {
    const rows = entries.filter(e => e.blank_id === blankId);
    return (
      <div key={blankId} className={`border rounded p-3 ${orphan ? 'border-accent-warning/50 bg-accent-warning/5' : 'border-border-default'}`}>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-mono font-semibold text-text-secondary">[{blankId}]</span>
          {orphan ? (
            <button type="button" onClick={() => dropBlank(blankId)} className="text-xs text-accent-danger hover:underline">
              Remove (not in question text)
            </button>
          ) : rows.length === 0 && (
            <span className="text-xs text-accent-warning flex items-center gap-1"><AlertCircle className="w-3 h-3" /> Needs an answer</span>
          )}
        </div>
        <div className="space-y-1.5">
          {rows.map((row, i) => (
            <div key={row.id} className="flex items-center gap-2">
              <input
                type="text"
                value={row.text}
                onChange={(e) => update(row.id, e.target.value)}
                className="flex-1 border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
                placeholder="Accepted answer"
                aria-label={`Accepted answer ${i + 1} for ${blankId}`}
              />
              <button type="button" onClick={() => remove(row.id)} className="p-1 text-text-disabled hover:text-accent-danger" aria-label={`Remove answer ${i + 1} for ${blankId}`}>
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
        {!orphan && (
          <button type="button" onClick={() => add(blankId)} className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
            <Plus className="w-3 h-3" /> Add accepted answer
          </button>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-3">
      <p className="text-xs text-text-tertiary">
        Type blanks in the question text as <code className="font-mono bg-surface-1 px-1 rounded">[blank_name]</code>,
        e.g. “Roses are [color1], violets are [color2].” Answers are not case-sensitive.
      </p>
      {blankIds.length === 0 && orphanIds.length === 0 && (
        <p className="text-xs text-accent-warning italic">No blanks yet — add a [token] to the question text.</p>
      )}
      {blankIds.map(id => renderBlank(id))}
      {orphanIds.map(id => renderBlank(id, true))}
    </div>
  );
};

export default FillInMultipleBlanksEditor;
