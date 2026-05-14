import React, { useCallback, useEffect, useState } from 'react';
import { Plus, Lock, Pencil, Trash2 } from 'lucide-react';
import { api } from '../../services/api';
import { BadgeIcon } from './BadgeIcon';
import BadgeEditor from './BadgeEditor';

// BadgesList — admin/instructor table of badges, scoped by route.
// Mirrors W2-B's CurrencyList shape: same scope-filter pattern, same
// edit/delete actions, same "system_owned rows can't be deleted"
// affordance.
export default function BadgesList({ courseId }) {
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
      const result = await api.gamification.listBadges();
      const all = result.badges || [];
      const filtered = all.filter((b) => {
        if (scope === 'course') {
          return b.scope_type === 'course' && Number(b.scope_id) === Number(courseId);
        }
        return b.scope_type === 'site';
      });
      filtered.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
      setRows(filtered);
    } catch (err) {
      console.error('BadgesList: load failed', err);
      setError(err.message || 'Could not load badges.');
    } finally {
      setLoading(false);
    }
  }, [scope, courseId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleSave = async (body) => {
    setSaving(true);
    setSaveError(null);
    try {
      if (editing) {
        await api.gamification.updateBadge(editing.id, body, { courseId });
      } else {
        await api.gamification.createBadge(body, { courseId });
      }
      setEditorOpen(false);
      setEditing(null);
      await load();
    } catch (err) {
      setSaveError(err.message || 'Save failed.');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (row) => {
    if (row.system_owned) return;
    const ok = window.confirm(
      `Delete "${row.name}"? This permanently removes the badge and ALL existing learner awards of it (the database CASCADEs).`,
    );
    if (!ok) return;
    try {
      await api.gamification.deleteBadge(row.id, { courseId });
      await load();
    } catch (err) {
      setError(err.message || 'Delete failed.');
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-text-primary">Badges</h1>
          <p className="text-sm text-text-secondary">
            {scope === 'course'
              ? 'Course-scoped badges. Only earnable inside this course.'
              : 'Site-wide badges. Available to every course.'}
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
          <Plus className="w-4 h-4" /> New badge
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
              <th className="text-left px-4 py-2 font-medium">Badge</th>
              <th className="text-left px-4 py-2 font-medium">Code</th>
              <th className="text-left px-4 py-2 font-medium">Visibility</th>
              <th className="text-right px-4 py-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={4} className="px-4 py-6 text-center text-text-tertiary">
                  Loading…
                </td>
              </tr>
            )}
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={4} className="px-4 py-6 text-center text-text-tertiary">
                  No badges yet. Click &ldquo;New badge&rdquo; to create one.
                </td>
              </tr>
            )}
            {!loading &&
              rows.map((row) => (
                <tr key={row.id} className="border-t border-surface-raised">
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-3">
                      <BadgeIcon badge={row} size="sm" />
                      <div className="min-w-0">
                        <div className="text-text-primary truncate flex items-center gap-1.5">
                          {row.name}
                          {row.system_owned && (
                            <Lock className="w-3 h-3 text-text-tertiary" aria-label="System badge" />
                          )}
                        </div>
                        <div className="text-xs text-text-tertiary truncate max-w-md">{row.description}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-2"><code className="text-xs text-text-tertiary">{row.code}</code></td>
                  <td className="px-4 py-2 text-xs">
                    {row.internal_only ? (
                      <span className="text-text-secondary">Internal-only</span>
                    ) : (
                      <span className="text-accent-warning">External-eligible</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-right">
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
                      disabled={row.system_owned}
                      title={row.system_owned ? 'System badges cannot be deleted' : 'Delete'}
                      className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 hover:text-accent-danger disabled:opacity-30 disabled:cursor-not-allowed"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <BadgeEditor
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
        badge={editing}
        onSave={handleSave}
        saving={saving}
        saveError={saveError}
      />
    </div>
  );
}
