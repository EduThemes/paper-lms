import React, { useState } from 'react';
import { Trash2, Plus } from 'lucide-react';
import { api } from '../../services/api';

const OverrideEditor = ({ courseId, overrides, setOverrides }) => {
  const [form, setForm] = useState({ date: '', link_type: 'module', link_id: '', link_url: '', label: '' });

  const handleAdd = async () => {
    if (!form.date) return;
    try {
      const payload = {
        ...form,
        link_id: form.link_id ? parseInt(form.link_id, 10) : null,
      };
      const { data } = await api.createTodaysLessonOverride(courseId, payload);
      setOverrides([...overrides, data]);
      setForm({ date: '', link_type: 'module', link_id: '', link_url: '', label: '' });
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  const handleDelete = async (id) => {
    try {
      await api.deleteTodaysLessonOverride(courseId, id);
      setOverrides(overrides.filter(o => o.id !== id));
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        {overrides.map(ov => (
          <div key={ov.id} className="flex items-center gap-3 p-3 border rounded-lg bg-surface-1">
            <div className="flex-1">
              <div className="font-medium text-sm">{ov.date?.split('T')[0] || ov.date}</div>
              <div className="text-xs text-text-tertiary">{ov.label || ov.link_type} {ov.link_id ? `#${ov.link_id}` : ''}</div>
            </div>
            <button onClick={() => handleDelete(ov.id)} className="text-accent-danger hover:text-accent-danger p-1">
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        ))}
        {overrides.length === 0 && <p className="text-sm text-text-tertiary">No overrides configured.</p>}
      </div>

      <div className="border-t pt-4">
        <h4 className="text-sm font-medium text-text-secondary mb-2">Add Override</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Date</label>
            <input type="date" className="w-full border rounded px-3 py-1.5 text-sm" value={form.date}
              onChange={e => setForm(f => ({ ...f, date: e.target.value }))} />
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Label</label>
            <input type="text" className="w-full border rounded px-3 py-1.5 text-sm" value={form.label}
              placeholder="e.g. Unit 3 Review" onChange={e => setForm(f => ({ ...f, label: e.target.value }))} />
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Link Type</label>
            <select className="w-full border rounded px-3 py-1.5 text-sm" value={form.link_type}
              onChange={e => setForm(f => ({ ...f, link_type: e.target.value }))}>
              <option value="module">Module</option>
              <option value="page">Page</option>
              <option value="assignment">Assignment</option>
              <option value="discussion">Discussion</option>
              <option value="external_url">External URL</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">{form.link_type === 'external_url' ? 'URL' : 'Content ID'}</label>
            {form.link_type === 'external_url' ? (
              <input type="text" className="w-full border rounded px-3 py-1.5 text-sm" value={form.link_url}
                onChange={e => setForm(f => ({ ...f, link_url: e.target.value }))} />
            ) : (
              <input type="number" className="w-full border rounded px-3 py-1.5 text-sm" value={form.link_id}
                onChange={e => setForm(f => ({ ...f, link_id: e.target.value }))} />
            )}
          </div>
        </div>
        <button onClick={handleAdd} className="mt-3 flex items-center gap-1 bg-brand-600 text-white px-4 py-1.5 rounded text-sm hover:bg-brand-700">
          <Plus className="w-4 h-4" /> Add Override
        </button>
      </div>
    </div>
  );
};

export default OverrideEditor;
