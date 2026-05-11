import React from 'react';
import { Plus, Trash2, CheckSquare, Square } from 'lucide-react';
import { makeId } from './types';

/**
 * Checkbox-style answer editor. Any number of answers can be marked correct.
 * Each correct answer carries weight=100; incorrect carry weight=0.
 */
const MultipleAnswerEditor = ({ answers, onChange }) => {
  const list = Array.isArray(answers) ? answers : [];

  const toggleCorrect = (i) => {
    const next = list.map((a, idx) => idx === i ? { ...a, weight: a.weight > 0 ? 0 : 100 } : a);
    onChange(next);
  };
  const updateText = (i, text) => {
    onChange(list.map((a, idx) => idx === i ? { ...a, text } : a));
  };
  const addAnswer = () => onChange([...list, { id: makeId('a'), text: '', weight: 0 }]);
  const removeAnswer = (i) => {
    if (list.length <= 2) return;
    onChange(list.filter((_, idx) => idx !== i));
  };

  return (
    <div>
      <label className="block text-xs font-medium text-text-secondary mb-2">
        Answers <span className="text-text-disabled">(check box to mark correct — any number)</span>
      </label>
      <div className="space-y-2">
        {list.map((a, i) => {
          const correct = a.weight > 0;
          return (
            <div key={a.id || i} className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => toggleCorrect(i)}
                className={`w-6 h-6 rounded border-2 flex items-center justify-center flex-shrink-0 transition-colors ${
                  correct
                    ? 'border-accent-success bg-accent-success text-white'
                    : 'border-border-strong hover:border-accent-success/60'
                }`}
                aria-pressed={correct}
                title={correct ? 'Correct answer' : 'Mark as correct'}
              >
                {correct ? <CheckSquare className="w-3.5 h-3.5" /> : <Square className="w-3.5 h-3.5 opacity-0" />}
              </button>
              <input
                type="text"
                value={a.text}
                onChange={(e) => updateText(i, e.target.value)}
                className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
                placeholder={`Answer ${i + 1}`}
              />
              {list.length > 2 && (
                <button
                  onClick={() => removeAnswer(i)}
                  className="p-1 text-text-disabled hover:text-accent-danger"
                  aria-label={`Remove answer ${i + 1}`}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
          );
        })}
      </div>
      <button
        onClick={addAnswer}
        className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
        type="button"
      >
        <Plus className="w-3 h-3" /> Add Answer
      </button>
    </div>
  );
};

export default MultipleAnswerEditor;
