import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { makeId } from './types';

/**
 * Fill-in-the-blank editor. Author supplies a list of accepted strings;
 * grading is case-insensitive (note shown to author).
 */
const FillInBlankEditor = ({ answers, onChange }) => {
  const list = Array.isArray(answers) ? answers : [];

  const updateText = (i, text) => {
    onChange(list.map((a, idx) => idx === i ? { ...a, text } : a));
  };
  const addAnswer = () => onChange([...list, { id: makeId('a'), text: '', weight: 100 }]);
  const removeAnswer = (i) => {
    if (list.length <= 1) return;
    onChange(list.filter((_, idx) => idx !== i));
  };

  return (
    <div>
      <label className="block text-xs font-medium text-text-secondary mb-2">
        Accepted Answers
        <span className="text-text-disabled ml-1">(grading is case-insensitive; whitespace trimmed)</span>
      </label>
      <div className="space-y-2">
        {list.map((a, i) => (
          <div key={a.id || i} className="flex items-center gap-2">
            <input
              type="text"
              value={a.text || ''}
              onChange={(e) => updateText(i, e.target.value)}
              className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
              placeholder="Accepted answer..."
            />
            {list.length > 1 && (
              <button
                onClick={() => removeAnswer(i)}
                className="p-1 text-text-disabled hover:text-accent-danger"
                aria-label={`Remove answer ${i + 1}`}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ))}
      </div>
      <button
        onClick={addAnswer}
        className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
        type="button"
      >
        <Plus className="w-3 h-3" /> Add Accepted Answer
      </button>
    </div>
  );
};

export default FillInBlankEditor;
