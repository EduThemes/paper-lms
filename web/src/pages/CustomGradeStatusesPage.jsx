import React, { useCallback, useEffect, useState } from 'react';
import { Plus, Trash2, Save, Tag } from 'lucide-react';
import Layout from '../components/Layout';
import useAccountId from '../hooks/useAccountId';
import gradingApi from '../services/api_grading_ui';

// Account admins define custom grade statuses (Canvas
// custom_grade_statuses): a name and a color, usable on submissions in
// SpeedGrader and the gradebook, and/or on final grades.

const blank = { name: '', color: '#E0E0E0', applies_to_submissions: true, applies_to_final_grade: false };
const HEX = /^#[0-9A-Fa-f]{6}$/;

const CustomGradeStatusesPage = () => {
  const accountId = useAccountId();
  const [statuses, setStatuses] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [draft, setDraft] = useState(blank);
  const [edits, setEdits] = useState({});
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!accountId) return;
    try {
      setLoading(true);
      setStatuses(await gradingApi.listAccountGradeStatuses(accountId));
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  useEffect(() => { load(); }, [load]);

  const create = async (e) => {
    e.preventDefault();
    if (!draft.name.trim()) return;
    if (!HEX.test(draft.color)) { setError('Color must be a hex value like #FFAA00.'); return; }
    setSaving(true);
    setError(null);
    try {
      const created = await gradingApi.createGradeStatus(accountId, { ...draft, name: draft.name.trim() });
      setStatuses((prev) => [...prev, created]);
      setDraft(blank);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const saveEdit = async (status) => {
    const patch = edits[status.id];
    if (!patch) return;
    if (patch.color && !HEX.test(patch.color)) { setError('Color must be a hex value like #FFAA00.'); return; }
    setError(null);
    try {
      const saved = await gradingApi.updateGradeStatus(accountId, status.id, patch);
      setStatuses((prev) => prev.map((s) => (s.id === status.id ? { ...s, ...saved } : s)));
      setEdits((prev) => { const n = { ...prev }; delete n[status.id]; return n; });
    } catch (err) {
      setError(err.message);
    }
  };

  const remove = async (status) => {
    if (!window.confirm(`Delete the "${status.name}" status? Submissions that use it lose it.`)) return;
    setError(null);
    try {
      await gradingApi.deleteGradeStatus(accountId, status.id);
      setStatuses((prev) => prev.filter((s) => s.id !== status.id));
    } catch (err) {
      setError(err.message);
    }
  };

  const edit = (id, patch) => setEdits((prev) => ({ ...prev, [id]: { ...prev[id], ...patch } }));
  const val = (s, k) => (edits[s.id] && k in edits[s.id] ? edits[s.id][k] : s[k]);

  return (
    <Layout>
      <div className="p-6 max-w-4xl">
        <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2 mb-1">
          <Tag className="w-6 h-6" /> Grade Statuses
        </h1>
        <p className="text-sm text-text-tertiary mb-6">
          Custom statuses graders can set on a submission (besides Late, Missing and Extended) or on a final grade.
        </p>

        {error && <div className="mb-4 p-3 rounded-md text-sm bg-accent-danger/10 text-accent-danger border border-accent-danger/30" role="alert">{error}</div>}

        <form onSubmit={create} className="bg-surface-0 rounded-lg shadow p-4 mb-6 flex flex-wrap items-end gap-3">
          <label htmlFor="status-name" className="text-sm text-text-secondary">
            Name
            <input id="status-name" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} maxLength={64}
              className="block mt-1 border border-border-strong rounded-md px-3 py-2 text-sm" placeholder="Incomplete" />
          </label>
          <label htmlFor="status-color" className="text-sm text-text-secondary">
            Color
            <input id="status-color" type="color" value={draft.color} onChange={(e) => setDraft({ ...draft, color: e.target.value.toUpperCase() })}
              className="block mt-1 h-9 w-14 border border-border-strong rounded-md" />
          </label>
          <label className="flex items-center gap-2 text-sm text-text-secondary">
            <input type="checkbox" checked={draft.applies_to_submissions} onChange={(e) => setDraft({ ...draft, applies_to_submissions: e.target.checked })} />
            Submissions
          </label>
          <label className="flex items-center gap-2 text-sm text-text-secondary">
            <input type="checkbox" checked={draft.applies_to_final_grade} onChange={(e) => setDraft({ ...draft, applies_to_final_grade: e.target.checked })} />
            Final grades
          </label>
          <button type="submit" disabled={saving || !draft.name.trim()}
            className="inline-flex items-center gap-1.5 px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
            <Plus className="w-4 h-4" /> Add status
          </button>
        </form>

        {loading ? (
          <p className="text-text-tertiary">Loading...</p>
        ) : statuses.length === 0 ? (
          <p className="text-text-tertiary">No custom statuses yet.</p>
        ) : (
          <ul className="bg-surface-0 rounded-lg shadow divide-y">
            {statuses.map((s) => (
              <li key={s.id} className="p-4 flex flex-wrap items-center gap-3">
                <span className="h-5 w-5 rounded border border-border-default" style={{ backgroundColor: val(s, 'color') }} aria-hidden="true" />
                <input aria-label={`Name of ${s.name}`} value={val(s, 'name')} onChange={(e) => edit(s.id, { name: e.target.value })}
                  className="border border-border-strong rounded-md px-2 py-1 text-sm" />
                <input aria-label={`Color of ${s.name}`} type="color" value={val(s, 'color')} onChange={(e) => edit(s.id, { color: e.target.value.toUpperCase() })}
                  className="h-8 w-12 border border-border-strong rounded" />
                <label className="flex items-center gap-1 text-xs text-text-secondary">
                  <input type="checkbox" checked={!!val(s, 'applies_to_submissions')} onChange={(e) => edit(s.id, { applies_to_submissions: e.target.checked })} />
                  Submissions
                </label>
                <label className="flex items-center gap-1 text-xs text-text-secondary">
                  <input type="checkbox" checked={!!val(s, 'applies_to_final_grade')} onChange={(e) => edit(s.id, { applies_to_final_grade: e.target.checked })} />
                  Final grades
                </label>
                <div className="ml-auto flex items-center gap-2">
                  {edits[s.id] && (
                    <button type="button" onClick={() => saveEdit(s)} className="inline-flex items-center gap-1 text-sm text-brand-600 hover:underline">
                      <Save className="w-4 h-4" /> Save
                    </button>
                  )}
                  <button type="button" onClick={() => remove(s)} aria-label={`Delete ${s.name}`} className="text-text-disabled hover:text-accent-danger">
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Layout>
  );
};

export default CustomGradeStatusesPage;
