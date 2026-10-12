import React, { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { CALCULATION_METHODS, methodInfo, intForMethod, validateOutcomeForm } from '../../utils/outcomeCalculation';

const parseRatings = (ratings) => {
  if (Array.isArray(ratings)) return ratings;
  try {
    const parsed = JSON.parse(ratings || '[]');
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

/**
 * Edit form for an existing outcome (G151): title, display name,
 * description, mastery threshold, calculation method + parameter, and the
 * rating scale. onSave receives the PUT payload; it may throw (the message
 * is shown).
 */
export default function OutcomeEditForm({ outcome, onSave, onCancel }) {
  const [title, setTitle] = useState(outcome.title || '');
  const [displayName, setDisplayName] = useState(outcome.display_name || '');
  const [description, setDescription] = useState(outcome.description || '');
  const [masteryPoints, setMasteryPoints] = useState(outcome.mastery_points ?? 3);
  const [method, setMethod] = useState(outcome.calculation_method === 'weighted_average' ? 'decaying_average' : (outcome.calculation_method || 'decaying_average'));
  const [calcInt, setCalcInt] = useState(intForMethod(outcome.calculation_method || 'decaying_average', outcome.calculation_int) ?? '');
  const [ratings, setRatings] = useState(() => parseRatings(outcome.ratings).map((r) => ({ description: r.description || '', points: r.points ?? 0 })));
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const info = methodInfo(method);

  const changeMethod = (m) => {
    setMethod(m);
    setCalcInt(intForMethod(m, calcInt) ?? '');
  };

  const submit = async (e) => {
    e.preventDefault();
    const problem = validateOutcomeForm({ title, masteryPoints, method, calcInt, ratings });
    if (problem) {
      setError(problem);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const sorted = [...ratings].map((r) => ({ description: r.description.trim(), points: Number(r.points) || 0 }))
        .sort((a, b) => b.points - a.points);
      await onSave({
        title: title.trim(),
        display_name: displayName.trim(),
        description,
        mastery_points: Number(masteryPoints),
        calculation_method: method,
        ...(info?.defaultInt != null ? { calculation_int: Number(calcInt) } : {}),
        ratings: JSON.stringify(sorted),
        points_possible: Math.max(...sorted.map((r) => r.points)),
      });
    } catch (err) {
      setError(err?.message || 'Could not save the outcome.');
    } finally {
      setSaving(false);
    }
  };

  const field = 'w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';
  return (
    <form onSubmit={submit} className="space-y-3" aria-label="Edit outcome">
      {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
      <label className="block text-xs font-medium text-text-secondary">Title
        <input className={field} value={title} onChange={(e) => setTitle(e.target.value)} required />
      </label>
      <label className="block text-xs font-medium text-text-secondary">Friendly name (optional)
        <input className={field} value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
      </label>
      <label className="block text-xs font-medium text-text-secondary">Description
        <textarea className={field} rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <div className="grid grid-cols-2 gap-2">
        <label className="block text-xs font-medium text-text-secondary">Mastery at (points)
          <input type="number" min="0" step="0.5" className={field} value={masteryPoints} onChange={(e) => setMasteryPoints(e.target.value)} />
        </label>
        <label className="block text-xs font-medium text-text-secondary">Calculation method
          <select className={field} value={method} onChange={(e) => changeMethod(e.target.value)}>
            {CALCULATION_METHODS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
          </select>
        </label>
      </div>
      <p className="text-xs text-text-tertiary">{info?.description}</p>
      {info?.defaultInt != null && (
        <label className="block text-xs font-medium text-text-secondary">{info.intLabel}
          <input type="number" min={info.min} max={info.max} className={field} value={calcInt} onChange={(e) => setCalcInt(e.target.value)} />
        </label>
      )}
      <fieldset className="space-y-2">
        <legend className="text-xs font-medium text-text-secondary">Ratings</legend>
        {ratings.map((r, idx) => (
          <div key={idx} className="flex items-center gap-2">
            <input aria-label={`Rating ${idx + 1} description`} className={field} value={r.description}
              onChange={(e) => setRatings((p) => p.map((x, i) => (i === idx ? { ...x, description: e.target.value } : x)))} />
            <input aria-label={`Rating ${idx + 1} points`} type="number" min="0" step="0.5" className="w-20 border border-border-strong rounded px-2 py-1.5 text-sm" value={r.points}
              onChange={(e) => setRatings((p) => p.map((x, i) => (i === idx ? { ...x, points: e.target.value } : x)))} />
            <button type="button" aria-label={`Remove rating ${idx + 1}`} disabled={ratings.length <= 2}
              onClick={() => setRatings((p) => p.filter((_, i) => i !== idx))} className="p-1 text-text-disabled hover:text-accent-danger disabled:opacity-40">
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        ))}
        <button type="button" onClick={() => setRatings((p) => [...p, { description: '', points: 0 }])} className="inline-flex items-center gap-1 text-xs text-brand-600 hover:underline">
          <Plus className="w-3 h-3" /> Add rating
        </button>
      </fieldset>
      <div className="flex gap-2">
        <button type="submit" disabled={saving} className="px-3 py-1.5 text-sm rounded bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50">{saving ? 'Saving…' : 'Save'}</button>
        <button type="button" onClick={onCancel} className="px-3 py-1.5 text-sm rounded text-text-secondary hover:bg-surface-1">Cancel</button>
      </div>
    </form>
  );
}
