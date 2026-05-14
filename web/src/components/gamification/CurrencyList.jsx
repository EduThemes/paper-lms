import React, { useCallback, useEffect, useState } from 'react';
import { Plus, Lock, Pencil, Trash2 } from 'lucide-react';
import { api } from '../../services/api';
import { CurrencyIcon } from './currencyIcon';
import CurrencyEditor from './CurrencyEditor';

// CurrencyList renders the per-scope currency table with create / edit /
// delete actions. `courseId` is optional; pass it to scope writes to a
// course (instructor surface), omit for site scope (tenant admin). The
// list itself comes from the existing tenant-wide endpoint; client-side
// filtering narrows to the route scope so an instructor only sees the
// row set they can actually edit.
export default function CurrencyList({ courseId }) {
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
      const result = await api.gamification.listCurrencies();
      const all = result.currencies || [];
      const filtered = all.filter((c) => {
        if (scope === 'course') {
          return c.scope_type === 'course' && Number(c.scope_id) === Number(courseId);
        }
        return c.scope_type === 'site';
      });
      filtered.sort((a, b) => (a.display_order ?? 99) - (b.display_order ?? 99));
      setRows(filtered);
    } catch (err) {
      console.error('CurrencyList: load failed', err);
      setError(err.message || 'Could not load currencies.');
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
        await api.gamification.updateCurrency(editing.id, body, { courseId });
      } else {
        await api.gamification.createCurrency(body, { courseId });
      }
      setEditorOpen(false);
      setEditing(null);
      await load();
      // Tell any mounted CurrencyPills to re-fetch so the new currency
      // shows immediately for currently-signed-in users.
      window.dispatchEvent(new Event('wallet:refresh'));
    } catch (err) {
      setSaveError(err.message || 'Save failed.');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (row) => {
    if (row.system_owned) return;
    const ok = window.confirm(
      `Delete "${row.display_label}"? Existing wallet balances will keep their currency_type_id but this currency will no longer be addressable by name.`,
    );
    if (!ok) return;
    try {
      await api.gamification.deleteCurrency(row.id, { courseId });
      await load();
      window.dispatchEvent(new Event('wallet:refresh'));
    } catch (err) {
      // surface the message inline rather than throwing
      setError(err.message || 'Delete failed.');
    }
  };

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-text-primary">Currencies</h1>
          <p className="text-sm text-text-secondary">
            {scope === 'course'
              ? 'Course-scoped currencies. Only visible inside this course.'
              : 'Site-wide currencies. Available to every course.'}
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
          <Plus className="w-4 h-4" /> New currency
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
              <th className="text-left px-4 py-2 font-medium">Code / Label</th>
              <th className="text-left px-4 py-2 font-medium">Behavior</th>
              <th className="text-left px-4 py-2 font-medium">Topbar</th>
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
                  {scope === 'course'
                    ? 'No course-scoped currencies yet. Click "New currency" to create one.'
                    : 'No currencies — the seeder should have created the four system rows. Check the server logs.'}
                </td>
              </tr>
            )}
            {!loading &&
              rows.map((row) => (
                <tr key={row.id} className="border-t border-surface-raised">
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      <CurrencyIcon icon={row.icon} color={row.color} className="w-4 h-4" />
                      <div className="min-w-0">
                        <div className="text-text-primary truncate flex items-center gap-1.5">
                          {row.display_label}
                          {row.system_owned && (
                            <Lock className="w-3 h-3 text-text-tertiary" aria-label="System currency" />
                          )}
                        </div>
                        <code className="text-xs text-text-tertiary">{row.code}</code>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-2 text-text-secondary text-xs">
                    {[
                      row.spendable ? 'spendable' : null,
                      row.monotonic ? 'monotonic' : null,
                      !row.visible_to_student ? 'instructor-only' : null,
                    ]
                      .filter(Boolean)
                      .join(' · ') || '—'}
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {row.visible_in_topbar ? (
                      <span className="text-accent-success">Visible</span>
                    ) : (
                      <span className="text-text-tertiary">Hidden</span>
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
                      title={row.system_owned ? 'System currencies cannot be deleted' : 'Delete'}
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

      <CurrencyEditor
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
        currency={editing}
        onSave={handleSave}
        saving={saving}
        saveError={saveError}
      />
    </div>
  );
}
