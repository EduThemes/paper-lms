import React, { useState, useEffect, useCallback } from 'react';
import { X, Plus, GripVertical, Trash2, Eye, EyeOff, Lock, Unlock, Pencil, Save } from 'lucide-react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { api } from '../../services/api';

/**
 * CustomColumns — modal dialog for managing Canvas-compatible custom
 * gradebook columns: add, rename, toggle visibility/read-only, reorder
 * (drag-and-drop), and delete.
 *
 * Props:
 *   - courseId         (string|number) required
 *   - open             (bool)          modal visibility
 *   - onClose          (fn)            invoked when user dismisses dialog
 *   - onColumnsChanged (fn(cols))      invoked after any successful mutation
 *                                      so the parent gradebook can refresh
 */
export default function CustomColumns({ courseId, open, onClose, onColumnsChanged }) {
  const [columns, setColumns] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [editingId, setEditingId] = useState(null);
  const [editingTitle, setEditingTitle] = useState('');

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const cols = await api.listCustomGradebookColumns(courseId, true);
      const list = Array.isArray(cols) ? cols : (cols?.data || []);
      setColumns(list);
      onColumnsChanged?.(list);
    } catch (err) {
      setError(err.message || 'Could not load custom columns');
    } finally {
      setLoading(false);
    }
  }, [courseId, onColumnsChanged]);

  useEffect(() => {
    if (open) refresh();
  }, [open, refresh]);

  const handleCreate = async (e) => {
    e?.preventDefault?.();
    const title = newTitle.trim();
    if (!title) return;
    setSaving(true);
    try {
      await api.createCustomGradebookColumn(courseId, { title });
      setNewTitle('');
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not create column');
    } finally {
      setSaving(false);
    }
  };

  const handleRename = async (id) => {
    const title = editingTitle.trim();
    if (!title) {
      setEditingId(null);
      return;
    }
    setSaving(true);
    try {
      await api.updateCustomGradebookColumn(courseId, id, { title });
      setEditingId(null);
      setEditingTitle('');
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not rename column');
    } finally {
      setSaving(false);
    }
  };

  const toggleField = async (col, field) => {
    setSaving(true);
    try {
      await api.updateCustomGradebookColumn(courseId, col.id, { [field]: !col[field] });
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not update column');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (col) => {
    if (!confirm(`Delete column "${col.title}"? Existing data will be hidden.`)) return;
    setSaving(true);
    try {
      await api.deleteCustomGradebookColumn(courseId, col.id);
      await refresh();
    } catch (err) {
      setError(err.message || 'Could not delete column');
    } finally {
      setSaving(false);
    }
  };

  const handleDragEnd = async (event) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = columns.findIndex((c) => c.id === active.id);
    const newIndex = columns.findIndex((c) => c.id === over.id);
    const reordered = arrayMove(columns, oldIndex, newIndex);
    setColumns(reordered); // optimistic
    try {
      await api.reorderCustomGradebookColumns(courseId, reordered.map((c) => c.id));
      onColumnsChanged?.(reordered);
    } catch (err) {
      setError(err.message || 'Could not reorder columns');
      refresh();
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="custom-cols-title"
      onClick={(e) => { if (e.target === e.currentTarget) onClose?.(); }}
    >
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-lg bg-surface-0 shadow-xl dark:bg-gray-800">
        <div className="flex items-center justify-between border-b border-border-default p-4 dark:border-gray-700">
          <h2 id="custom-cols-title" className="text-lg font-semibold text-text-primary dark:text-white">
            Custom Gradebook Columns
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 text-text-tertiary hover:bg-surface-2 dark:hover:bg-gray-700"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-4 p-4">
          {error && (
            <div className="rounded border border-red-300 bg-accent-danger/10 p-3 text-sm text-accent-danger dark:border-red-700 dark:bg-red-900/30 dark:text-red-200">
              {error}
            </div>
          )}

          <form onSubmit={handleCreate} className="flex gap-2">
            <input
              type="text"
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              placeholder="New column title (e.g., Notes, Effort)"
              maxLength={255}
              className="flex-1 rounded border border-border-strong px-3 py-2 text-sm focus:border-brand-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
            <button
              type="submit"
              disabled={saving || !newTitle.trim()}
              className="inline-flex items-center gap-1 rounded bg-brand-600 px-3 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
            >
              <Plus className="h-4 w-4" /> Add
            </button>
          </form>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <svg className="h-6 w-6 animate-spin text-brand-600" viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" className="opacity-25" />
                <path fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" className="opacity-75" />
              </svg>
            </div>
          ) : columns.length === 0 ? (
            <div className="rounded border border-dashed border-border-strong p-6 text-center text-sm text-text-tertiary dark:border-gray-600 dark:text-text-disabled">
              No custom columns yet. Add one above to get started.
            </div>
          ) : (
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={handleDragEnd}
            >
              <SortableContext items={columns.map((c) => c.id)} strategy={verticalListSortingStrategy}>
                <ul className="space-y-2">
                  {columns.map((col) => (
                    <SortableRow
                      key={col.id}
                      col={col}
                      isEditing={editingId === col.id}
                      editingTitle={editingTitle}
                      onStartEdit={() => { setEditingId(col.id); setEditingTitle(col.title); }}
                      onCancelEdit={() => { setEditingId(null); setEditingTitle(''); }}
                      onChangeTitle={setEditingTitle}
                      onSaveTitle={() => handleRename(col.id)}
                      onToggle={(field) => toggleField(col, field)}
                      onDelete={() => handleDelete(col)}
                      saving={saving}
                    />
                  ))}
                </ul>
              </SortableContext>
            </DndContext>
          )}
        </div>

        <div className="flex justify-end border-t border-border-default p-4 dark:border-gray-700">
          <button
            type="button"
            onClick={onClose}
            className="rounded bg-border-default px-4 py-2 text-sm font-medium text-text-primary hover:bg-gray-300 dark:bg-gray-700 dark:text-white dark:hover:bg-gray-600"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
}

function SortableRow({
  col,
  isEditing,
  editingTitle,
  onStartEdit,
  onCancelEdit,
  onChangeTitle,
  onSaveTitle,
  onToggle,
  onDelete,
  saving,
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: col.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  return (
    <li
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-2 rounded border border-border-default bg-surface-0 p-2 dark:border-gray-700 dark:bg-gray-900"
    >
      <button
        type="button"
        className="cursor-grab touch-none rounded p-1 text-text-disabled hover:bg-surface-2 dark:hover:bg-gray-700"
        aria-label={`Drag to reorder ${col.title}`}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-4 w-4" />
      </button>

      {isEditing ? (
        <input
          type="text"
          value={editingTitle}
          onChange={(e) => onChangeTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSaveTitle();
            if (e.key === 'Escape') onCancelEdit();
          }}
          autoFocus
          maxLength={255}
          className="flex-1 rounded border border-border-strong px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
        />
      ) : (
        <span className="flex-1 truncate text-sm text-text-primary dark:text-white">
          {col.title}
          {col.teacher_notes && (
            <span className="ml-2 inline-block rounded bg-purple-100 px-1.5 py-0.5 text-xs text-purple-800 dark:bg-purple-900/40 dark:text-purple-200">
              Teacher Notes
            </span>
          )}
        </span>
      )}

      <div className="flex items-center gap-1">
        {isEditing ? (
          <>
            <button
              type="button"
              onClick={onSaveTitle}
              disabled={saving}
              className="rounded p-1 text-accent-success hover:bg-accent-success/10 dark:hover:bg-green-900/30"
              aria-label="Save"
            >
              <Save className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={onCancelEdit}
              className="rounded p-1 text-text-tertiary hover:bg-surface-2 dark:hover:bg-gray-700"
              aria-label="Cancel"
            >
              <X className="h-4 w-4" />
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              onClick={() => onToggle('hidden')}
              disabled={saving}
              className="rounded p-1 text-text-tertiary hover:bg-surface-2 dark:hover:bg-gray-700"
              aria-label={col.hidden ? 'Show column' : 'Hide column'}
              title={col.hidden ? 'Hidden' : 'Visible'}
            >
              {col.hidden ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
            <button
              type="button"
              onClick={() => onToggle('read_only')}
              disabled={saving}
              className="rounded p-1 text-text-tertiary hover:bg-surface-2 dark:hover:bg-gray-700"
              aria-label={col.read_only ? 'Make editable' : 'Make read-only'}
              title={col.read_only ? 'Read-only' : 'Editable'}
            >
              {col.read_only ? <Lock className="h-4 w-4" /> : <Unlock className="h-4 w-4" />}
            </button>
            <button
              type="button"
              onClick={onStartEdit}
              disabled={saving}
              className="rounded p-1 text-text-tertiary hover:bg-surface-2 dark:hover:bg-gray-700"
              aria-label="Rename column"
            >
              <Pencil className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={onDelete}
              disabled={saving}
              className="rounded p-1 text-accent-danger hover:bg-accent-danger/10 dark:hover:bg-red-900/30"
              aria-label="Delete column"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </>
        )}
      </div>
    </li>
  );
}
