import React, { useCallback, useEffect, useState } from 'react';
import { Plus, Pencil, Trash2, Power, PowerOff } from 'lucide-react';
import { api } from '../../services/api';
import RecipeEditor from './recipe/RecipeEditor';

// RecipesList — admin/instructor table of authored rules.
//
// Scope follows the same pattern as W2-B currencies + W2-D badges:
// when the route includes `:courseId`, the list shows course-scope
// rules and writes hit the course-scope handlers (instructor auth).
// Without `:courseId`, site-scope (tenant admin).
//
// The list endpoint already returns scope-filtered rows via
// `ListByScope` (W2-E.1), so we don't post-filter here. The display
// surfaces just enough of each rule that an admin can identify it
// without opening the editor: name, trigger summary, condition count,
// effect count, enabled toggle.
export default function RecipesList({ courseId }) {
  const scope = courseId ? 'course' : 'site';
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.gamification.listRules({ courseId });
      setRows(result?.rules || []);
    } catch (err) {
      console.error('RecipesList: load failed', err);
      setError(err.message || 'Could not load recipes.');
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleSave = async (body) => {
    setSaving(true);
    setSaveError(null);
    try {
      if (editing) {
        await api.gamification.updateRule(editing.id, body, { courseId });
      } else {
        await api.gamification.createRule(body, { courseId });
      }
      setEditorOpen(false);
      setEditing(null);
      await load();
    } catch (err) {
      // The W2-E.1 validator returns descriptive 400 messages — surface
      // them to the editor so the author sees exactly which field
      // failed. err.message is whatever services/api.js packs into
      // the thrown Error for non-2xx responses.
      setSaveError(err.message || 'Save failed.');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (row) => {
    const ok = window.confirm(
      `Delete recipe "${row.name}"? Evaluation history for this rule is also removed (the rule_evaluations rows CASCADE).`,
    );
    if (!ok) return;
    try {
      await api.gamification.deleteRule(row.id, { courseId });
      await load();
    } catch (err) {
      setError(err.message || 'Delete failed.');
    }
  };

  // Quick-toggle the `enabled` flag without opening the full editor.
  // The W2-E.1 bool-default fix on the repo Create path means
  // PATCH with enabled:false is actually persisted as false.
  const handleToggleEnabled = async (row) => {
    try {
      await api.gamification.updateRule(row.id, { enabled: !row.enabled }, { courseId });
      await load();
    } catch (err) {
      setError(err.message || 'Could not toggle.');
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-text-primary">Recipes</h1>
          <p className="text-sm text-text-secondary">
            {scope === 'course'
              ? 'Course-scoped rules. Only fire on events that happen inside this course.'
              : 'Site-wide rules. Fire on events from anywhere in the tenant.'}
          </p>
        </div>
        <button
          type="button"
          onClick={() => {
            setEditing(null);
            setEditorOpen(true);
          }}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-400/60"
        >
          <Plus className="w-4 h-4" /> New recipe
        </button>
      </header>

      <div className="inline-flex items-center gap-2 text-xs text-text-tertiary">
        <span className="px-2 py-0.5 rounded-full bg-surface-2 border border-surface-raised">
          Scope: {scope === 'course' ? `course #${courseId}` : 'site'}
        </span>
      </div>

      {error && (
        <div className="text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2">
          {error}
        </div>
      )}

      <div className="border border-surface-raised rounded-lg overflow-hidden bg-surface-0">
        <table className="w-full text-sm">
          <thead className="bg-surface-1 text-text-tertiary text-xs uppercase">
            <tr>
              <th className="text-left px-4 py-2 font-medium">Name</th>
              <th className="text-left px-4 py-2 font-medium">Trigger</th>
              <th className="text-left px-4 py-2 font-medium">Conditions</th>
              <th className="text-left px-4 py-2 font-medium">Effects</th>
              <th className="text-left px-4 py-2 font-medium">Status</th>
              <th className="text-right px-4 py-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center text-text-tertiary">Loading…</td>
              </tr>
            )}
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-6 text-center text-text-tertiary">
                  No recipes yet. Click &ldquo;New recipe&rdquo; to create one.
                </td>
              </tr>
            )}
            {!loading &&
              rows.map((row) => (
                <tr key={row.id} className="border-t border-surface-raised">
                  <td className="px-4 py-2">
                    <div className="text-text-primary">{row.name}</div>
                    {row.description && (
                      <div className="text-xs text-text-tertiary truncate max-w-md">{row.description}</div>
                    )}
                  </td>
                  <td className="px-4 py-2 text-xs text-text-secondary">{summarizeTrigger(row.trigger_event)}</td>
                  <td className="px-4 py-2 text-xs text-text-tertiary">{countConditions(row.condition_set)}</td>
                  <td className="px-4 py-2 text-xs text-text-tertiary">{(parseJSON(row.effects, []) || []).length}</td>
                  <td className="px-4 py-2 text-xs">
                    {row.enabled ? (
                      <span className="text-accent-success">Enabled</span>
                    ) : (
                      <span className="text-text-tertiary">Disabled</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <button
                      type="button"
                      onClick={() => handleToggleEnabled(row)}
                      title={row.enabled ? 'Disable' : 'Enable'}
                      aria-label={row.enabled ? 'Disable recipe' : 'Enable recipe'}
                      className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary"
                    >
                      {row.enabled ? <Power className="w-4 h-4" /> : <PowerOff className="w-4 h-4" />}
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setEditing(row);
                        setEditorOpen(true);
                      }}
                      title="Edit"
                      className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary"
                    >
                      <Pencil className="w-4 h-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDelete(row)}
                      title="Delete"
                      className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-accent-danger"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <RecipeEditor
        open={editorOpen}
        onOpenChange={(o) => {
          if (!o) {
            setEditorOpen(false);
            setEditing(null);
            setSaveError(null);
          } else {
            setEditorOpen(true);
          }
        }}
        recipe={editing}
        onSave={handleSave}
        saving={saving}
        saveError={saveError}
      />
    </div>
  );
}

function summarizeTrigger(triggerJSON) {
  const t = parseJSON(triggerJSON, null);
  if (!t || !t.kind) return '—';
  if (t.kind === 'OnEvent') return `${t.verb || '?'} ${t.object_type || '?'}`;
  if (t.kind === 'OnSchedule') return `cron: ${t.cron || '?'}`;
  if (t.kind === 'OnManualTrigger') return `manual: ${t.handle || '?'}`;
  return t.kind;
}

function countConditions(conditionJSON) {
  const c = parseJSON(conditionJSON, null);
  if (!c || c.kind !== 'ConditionSet') return '0';
  return `${(c.children || []).length} (${c.op || 'AND'})`;
}

function parseJSON(value, fallback) {
  if (value == null) return fallback;
  if (typeof value === 'object') return value;
  try {
    return JSON.parse(value);
  } catch {
    return fallback;
  }
}
