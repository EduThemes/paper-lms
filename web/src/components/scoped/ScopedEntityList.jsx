import { useCallback, useEffect, useState } from 'react';
import { Plus, Lock, Pencil, Trash2 } from 'lucide-react';

// ScopedEntityList — shared admin table for tenant + course-scoped
// gamification primitives (currencies, badges, future shop items, etc.).
//
// Captures the structure both CurrencyList and BadgesList used to grow
// independently:
//   • per-scope load + filter (site vs course)
//   • create/edit/delete actions wired into a tab-bar dialog editor
//   • scope chip + page header
//   • system-owned row affordances (Lock icon, delete disabled)
//
// Configuration is by callbacks rather than booleans — every caller has
// slightly different column shape, sort order, and post-mutate side
// effects. The config object is documented in the propTypes-equivalent
// JSDoc on each prop below.
//
// @param {string} entityName            singular display name, e.g. "currency"
// @param {string} entityNamePlural      plural, e.g. "currencies"
// @param {string} pageTitle             header title, e.g. "Currencies"
// @param {(scope: 'site'|'course', courseId?: number) => string} pageSubtitle
//                                       header subtitle copy
// @param {(scope: 'site'|'course') => string} emptyStateCopy
// @param {string|number} [courseId]     pass when scoping to a course
// @param {{
//   list:   () => Promise<object>,
//   create: (body: object, opts: {courseId?: any}) => Promise<any>,
//   update: (id: any, body: object, opts: {courseId?: any}) => Promise<any>,
//   delete: (id: any, opts: {courseId?: any}) => Promise<any>,
// }} apiCalls
// @param {string} resultsKey            key on list-response that holds rows
//                                       (e.g. 'currencies', 'badges')
// @param {(rows: any[]) => any[]} sortRows
// @param {(row: any) => string} deleteConfirmMessage
// @param {Array<{ header: string, align?: 'left'|'right', render: (row) => React.Node }>} columns
// @param {React.ComponentType<{open,onOpenChange,onSave,saving,saveError, ...}>} EditorComponent
// @param {(props: {open,onOpenChange,row,onSave,saving,saveError}) => React.Node} renderEditor
//                                       optional; for editors that take a
//                                       custom prop name (e.g. `currency`,
//                                       `badge`). Receives the editing row.
// @param {() => void} [onAfterMutate]   called after successful save/delete
//                                       (e.g. dispatch `wallet:refresh`)
export default function ScopedEntityList({
  entityName,
  entityNamePlural,
  pageTitle,
  pageSubtitle,
  emptyStateCopy,
  courseId,
  apiCalls,
  resultsKey,
  sortRows,
  deleteConfirmMessage,
  columns,
  renderEditor,
  onAfterMutate,
}) {
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
      const result = await apiCalls.list();
      const all = result[resultsKey] || [];
      const filtered = all.filter((r) => {
        if (scope === 'course') {
          return r.scope_type === 'course' && Number(r.scope_id) === Number(courseId);
        }
        return r.scope_type === 'site';
      });
      setRows(sortRows ? sortRows(filtered) : filtered);
    } catch (err) {
      console.error(`ScopedEntityList(${entityName}): load failed`, err);
      setError(err.message || `Could not load ${entityNamePlural}.`);
    } finally {
      setLoading(false);
    }
  }, [scope, courseId, apiCalls, resultsKey, sortRows, entityName, entityNamePlural]);

  useEffect(() => {
    load();
  }, [load]);

  const handleSave = async (body) => {
    setSaving(true);
    setSaveError(null);
    try {
      if (editing) {
        await apiCalls.update(editing.id, body, { courseId });
      } else {
        await apiCalls.create(body, { courseId });
      }
      setEditorOpen(false);
      setEditing(null);
      await load();
      if (onAfterMutate) onAfterMutate();
    } catch (err) {
      setSaveError(err.message || 'Save failed.');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (row) => {
    if (row.system_owned) return;
    const ok = window.confirm(deleteConfirmMessage(row));
    if (!ok) return;
    try {
      await apiCalls.delete(row.id, { courseId });
      await load();
      if (onAfterMutate) onAfterMutate();
    } catch (err) {
      setError(err.message || 'Delete failed.');
    }
  };

  const colCount = columns.length + 1; // +1 for Actions column

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-text-primary">{pageTitle}</h1>
          <p className="text-sm text-text-secondary">{pageSubtitle(scope, courseId)}</p>
        </div>
        <button
          type="button"
          onClick={() => {
            setEditing(null);
            setEditorOpen(true);
          }}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-400/60"
        >
          <Plus className="w-4 h-4" /> New {entityName}
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
              {columns.map((col, i) => (
                <th
                  key={i}
                  className={`text-${col.align || 'left'} px-4 py-2 font-medium`}
                >
                  {col.header}
                </th>
              ))}
              <th className="text-right px-4 py-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={colCount} className="px-4 py-6 text-center text-text-tertiary">
                  Loading…
                </td>
              </tr>
            )}
            {!loading && rows.length === 0 && (
              <tr>
                <td colSpan={colCount} className="px-4 py-6 text-center text-text-tertiary">
                  {emptyStateCopy(scope)}
                </td>
              </tr>
            )}
            {!loading &&
              rows.map((row) => (
                <tr key={row.id} className="border-t border-surface-raised">
                  {columns.map((col, i) => (
                    <td
                      key={i}
                      className={`px-4 py-2 text-${col.align || 'left'}`}
                    >
                      {col.render(row, { Lock })}
                    </td>
                  ))}
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
                      title={row.system_owned ? `System ${entityNamePlural} cannot be deleted` : 'Delete'}
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

      {renderEditor({
        open: editorOpen,
        onOpenChange: (o) => {
          if (!o) {
            setEditorOpen(false);
            setEditing(null);
            setSaveError(null);
          } else {
            setEditorOpen(true);
          }
        },
        row: editing,
        onSave: handleSave,
        saving,
        saveError,
      })}
    </div>
  );
}
