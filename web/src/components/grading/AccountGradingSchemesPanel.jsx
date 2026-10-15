import React, { useEffect, useState } from 'react';
import { Plus, Trash2, Edit2 } from 'lucide-react';
import { gradingPolicyApi } from '../../services/api_grading_policy';

const DEFAULT_ROWS = [
  ['A', 94], ['A-', 90], ['B+', 87], ['B', 84], ['B-', 80], ['C+', 77], ['C', 74], ['C-', 70], ['D+', 67], ['D', 64], ['D-', 61], ['F', 0],
];

// toRows turns stored data ([name, fraction]) into editable rows in percent.
const toRows = (data) =>
  Array.isArray(data) && data.length > 0
    ? data.map(([n, v]) => [n, Math.round(Number(v) * 10000) / 100])
    : DEFAULT_ROWS.map((r) => [...r]);

// AccountGradingSchemesPanel manages grading schemes shared by every
// course of the account (G039): letter / GPA cutoffs, optionally shown
// as points on a scale (e.g. 4.0).
const AccountGradingSchemesPanel = ({ accountId }) => {
  const [schemes, setSchemes] = useState([]);
  const [editing, setEditing] = useState(null); // {id?, title, rows, points_based, scaling_factor}
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = () =>
    gradingPolicyApi
      .listAccountGradingStandards(accountId)
      .then((d) => setSchemes(Array.isArray(d) ? d : []))
      .catch((e) => setError(e.message));

  useEffect(() => {
    if (accountId) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId]);

  const startNew = () =>
    setEditing({ title: '', rows: DEFAULT_ROWS.map((r) => [...r]), points_based: false, scaling_factor: 4 });
  const startEdit = (s) =>
    setEditing({ id: s.id, title: s.title, rows: toRows(s.data), points_based: !!s.points_based, scaling_factor: s.scaling_factor || 1 });

  const setRow = (i, col, value) =>
    setEditing((e) => ({ ...e, rows: e.rows.map((r, j) => (j === i ? (col === 0 ? [value, r[1]] : [r[0], value]) : r)) }));

  const save = async (ev) => {
    ev.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const payload = {
        title: editing.title,
        data: editing.rows.filter((r) => String(r[0]).trim() !== '').map(([n, v]) => [String(n).trim(), Number(v)]),
        points_based: editing.points_based,
        scaling_factor: editing.points_based ? Number(editing.scaling_factor) : 1,
      };
      if (editing.id) await gradingPolicyApi.updateAccountGradingStandard(accountId, editing.id, payload);
      else await gradingPolicyApi.createAccountGradingStandard(accountId, payload);
      setEditing(null);
      load();
    } catch (e) {
      setError(e.message || 'Could not save the grading scheme.');
    } finally {
      setBusy(false);
    }
  };

  const remove = async (s) => {
    if (!window.confirm(`Delete the grading scheme "${s.title}"? Courses using it fall back to their default.`)) return;
    try {
      await gradingPolicyApi.deleteAccountGradingStandard(accountId, s.id);
      load();
    } catch (e) {
      setError(e.message);
    }
  };

  return (
    <section aria-labelledby="account-schemes-heading" className="mt-8">
      <div className="flex items-center justify-between mb-3">
        <h3 id="account-schemes-heading" className="text-xl font-bold">Grading schemes</h3>
        {!editing && (
          <button type="button" onClick={startNew}
            className="flex items-center gap-2 bg-brand-600 text-on-brand px-3 py-1.5 rounded-md hover:bg-brand-700 text-sm">
            <Plus className="w-4 h-4" /> New scheme
          </button>
        )}
      </div>
      <p className="text-sm text-text-tertiary mb-3">Schemes defined here can be selected by every course in the account.</p>
      {error && <div role="alert" className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-3 text-sm">{error}</div>}

      {editing && (
        <form onSubmit={save} className="bg-surface-0 rounded-lg shadow p-4 mb-4 space-y-3 text-sm">
          <label className="block">
            <span className="block font-medium text-text-secondary mb-1">Title</span>
            <input required value={editing.title} onChange={(e) => setEditing({ ...editing, title: e.target.value })}
              className="w-full border border-border-strong rounded px-3 py-2" />
          </label>
          <div className="flex flex-wrap items-center gap-4">
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={editing.points_based}
                onChange={(e) => setEditing({ ...editing, points_based: e.target.checked })} />
              Points based
            </label>
            {editing.points_based && (
              <label className="flex items-center gap-2">
                Scale out of
                <input type="number" min="0.01" step="0.01" value={editing.scaling_factor}
                  onChange={(e) => setEditing({ ...editing, scaling_factor: e.target.value })}
                  className="w-20 border border-border-strong rounded px-2 py-1" />
              </label>
            )}
          </div>
          <table className="w-full max-w-md">
            <thead>
              <tr className="text-left text-text-tertiary text-xs">
                <th className="py-1">Grade</th>
                <th className="py-1">Minimum %</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {editing.rows.map((r, i) => (
                <tr key={i}>
                  <td className="pr-2 py-0.5">
                    <input aria-label={`Grade ${i + 1} name`} value={r[0]} onChange={(e) => setRow(i, 0, e.target.value)}
                      className="w-24 border border-border-strong rounded px-2 py-1" />
                  </td>
                  <td className="pr-2 py-0.5">
                    <input aria-label={`Grade ${i + 1} minimum percent`} type="number" min="0" max="100" step="0.01" value={r[1]}
                      onChange={(e) => setRow(i, 1, e.target.value)} className="w-24 border border-border-strong rounded px-2 py-1" />
                  </td>
                  <td>
                    <button type="button" aria-label={`Remove grade ${r[0] || i + 1}`}
                      onClick={() => setEditing((e) => ({ ...e, rows: e.rows.filter((_, j) => j !== i) }))}
                      className="text-text-tertiary hover:text-accent-danger">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <button type="button" onClick={() => setEditing((e) => ({ ...e, rows: [...e.rows, ['', 0]] }))}
            className="text-brand-600 hover:underline">
            Add grade
          </button>
          <div className="flex gap-2">
            <button type="submit" disabled={busy} className="bg-brand-600 text-on-brand px-3 py-1.5 rounded hover:bg-brand-700 disabled:opacity-50">
              {busy ? 'Saving…' : 'Save scheme'}
            </button>
            <button type="button" onClick={() => setEditing(null)} className="text-text-tertiary">Cancel</button>
          </div>
        </form>
      )}

      <div className="space-y-2">
        {schemes.length === 0 && !editing && (
          <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary text-sm">No account grading schemes yet.</div>
        )}
        {schemes.map((s) => (
          <div key={s.id} className="bg-surface-0 rounded-lg shadow px-4 py-3 flex items-start justify-between">
            <div>
              <p className="font-medium">{s.title}{s.points_based ? ` (points, out of ${s.scaling_factor})` : ''}</p>
              <p className="text-xs text-text-tertiary">
                {(Array.isArray(s.data) ? s.data : []).map(([n, v]) => `${n} ≥ ${Math.round(Number(v) * 10000) / 100}%`).join(', ')}
              </p>
            </div>
            <div className="flex gap-2">
              <button type="button" aria-label={`Edit ${s.title}`} onClick={() => startEdit(s)} className="text-text-tertiary hover:text-brand-600">
                <Edit2 className="w-4 h-4" />
              </button>
              <button type="button" aria-label={`Delete ${s.title}`} onClick={() => remove(s)} className="text-text-tertiary hover:text-accent-danger">
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
};

export default AccountGradingSchemesPanel;
