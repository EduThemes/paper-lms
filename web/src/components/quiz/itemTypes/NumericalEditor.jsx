import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { makeId } from './types';

/**
 * Numerical answer editor (Canvas numerical_question).
 *
 * Each accepted answer is one of:
 *   exact_answer     { exact, margin }        margin: "0.5" (±) or "5%" of the answer
 *   range_answer     { start, end }           inclusive range
 *   precision_answer { approximate, precision } N significant digits must match
 *
 * Legacy Paper answers ({ text, margin }) are read as exact answers.
 */
export const NUMERICAL_KINDS = [
  { value: 'exact_answer', label: 'Exact answer' },
  { value: 'range_answer', label: 'Answer in range' },
  { value: 'precision_answer', label: 'Answer with precision' },
];

const str = (v) => (v === undefined || v === null ? '' : String(v));

export function normalizeNumericalAnswer(a) {
  const kind = a?.numerical_answer_type
    || (str(a?.start) !== '' && str(a?.end) !== '' && str(a?.exact) === '' && str(a?.text) === '' ? 'range_answer' : 'exact_answer');
  return {
    id: a?.id || makeId('n'),
    numerical_answer_type: kind,
    exact: str(a?.exact) !== '' ? str(a.exact) : str(a?.text),
    margin: str(a?.margin),
    start: str(a?.start),
    end: str(a?.end),
    approximate: str(a?.approximate),
    precision: str(a?.precision),
    weight: typeof a?.weight === 'number' && a.weight > 0 ? a.weight : 100,
  };
}

// Only the fields that matter for the chosen kind are stored.
export function serializeNumericalAnswer(a) {
  const base = { id: a.id, numerical_answer_type: a.numerical_answer_type, weight: a.weight || 100 };
  switch (a.numerical_answer_type) {
    case 'range_answer':
      return { ...base, start: a.start, end: a.end, text: `${a.start}..${a.end}` };
    case 'precision_answer':
      return { ...base, approximate: a.approximate, precision: a.precision, text: a.approximate };
    default:
      return { ...base, exact: a.exact, margin: a.margin, text: a.exact };
  }
}

// A short validation message for an answer, or '' when it is complete.
export function numericalAnswerProblem(a) {
  const isNum = (v) => v !== '' && !Number.isNaN(Number(v));
  switch (a.numerical_answer_type) {
    case 'range_answer':
      if (!isNum(a.start) || !isNum(a.end)) return 'Enter both ends of the range.';
      if (Number(a.start) > Number(a.end)) return 'The range start is above its end.';
      return '';
    case 'precision_answer':
      if (!isNum(a.approximate)) return 'Enter the approximate value.';
      if (!/^[1-9]\d*$/.test(String(a.precision))) return 'Precision is a whole number of significant digits (1 or more).';
      return '';
    default: {
      if (!isNum(a.exact)) return 'Enter the exact answer.';
      const m = String(a.margin || '').trim();
      if (m && !/^\d*\.?\d+%?$/.test(m)) return 'Margin is a number, or a percentage like 5%.';
      return '';
    }
  }
}

const inputCls = 'w-full border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 text-text-primary';

const NumericalEditor = ({ answers, onChange }) => {
  const list = (Array.isArray(answers) && answers.length > 0 ? answers : [{}]).map(normalizeNumericalAnswer);
  const emit = (next) => onChange(next.map(serializeNumericalAnswer));
  const patch = (i, p) => emit(list.map((a, idx) => (idx === i ? { ...a, ...p } : a)));

  return (
    <div className="space-y-3">
      <label className="block text-xs font-medium text-text-secondary">
        Accepted answers <span className="text-text-disabled">(a student matching any of them gets full credit)</span>
      </label>
      {list.map((a, i) => {
        const problem = numericalAnswerProblem(a);
        return (
          <div key={a.id} className="border border-border-default rounded p-3 space-y-2">
            <div className="flex items-center gap-2">
              <select
                value={a.numerical_answer_type}
                onChange={(e) => patch(i, { numerical_answer_type: e.target.value })}
                className="border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 text-text-primary"
                aria-label={`Answer ${i + 1} type`}
              >
                {NUMERICAL_KINDS.map(k => <option key={k.value} value={k.value}>{k.label}</option>)}
              </select>
              <div className="flex-1" />
              {list.length > 1 && (
                <button type="button" onClick={() => emit(list.filter((_, idx) => idx !== i))} className="p-1 text-text-disabled hover:text-accent-danger" aria-label={`Remove answer ${i + 1}`}>
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
            {a.numerical_answer_type === 'exact_answer' && (
              <div className="grid grid-cols-2 gap-2">
                <label className="text-xs text-text-tertiary">Answer
                  <input type="text" inputMode="decimal" value={a.exact} onChange={(e) => patch(i, { exact: e.target.value })} className={inputCls} aria-label={`Answer ${i + 1} exact value`} />
                </label>
                <label className="text-xs text-text-tertiary">Margin of error (± or %)
                  <input type="text" value={a.margin} onChange={(e) => patch(i, { margin: e.target.value })} className={inputCls} placeholder="0" aria-label={`Answer ${i + 1} margin`} />
                </label>
              </div>
            )}
            {a.numerical_answer_type === 'range_answer' && (
              <div className="grid grid-cols-2 gap-2">
                <label className="text-xs text-text-tertiary">From
                  <input type="text" inputMode="decimal" value={a.start} onChange={(e) => patch(i, { start: e.target.value })} className={inputCls} aria-label={`Answer ${i + 1} range start`} />
                </label>
                <label className="text-xs text-text-tertiary">To
                  <input type="text" inputMode="decimal" value={a.end} onChange={(e) => patch(i, { end: e.target.value })} className={inputCls} aria-label={`Answer ${i + 1} range end`} />
                </label>
              </div>
            )}
            {a.numerical_answer_type === 'precision_answer' && (
              <div className="grid grid-cols-2 gap-2">
                <label className="text-xs text-text-tertiary">Approximate value
                  <input type="text" inputMode="decimal" value={a.approximate} onChange={(e) => patch(i, { approximate: e.target.value })} className={inputCls} aria-label={`Answer ${i + 1} approximate value`} />
                </label>
                <label className="text-xs text-text-tertiary">Significant digits
                  <input type="number" min="1" value={a.precision} onChange={(e) => patch(i, { precision: e.target.value })} className={inputCls} aria-label={`Answer ${i + 1} precision`} />
                </label>
              </div>
            )}
            {problem && <p className="text-xs text-accent-warning" role="alert">{problem}</p>}
          </div>
        );
      })}
      <button type="button" onClick={() => emit([...list, normalizeNumericalAnswer({})])} className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
        <Plus className="w-3 h-3" /> Add another accepted answer
      </button>
    </div>
  );
};

export default NumericalEditor;
