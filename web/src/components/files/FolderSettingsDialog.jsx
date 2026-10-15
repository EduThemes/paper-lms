import React, { useState } from 'react';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { moveTargets } from '../../utils/fileTree';

const inputClass = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-0';

// FolderSettingsDialog renames, moves, publishes and hides a folder
// (G070 / G071). The folder itself and its subfolders are left out of the
// move targets.
export default function FolderSettingsDialog({ folder, tree, open, onOpenChange, onSave, saving, error }) {
  const [form, setForm] = useState(() => ({
    name: folder.name || '',
    parent: folder.parent_folder_id && folder.parent_folder_id !== tree.root?.id ? String(folder.parent_folder_id) : '',
    published: !folder.locked,
    hidden: Boolean(folder.hidden),
  }));
  const set = (key) => (e) => setForm((prev) => ({ ...prev, [key]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogTitle>Folder settings</DialogTitle>
        <form
          aria-label="Folder settings"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            onSave({
              name: form.name,
              parent_folder_id: form.parent ? Number(form.parent) : 'root',
              locked: !form.published,
              hidden: form.hidden,
            });
          }}
        >
          <div>
            <label htmlFor="folder-name" className="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input id="folder-name" required value={form.name} onChange={set('name')} className={inputClass} />
          </div>
          <div>
            <label htmlFor="folder-parent" className="block text-sm font-medium text-text-secondary mb-1">Inside</label>
            <select id="folder-parent" value={form.parent} onChange={set('parent')} className={inputClass}>
              {moveTargets(tree, folder.id).map((t) => (
                <option key={t.value ?? 'root'} value={t.value ?? ''}>{'  '.repeat(t.depth)}{t.label}</option>
              ))}
            </select>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={form.published} onChange={set('published')} /> Published
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={form.hidden} onChange={set('hidden')} /> Hidden from the file list
          </label>
          {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => onOpenChange(false)} className="px-4 py-2 text-sm text-text-secondary">Cancel</button>
            <button type="submit" disabled={saving} className="px-4 py-2 bg-brand-600 text-on-brand rounded-md text-sm font-medium disabled:opacity-50">
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
