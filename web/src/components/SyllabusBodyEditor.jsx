import React, { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Pencil, Save, X } from 'lucide-react';
import { api } from '../services/api';
import RichContentEditorV2 from './rce/RichContentEditorV2';
import { clearRceDraft } from './rce/rceDrafts';
import useUnsavedChanges from '../hooks/useUnsavedChanges';

export const syllabusDraftKey = (courseId) => `syllabus-${courseId}-body`;

// SyllabusBodyEditor lets course staff write the syllabus description in
// the rich content editor (G075). It saves syllabus_body through
// PUT /courses/:id (instructor-only; the server sanitizes the HTML) and
// reports the stored value back through onSaved.
export default function SyllabusBodyEditor({ courseId, body, onSaved }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(body || '');
  const [dirty, setDirty] = useState(false);
  useUnsavedChanges(editing && dirty);

  const save = useMutation({
    mutationFn: (html) => api.updateCourse(courseId, { syllabus_body: html }),
    onSuccess: (course) => {
      clearRceDraft(syllabusDraftKey(courseId));
      setDirty(false);
      setEditing(false);
      // Show what the server stored (it sanitizes), not the local HTML.
      onSaved?.(course && typeof course.syllabus_body === 'string' ? course.syllabus_body : draft);
    },
  });

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => { setDraft(body || ''); setDirty(false); setEditing(true); }}
        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default print:hidden"
      >
        <Pencil className="w-4 h-4" aria-hidden="true" />
        {body ? 'Edit syllabus' : 'Add syllabus description'}
      </button>
    );
  }

  return (
    <form
      className="space-y-3"
      aria-label="Edit syllabus"
      onSubmit={(e) => { e.preventDefault(); save.mutate(draft); }}
    >
      <RichContentEditorV2
        value={draft}
        onChange={(html) => { setDraft(html); setDirty(true); }}
        placeholder="Course description, policies, schedule…"
        minHeight="280px"
        courseId={courseId}
        autoSaveKey={syllabusDraftKey(courseId)}
      />
      {save.isError && <p role="alert" className="text-sm text-accent-danger">{save.error.message}</p>}
      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={() => { setEditing(false); setDirty(false); }}
          className="inline-flex items-center gap-1 px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
        >
          <X className="w-4 h-4" aria-hidden="true" /> Cancel
        </button>
        <button
          type="submit"
          disabled={save.isPending}
          className="inline-flex items-center gap-1.5 px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          <Save className="w-4 h-4" aria-hidden="true" />
          {save.isPending ? 'Saving…' : 'Save syllabus'}
        </button>
      </div>
    </form>
  );
}
