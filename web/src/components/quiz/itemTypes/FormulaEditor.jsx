import React, { useState } from 'react';
import { Plus, Trash2, Play } from 'lucide-react';
import { makeId } from './types';
import { quizItemsApi } from '../../../services/api_quiz_items';

/**
 * Formula (calculated) question editor. Stores config in a single answer entry:
 *   { formula, variables: [{name, min, max, precision}], tolerance,
 *     answer_precision, answer_value }
 *
 * Each student's attempt gets its own random value for every variable
 * (between min and max, rounded to `precision` decimals); [name] tokens in
 * the question text are replaced with those values and the answer is
 * computed from the formula. `tolerance` is an absolute margin ("0.5") or
 * a percentage of the answer ("5%"). `answer_value` is only used when
 * there is no formula (a fixed expected value).
 */
export const FORMULA_NAME_RE = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function formulaProblems(cfg) {
  const problems = [];
  const names = new Set();
  for (const v of cfg.variables || []) {
    const name = (v.name || '').trim();
    if (!FORMULA_NAME_RE.test(name)) problems.push(`"${name || '(blank)'}" is not a valid variable name.`);
    else if (names.has(name)) problems.push(`Variable "${name}" is defined twice.`);
    names.add(name);
    if (Number(v.min) > Number(v.max)) problems.push(`Variable "${name}": min is above max.`);
  }
  const tol = String(cfg.tolerance ?? '').trim();
  if (tol && !/^\d*\.?\d+%?$/.test(tol)) problems.push('Tolerance is a number, or a percentage like 5%.');
  if (!String(cfg.formula || '').trim() && !String(cfg.answer_value || '').trim()) {
    problems.push('Enter a formula (or a fixed expected value).');
  }
  return problems;
}

const FormulaEditor = ({ answers, onChange, courseId, quizId }) => {
  const list = Array.isArray(answers) && answers.length > 0 ? answers : [{
    id: makeId('a'), text: '', weight: 100, formula: '', variables: [], tolerance: '', answer_value: '',
  }];
  const cfg = list[0];
  const [samples, setSamples] = useState(null);
  const [previewError, setPreviewError] = useState('');
  const [previewing, setPreviewing] = useState(false);

  const patch = (p) => { setSamples(null); onChange([{ ...cfg, ...p }]); };
  const setVar = (i, p) => {
    const vars = [...(cfg.variables || [])];
    vars[i] = { ...vars[i], ...p };
    patch({ variables: vars });
  };
  const addVar = () => patch({ variables: [...(cfg.variables || []), { name: '', min: 0, max: 10, precision: 0 }] });
  const removeVar = (i) => patch({ variables: (cfg.variables || []).filter((_, idx) => idx !== i) });

  const runPreview = async () => {
    setPreviewing(true);
    setPreviewError('');
    try {
      setSamples(await quizItemsApi.formulaPreview(courseId, quizId, [cfg], 5));
    } catch (e) {
      setSamples(null);
      setPreviewError(e?.message || 'Could not evaluate the formula');
    } finally {
      setPreviewing(false);
    }
  };

  const problems = formulaProblems(cfg);
  const numInput = 'border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary';

  return (
    <div className="space-y-3">
      <p className="text-xs text-text-tertiary">
        Put variables in the question text as <code className="font-mono bg-surface-1 px-1 rounded">[x]</code>.
        Each student sees their own values. Functions: sqrt, abs, sin, cos, tan, ln, log, exp, round, floor, ceil, min, max, pow; constants pi, e; ^ for powers.
      </p>
      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1" htmlFor={`formula-${cfg.id}`}>
          Formula
        </label>
        <input
          id={`formula-${cfg.id}`}
          type="text"
          value={cfg.formula || ''}
          onChange={(e) => patch({ formula: e.target.value })}
          className="w-full border border-border-strong rounded px-3 py-2 text-sm font-mono bg-surface-0 text-text-primary"
          placeholder="x * 2 + y"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="block text-xs font-medium text-text-secondary">Variables</span>
          <button onClick={addVar} type="button" className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
            <Plus className="w-3 h-3" /> Add Variable
          </button>
        </div>
        {(cfg.variables || []).length === 0 ? (
          <p className="text-xs text-text-tertiary italic">No variables yet. Each variable gets a random value per student.</p>
        ) : (
          <div className="space-y-2">
            <div className="grid grid-cols-12 gap-2 text-[10px] uppercase tracking-wide text-text-tertiary px-1">
              <div className="col-span-3">Name</div>
              <div className="col-span-3">Min</div>
              <div className="col-span-3">Max</div>
              <div className="col-span-2">Decimals</div>
              <div className="col-span-1" />
            </div>
            {(cfg.variables || []).map((v, i) => (
              <div key={i} className="grid grid-cols-12 gap-2 items-center">
                <input type="text" value={v.name} onChange={(e) => setVar(i, { name: e.target.value })}
                  className={`col-span-3 font-mono ${numInput}`} placeholder="x" aria-label={`Variable ${i + 1} name`} />
                <input type="number" step="any" value={v.min} onChange={(e) => setVar(i, { min: parseFloat(e.target.value) || 0 })}
                  className={`col-span-3 ${numInput}`} aria-label={`Variable ${i + 1} minimum`} />
                <input type="number" step="any" value={v.max} onChange={(e) => setVar(i, { max: parseFloat(e.target.value) || 0 })}
                  className={`col-span-3 ${numInput}`} aria-label={`Variable ${i + 1} maximum`} />
                <input type="number" min="0" max="10" value={v.precision ?? 0} onChange={(e) => setVar(i, { precision: parseInt(e.target.value, 10) || 0 })}
                  className={`col-span-2 ${numInput}`} aria-label={`Variable ${i + 1} decimals`} />
                <button onClick={() => removeVar(i)} type="button"
                  className="col-span-1 p-1 text-text-disabled hover:text-accent-danger flex items-center justify-center" aria-label={`Remove variable ${i + 1}`}>
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-3 gap-3">
        <label className="block text-xs font-medium text-text-secondary">Tolerance
          <input type="text" value={cfg.tolerance ?? ''} onChange={(e) => patch({ tolerance: e.target.value })}
            className={`w-full mt-1 ${numInput}`} placeholder="0 or 5%" aria-label="Tolerance" />
        </label>
        <label className="block text-xs font-medium text-text-secondary">Answer decimals
          <input type="number" min="0" max="10" value={cfg.answer_precision ?? ''}
            onChange={(e) => patch({ answer_precision: e.target.value === '' ? undefined : parseInt(e.target.value, 10) || 0 })}
            className={`w-full mt-1 ${numInput}`} placeholder="unrounded" aria-label="Answer decimals" />
        </label>
        <label className="block text-xs font-medium text-text-secondary">Fixed value <span className="text-text-disabled">(no formula)</span>
          <input type="text" value={cfg.answer_value || ''} onChange={(e) => patch({ answer_value: e.target.value })}
            className={`w-full mt-1 font-mono ${numInput}`} placeholder="e.g. 42" aria-label="Fixed expected value" />
        </label>
      </div>

      {problems.length > 0 && (
        <ul className="text-xs text-accent-warning list-disc pl-5" role="alert">
          {problems.map(p => <li key={p}>{p}</li>)}
        </ul>
      )}

      {courseId && quizId && String(cfg.formula || '').trim() && (
        <div>
          <button type="button" onClick={runPreview} disabled={previewing || problems.length > 0}
            className="inline-flex items-center gap-1 text-xs px-2 py-1 rounded border border-border-strong hover:bg-surface-1 disabled:opacity-50">
            <Play className="w-3 h-3" /> {previewing ? 'Generating…' : 'Preview sample values'}
          </button>
          {previewError && <p className="text-xs text-accent-danger mt-1" role="alert">{previewError}</p>}
          {samples && samples.length > 0 && (
            <table className="mt-2 text-xs border border-border-default rounded w-full">
              <thead className="bg-surface-1 text-text-tertiary">
                <tr>
                  {(samples[0].variables || []).map(v => <th key={v.name} className="px-2 py-1 text-left font-mono">{v.name}</th>)}
                  <th className="px-2 py-1 text-left">Answer</th>
                </tr>
              </thead>
              <tbody>
                {samples.map((s, i) => (
                  <tr key={i} className="border-t border-border-default">
                    {(s.variables || []).map(v => <td key={v.name} className="px-2 py-1 font-mono">{v.value}</td>)}
                    <td className="px-2 py-1 font-mono font-semibold">{s.expected}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
};

export default FormulaEditor;
