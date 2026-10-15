import React, { useState } from 'react';
import { CalendarClock } from 'lucide-react';
import { gradingPolicyApi } from '../../services/api_grading_policy';

// AccommodationExtensionsPanel shows which students' extra-day
// accommodations extend this assignment (G040). Extensions are part of
// each student's effective due and lock dates, so late / missing follow
// them; "Apply" records the application and refreshes statuses now.
const AccommodationExtensionsPanel = ({ courseId, assignmentId, students }) => {
  const [rows, setRows] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  const apply = async () => {
    const ids = (students || []).map((s) => s.id).filter(Boolean);
    if (ids.length === 0) {
      setRows([]);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await gradingPolicyApi.applyAccommodations(courseId, assignmentId, ids);
      setRows(res?.adjustments || []);
    } catch (e) {
      setError(e.message || 'Could not apply accommodations.');
    } finally {
      setBusy(false);
    }
  };

  const nameOf = (uid) => students.find((s) => s.id === uid)?.name || `User ${uid}`;
  const fmt = (d) => (d ? new Date(d).toLocaleString() : '—');

  return (
    <div className="bg-surface-0 rounded-lg shadow p-4 mb-6">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-text-secondary flex items-center gap-2">
          <CalendarClock className="w-4 h-4" />
          Accommodation extensions
        </h3>
        <button
          type="button"
          onClick={apply}
          disabled={busy}
          className="px-3 py-1 text-xs border border-border-strong rounded-md hover:bg-surface-1 disabled:opacity-50"
        >
          {busy ? 'Applying…' : 'Apply accommodations'}
        </button>
      </div>
      {error && <p role="alert" className="mt-2 text-xs text-accent-danger">{error}</p>}
      {rows && rows.length === 0 && (
        <p className="mt-2 text-xs text-text-tertiary">No student has an extra-day accommodation for this assignment.</p>
      )}
      {rows && rows.length > 0 && (
        <table className="mt-2 w-full text-xs">
          <thead>
            <tr className="text-left text-text-tertiary">
              <th className="py-1">Student</th>
              <th className="py-1">Due</th>
              <th className="py-1">Extended to</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.user_id} className="border-t border-border-subtle">
                <td className="py-1">{nameOf(r.user_id)}</td>
                <td className="py-1">{fmt(r.original_due_at)}</td>
                <td className="py-1">
                  {fmt(r.adjusted_due_at)} <span className="text-text-tertiary">(+{r.extra_days}d)</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
};

export default AccommodationExtensionsPanel;
