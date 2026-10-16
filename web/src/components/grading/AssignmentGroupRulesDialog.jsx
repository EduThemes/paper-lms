import React, { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '../ui/dialog';
import { gradingPolicyApi } from '../../services/api_grading_policy';
import { parseRules } from '../../utils/gradingRules';

// AssignmentGroupRulesDialog edits a group's weight and drop rules
// (G020): drop the N lowest / highest scores, never dropping the chosen
// assignments.
const AssignmentGroupRulesDialog = ({ open, onOpenChange, courseId, group, assignments = [], onSaved }) => {
  const [weight, setWeight] = useState('0');
  const [lowest, setLowest] = useState('0');
  const [highest, setHighest] = useState('0');
  const [neverDrop, setNeverDrop] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!open || !group) return;
    const r = parseRules(group.rules);
    setWeight(String(group.group_weight ?? 0));
    setLowest(String(r.drop_lowest));
    setHighest(String(r.drop_highest));
    setNeverDrop(r.never_drop);
    setError(null);
  }, [open, group]);

  const save = async () => {
    const lo = Number(lowest);
    const hi = Number(highest);
    const w = Number(weight);
    if (![lo, hi].every((n) => Number.isInteger(n) && n >= 0)) {
      setError('Drop counts must be whole numbers of 0 or more.');
      return;
    }
    if (Number.isNaN(w) || w < 0) {
      setError('Weight cannot be negative.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const saved = await gradingPolicyApi.updateAssignmentGroup(courseId, group.id, {
        groupWeight: w,
        rules: { drop_lowest: lo, drop_highest: hi, never_drop: neverDrop },
      });
      onSaved?.(saved);
      onOpenChange(false);
    } catch (e) {
      setError(e.message || 'Could not save the group.');
    } finally {
      setBusy(false);
    }
  };

  const toggleNever = (id) =>
    setNeverDrop((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Group settings{group ? `: ${group.name}` : ''}</DialogTitle>
          <DialogDescription>Weight and drop rules for this assignment group.</DialogDescription>
        </DialogHeader>
        <div className="space-y-3 text-sm">
          <div className="grid grid-cols-3 gap-3">
            <label className="block">
              <span className="block font-medium text-text-secondary mb-1">Weight (%)</span>
              <input type="number" min="0" step="0.01" value={weight} onChange={(e) => setWeight(e.target.value)}
                className="w-full border border-border-strong rounded-md px-2 py-1.5" />
            </label>
            <label className="block">
              <span className="block font-medium text-text-secondary mb-1">Drop lowest</span>
              <input type="number" min="0" step="1" value={lowest} onChange={(e) => setLowest(e.target.value)}
                className="w-full border border-border-strong rounded-md px-2 py-1.5" />
            </label>
            <label className="block">
              <span className="block font-medium text-text-secondary mb-1">Drop highest</span>
              <input type="number" min="0" step="1" value={highest} onChange={(e) => setHighest(e.target.value)}
                className="w-full border border-border-strong rounded-md px-2 py-1.5" />
            </label>
          </div>
          {assignments.length > 0 && (
            <fieldset>
              <legend className="font-medium text-text-secondary mb-1">Never drop</legend>
              <div className="max-h-40 overflow-y-auto space-y-1">
                {assignments.map((a) => (
                  <label key={a.id} className="flex items-center gap-2">
                    <input type="checkbox" checked={neverDrop.includes(a.id)} onChange={() => toggleNever(a.id)} />
                    {a.name}
                  </label>
                ))}
              </div>
            </fieldset>
          )}
          {error && (
            <p role="alert" className="text-accent-danger">
              {error}
            </p>
          )}
        </div>
        <DialogFooter>
          <button type="button" onClick={() => onOpenChange(false)} disabled={busy}
            className="px-4 py-2 text-sm border border-border-strong rounded-md hover:bg-surface-1">
            Cancel
          </button>
          <button type="button" onClick={save} disabled={busy}
            className="px-4 py-2 text-sm bg-brand-600 text-on-brand rounded-md hover:bg-brand-700 disabled:opacity-50">
            {busy ? 'Saving…' : 'Save'}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default AssignmentGroupRulesDialog;
