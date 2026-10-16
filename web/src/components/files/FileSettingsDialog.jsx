import React, { useState } from 'react';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { isoToDateTimeLocal, dateTimeLocalToISO } from '../../utils/datetimeLocal';
import { moveTargets, USAGE_RIGHTS_OPTIONS, CC_LICENSES } from '../../utils/fileTree';

const inputClass = 'w-full border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-0';

// FileSettingsDialog edits one course file (G070 / G071): name, folder,
// publish state, hidden, availability window, usage rights, and replacing
// its content. onSave receives the PUT body; onReplace a File.
export default function FileSettingsDialog({ file, tree, open, onOpenChange, onSave, onReplace, saving, error }) {
  const [form, setForm] = useState(() => ({
    display_name: file.display_name || '',
    folder: file.folder_id && file.folder_id !== tree.root?.id ? String(file.folder_id) : '',
    published: !file.locked,
    hidden: Boolean(file.hidden),
    unlock_at: isoToDateTimeLocal(file.unlock_at),
    lock_at: isoToDateTimeLocal(file.lock_at),
    use_justification: file.usage_rights?.use_justification || '',
    legal_copyright: file.usage_rights?.legal_copyright || '',
    license: file.usage_rights?.license || 'cc_by',
  }));
  const set = (key) => (e) => setForm((prev) => ({ ...prev, [key]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));

  const submit = (e) => {
    e.preventDefault();
    onSave({
      display_name: form.display_name,
      parent_folder_id: form.folder ? Number(form.folder) : 'root',
      locked: !form.published,
      hidden: form.hidden,
      unlock_at: form.unlock_at ? dateTimeLocalToISO(form.unlock_at) : null,
      lock_at: form.lock_at ? dateTimeLocalToISO(form.lock_at) : null,
      usage_rights: {
        use_justification: form.use_justification,
        legal_copyright: form.legal_copyright,
        license: form.use_justification === 'creative_commons' ? form.license : '',
      },
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogTitle>File settings</DialogTitle>
        <form onSubmit={submit} className="space-y-4" aria-label="File settings">
          <div>
            <label htmlFor="file-name" className="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input id="file-name" required value={form.display_name} onChange={set('display_name')} className={inputClass} />
          </div>
          <div>
            <label htmlFor="file-folder" className="block text-sm font-medium text-text-secondary mb-1">Folder</label>
            <select id="file-folder" value={form.folder} onChange={set('folder')} className={inputClass}>
              {moveTargets(tree).map((t) => (
                <option key={t.value ?? 'root'} value={t.value ?? ''}>{'  '.repeat(t.depth)}{t.label}</option>
              ))}
            </select>
          </div>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-text-secondary">Student access</legend>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={form.published} onChange={set('published')} /> Published
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={form.hidden} onChange={set('hidden')} /> Hidden: only students with the link can open it
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label htmlFor="file-unlock" className="block text-xs text-text-tertiary mb-1">Available from</label>
                <input id="file-unlock" type="datetime-local" value={form.unlock_at} onChange={set('unlock_at')} className={inputClass} />
              </div>
              <div>
                <label htmlFor="file-lock" className="block text-xs text-text-tertiary mb-1">Available until</label>
                <input id="file-lock" type="datetime-local" value={form.lock_at} onChange={set('lock_at')} className={inputClass} />
              </div>
            </div>
          </fieldset>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium text-text-secondary">Usage rights</legend>
            <select aria-label="Usage right" value={form.use_justification} onChange={set('use_justification')} className={inputClass}>
              {USAGE_RIGHTS_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
            {form.use_justification && (
              <input aria-label="Copyright holder" placeholder="Copyright holder" value={form.legal_copyright} onChange={set('legal_copyright')} className={inputClass} />
            )}
            {form.use_justification === 'creative_commons' && (
              <select aria-label="Creative Commons license" value={form.license} onChange={set('license')} className={inputClass}>
                {CC_LICENSES.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            )}
          </fieldset>
          <div>
            <label htmlFor="file-replace" className="block text-sm font-medium text-text-secondary mb-1">Replace content</label>
            <input
              id="file-replace"
              type="file"
              onChange={(e) => { const f = e.target.files?.[0]; if (f) onReplace(f); e.target.value = ''; }}
              className="text-sm"
            />
            <p className="text-xs text-text-tertiary mt-1">Keeps this file&apos;s links and embeds; only the content changes.</p>
          </div>
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
