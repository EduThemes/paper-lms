import React from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { makeId } from './types';

/**
 * Formula editor. Stores config in a single answer entry:
 * { formula, variables: [{name, min, max, precision}], tolerance, answer_value }
 */
const FormulaEditor = ({ answers, onChange }) => {
  const list = Array.isArray(answers) && answers.length > 0 ? answers : [{
    id: makeId('a'), text: '', weight: 100, formula: '', variables: [], tolerance: 0, answer_value: '',
  }];
  const cfg = list[0];

  const patch = (p) => onChange([{ ...cfg, ...p }]);
  const setVar = (i, p) => {
    const vars = [...(cfg.variables || [])];
    vars[i] = { ...vars[i], ...p };
    patch({ variables: vars });
  };
  const addVar = () => {
    patch({
      variables: [...(cfg.variables || []), { name: '', min: 0, max: 10, precision: 0 }],
    });
  };
  const removeVar = (i) => {
    patch({ variables: (cfg.variables || []).filter((_, idx) => idx !== i) });
  };

  return (
    <div className="space-y-3">
      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">
          Formula expression
          <span className="text-text-disabled ml-1">(e.g. <code className="font-mono bg-surface-1 px-1 rounded">x * 2 + y</code>)</span>
        </label>
        <input
          type="text"
          value={cfg.formula || ''}
          onChange={(e) => patch({ formula: e.target.value })}
          className="w-full border border-border-strong rounded px-3 py-2 text-sm font-mono bg-surface-0 text-text-primary"
          placeholder="x * 2 + y"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="block text-xs font-medium text-text-secondary">Variables</label>
          <button
            onClick={addVar}
            type="button"
            className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
          >
            <Plus className="w-3 h-3" /> Add Variable
          </button>
        </div>
        {(cfg.variables || []).length === 0 ? (
          <p className="text-xs text-text-tertiary italic">No variables yet. Each variable gets randomized per student.</p>
        ) : (
          <div className="space-y-2">
            <div className="grid grid-cols-12 gap-2 text-[10px] uppercase tracking-wide text-text-tertiary px-1">
              <div className="col-span-3">Name</div>
              <div className="col-span-3">Min</div>
              <div className="col-span-3">Max</div>
              <div className="col-span-2">Precision</div>
              <div className="col-span-1"></div>
            </div>
            {(cfg.variables || []).map((v, i) => (
              <div key={i} className="grid grid-cols-12 gap-2 items-center">
                <input
                  type="text"
                  value={v.name}
                  onChange={(e) => setVar(i, { name: e.target.value })}
                  className="col-span-3 border border-border-strong rounded px-2 py-1 text-sm font-mono bg-surface-0 text-text-primary"
                  placeholder="x"
                />
                <input
                  type="number"
                  value={v.min}
                  onChange={(e) => setVar(i, { min: parseFloat(e.target.value) || 0 })}
                  className="col-span-3 border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
                />
                <input
                  type="number"
                  value={v.max}
                  onChange={(e) => setVar(i, { max: parseFloat(e.target.value) || 0 })}
                  className="col-span-3 border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
                />
                <input
                  type="number"
                  min="0"
                  value={v.precision}
                  onChange={(e) => setVar(i, { precision: parseInt(e.target.value, 10) || 0 })}
                  className="col-span-2 border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
                />
                <button
                  onClick={() => removeVar(i)}
                  className="col-span-1 p-1 text-text-disabled hover:text-accent-danger flex items-center justify-center"
                  aria-label={`Remove variable ${i + 1}`}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-text-secondary mb-1">Tolerance</label>
          <input
            type="number"
            step="any"
            value={cfg.tolerance ?? 0}
            onChange={(e) => patch({ tolerance: parseFloat(e.target.value) || 0 })}
            className="w-full border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
            placeholder="0"
          />
          <p className="text-[10px] text-text-disabled mt-0.5">± allowed deviation from expected value</p>
        </div>
        <div>
          <label className="block text-xs font-medium text-text-secondary mb-1">
            Expected value <span className="text-text-disabled">(optional, no-variable mode)</span>
          </label>
          <input
            type="text"
            value={cfg.answer_value || ''}
            onChange={(e) => patch({ answer_value: e.target.value })}
            className="w-full border border-border-strong rounded px-3 py-1.5 text-sm font-mono bg-surface-0 text-text-primary"
            placeholder="e.g. 42"
          />
        </div>
      </div>
    </div>
  );
};

export default FormulaEditor;
