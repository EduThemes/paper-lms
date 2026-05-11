import React, { useState } from 'react';
import { Trash2, Plus, GripVertical } from 'lucide-react';
import { api } from '../../services/api';
import IconPicker from './IconPicker';
import ColorPicker from './ColorPicker';

const presetTypes = [
  { type: 'todays_lesson', label: "Today's Lesson", icon: 'Play', color: '#27AE60' },
  { type: 'continue', label: 'Continue Where You Left Off', icon: 'BookOpen', color: '#0374B5' },
  { type: 'my_work', label: 'My Work', icon: 'CheckSquare', color: '#F2994A' },
  { type: 'inbox', label: 'Inbox', icon: 'Inbox', color: '#9B51E0' },
  { type: 'announcements', label: 'Announcements', icon: 'Bell', color: '#EB5757' },
];

const ButtonEditor = ({ courseId, buttons, setButtons }) => {
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState({
    button_type: 'custom', label: '', icon: 'BookOpen', color: '#0374B5',
    link_type: 'auto', link_id: null, link_url: '', visible: true,
  });

  const handleAddPreset = async (preset) => {
    try {
      const { data } = await api.createCourseHomeButton(courseId, {
        button_type: preset.type, label: preset.label, icon: preset.icon,
        color: preset.color, link_type: 'auto', position: buttons.length, visible: true,
      });
      setButtons([...buttons, data]);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  const handleAddCustom = async () => {
    try {
      const { data } = await api.createCourseHomeButton(courseId, {
        ...form, position: buttons.length,
      });
      setButtons([...buttons, data]);
      setForm({ button_type: 'custom', label: '', icon: 'BookOpen', color: '#0374B5', link_type: 'auto', link_id: null, link_url: '', visible: true });
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  const handleDelete = async (id) => {
    try {
      await api.deleteCourseHomeButton(courseId, id);
      setButtons(buttons.filter(b => b.id !== id));
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  const handleUpdate = async (btn) => {
    try {
      const { data } = await api.updateCourseHomeButton(courseId, btn.id, btn);
      setButtons(buttons.map(b => b.id === data.id ? data : b));
      setEditing(null);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  };

  return (
    <div className="space-y-4">
      {/* Current buttons */}
      <div className="space-y-2">
        {buttons.map((btn, idx) => (
          <div key={btn.id} className="flex items-center gap-3 p-3 border rounded-lg bg-surface-1">
            <GripVertical className="w-4 h-4 text-text-disabled cursor-grab" />
            <div className="w-8 h-8 rounded flex items-center justify-center" style={{ backgroundColor: btn.color || '#0374B5' }}>
              <span className="text-white text-xs">{btn.icon?.[0]}</span>
            </div>
            <div className="flex-1">
              <div className="font-medium text-sm">{btn.label || btn.button_type}</div>
              <div className="text-xs text-text-tertiary">{btn.button_type} &middot; {btn.visible ? 'Visible' : 'Hidden'}</div>
            </div>
            <button onClick={() => handleDelete(btn.id)} className="text-accent-danger hover:text-accent-danger p-1">
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        ))}
        {buttons.length === 0 && <p className="text-sm text-text-tertiary">No buttons configured.</p>}
      </div>

      {/* Add preset buttons */}
      <div>
        <h4 className="text-sm font-medium text-text-secondary mb-2">Add Preset Button</h4>
        <div className="flex flex-wrap gap-2">
          {presetTypes.map(preset => (
            <button key={preset.type} onClick={() => handleAddPreset(preset)}
              className="flex items-center gap-1 px-3 py-1.5 border rounded-full text-sm hover:bg-surface-1">
              <Plus className="w-3 h-3" /> {preset.label}
            </button>
          ))}
        </div>
      </div>

      {/* Add custom button */}
      <div className="border-t pt-4">
        <h4 className="text-sm font-medium text-text-secondary mb-2">Add Custom Button</h4>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Label</label>
            <input type="text" className="w-full border rounded px-3 py-1.5 text-sm" value={form.label}
              onChange={e => setForm(f => ({ ...f, label: e.target.value }))} />
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Link URL</label>
            <input type="text" className="w-full border rounded px-3 py-1.5 text-sm" value={form.link_url}
              placeholder="https://... or /courses/..." onChange={e => setForm(f => ({ ...f, link_url: e.target.value, link_type: 'external_url' }))} />
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Icon</label>
            <IconPicker value={form.icon} onChange={icon => setForm(f => ({ ...f, icon }))} />
          </div>
          <div>
            <label className="block text-xs text-text-tertiary mb-1">Color</label>
            <ColorPicker value={form.color} onChange={color => setForm(f => ({ ...f, color }))} />
          </div>
        </div>
        <button onClick={handleAddCustom} className="mt-3 flex items-center gap-1 bg-brand-600 text-white px-4 py-1.5 rounded text-sm hover:bg-brand-700">
          <Plus className="w-4 h-4" /> Add Custom Button
        </button>
      </div>
    </div>
  );
};

export default ButtonEditor;
