import React, { useState, useEffect, useRef, useCallback } from 'react';
import { X, Lock, ListChecks, Settings2 } from 'lucide-react';
import { api } from '../services/api';
import useDismissable from '../hooks/useDismissable';
import FocusTrap from './FocusTrap';

const COMPLETION_TYPES = {
  Assignment: [
    { value: '', label: 'No requirement' },
    { value: 'must_submit', label: 'Must submit' },
    { value: 'min_score', label: 'Must score at least' },
    { value: 'must_mark_done', label: 'Must mark as done' },
  ],
  Quiz: [
    { value: '', label: 'No requirement' },
    { value: 'must_submit', label: 'Must submit' },
    { value: 'min_score', label: 'Must score at least' },
    { value: 'must_mark_done', label: 'Must mark as done' },
  ],
  Page: [
    { value: '', label: 'No requirement' },
    { value: 'must_view', label: 'Must view' },
    { value: 'must_mark_done', label: 'Must mark as done' },
  ],
  Discussion: [
    { value: '', label: 'No requirement' },
    { value: 'must_contribute', label: 'Must contribute' },
    { value: 'must_view', label: 'Must view' },
    { value: 'must_mark_done', label: 'Must mark as done' },
  ],
  ExternalUrl: [
    { value: '', label: 'No requirement' },
    { value: 'must_view', label: 'Must view' },
    { value: 'must_mark_done', label: 'Must mark as done' },
  ],
  SubHeader: [],
};

const TABS = [
  { id: 'settings', label: 'Settings', icon: Settings2 },
  { id: 'prerequisites', label: 'Prerequisites', icon: Lock },
  { id: 'requirements', label: 'Requirements', icon: ListChecks },
];

const ModuleSettingsModal = ({ courseId, module, modules, prerequisites, onClose, onSave }) => {
  const [activeTab, setActiveTab] = useState('settings');
  const [requireSequential, setRequireSequential] = useState(module.require_sequential_progress || false);
  const [prereqIds, setPrereqIds] = useState(prerequisites || []);
  const [itemRequirements, setItemRequirements] = useState({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const modalRef = useRef(null);
  const previouslyFocusedRef = useRef(null);
  const tabRefs = useRef({});

  // Escape + outside-click dismissal. The overlay below also calls onClose
  // directly via onClick; both routes converge to the same handler.
  useDismissable(modalRef, true, onClose);

  // Save the previously-focused element on mount; restore focus to it on unmount.
  useEffect(() => {
    previouslyFocusedRef.current = document.activeElement;
    return () => {
      const prev = previouslyFocusedRef.current;
      if (prev && typeof prev.focus === 'function') {
        try { prev.focus(); } catch (e) { /* element may be gone */ }
      }
    };
  }, []);

  // Arrow-key navigation between tabs (per WAI-ARIA Authoring Practices for tabs).
  const handleTabKeyDown = useCallback((e, index) => {
    const ids = TABS.map(t => t.id);
    let nextIndex = null;
    if (e.key === 'ArrowRight') {
      nextIndex = (index + 1) % ids.length;
    } else if (e.key === 'ArrowLeft') {
      nextIndex = (index - 1 + ids.length) % ids.length;
    } else if (e.key === 'Home') {
      nextIndex = 0;
    } else if (e.key === 'End') {
      nextIndex = ids.length - 1;
    }
    if (nextIndex !== null) {
      e.preventDefault();
      const nextId = ids[nextIndex];
      setActiveTab(nextId);
      const nextEl = tabRefs.current[nextId];
      if (nextEl && typeof nextEl.focus === 'function') nextEl.focus();
    }
  }, []);

  // Initialize item requirements from module items
  useEffect(() => {
    const reqs = {};
    (module.items || []).forEach(item => {
      reqs[item.id] = {
        completion_type: item.completion_type || '',
        min_score: item.min_score ?? '',
      };
    });
    setItemRequirements(reqs);
  }, [module]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      // Save module settings (sequential progress)
      await api.updateModule(courseId, module.id, {
        require_sequential_progress: requireSequential,
      });

      // Save prerequisites
      await api.setModulePrerequisites(courseId, module.id, prereqIds);

      // Save item requirements
      const items = module.items || [];
      await Promise.all(
        items.map(item => {
          const req = itemRequirements[item.id];
          if (!req) return null;
          const changed =
            (req.completion_type || '') !== (item.completion_type || '') ||
            (req.min_score ?? '') !== (item.min_score ?? '');
          if (!changed) return null;
          const payload = { completion_type: req.completion_type };
          if (req.completion_type === 'min_score' && req.min_score !== '') {
            payload.min_score = parseFloat(req.min_score);
          }
          return api.updateModuleItem(courseId, module.id, item.id, payload);
        }).filter(Boolean)
      );

      onSave({
        prereqIds,
        requireSequential,
        itemRequirements,
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const addPrereq = (id) => {
    if (id && !prereqIds.includes(id)) {
      setPrereqIds([...prereqIds, id]);
    }
  };

  const removePrereq = (id) => {
    setPrereqIds(prereqIds.filter(p => p !== id));
  };

  const updateItemReq = (itemId, field, value) => {
    setItemRequirements(prev => ({
      ...prev,
      [itemId]: { ...prev[itemId], [field]: value },
    }));
  };

  const availablePrereqs = modules.filter(
    m => m.id !== module.id && !prereqIds.includes(m.id)
  );

  const requireableItems = (module.items || []).filter(
    item => item.type !== 'SubHeader'
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <FocusTrap ariaLabelledBy="module-settings-title">
        <div ref={modalRef} className="relative bg-surface-0 rounded-lg shadow-xl w-full max-w-2xl max-h-[85vh] flex flex-col z-10">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b">
          <h2 id="module-settings-title" className="text-lg font-semibold text-text-primary">
            Module Settings — {module.name}
          </h2>
          <button
            onClick={onClose}
            className="p-1 text-text-secondary hover:text-text-secondary rounded"
            aria-label="Close module settings"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Tabs */}
        <div className="border-b px-6">
          <div role="tablist" aria-label="Module settings sections" className="flex gap-0 -mb-px">
            {TABS.map((tab, index) => {
              const selected = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  ref={el => { tabRefs.current[tab.id] = el; }}
                  id={`module-settings-tab-${tab.id}`}
                  type="button"
                  role="tab"
                  aria-selected={selected}
                  aria-controls={`module-settings-panel-${tab.id}`}
                  tabIndex={selected ? 0 : -1}
                  onClick={() => setActiveTab(tab.id)}
                  onKeyDown={(e) => handleTabKeyDown(e, index)}
                  className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                    selected
                      ? 'border-brand-600 text-brand-600'
                      : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
                  }`}
                >
                  <tab.icon className="w-4 h-4" />
                  {tab.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto px-6 py-5">
          {activeTab === 'settings' && (
            <div
              role="tabpanel"
              id="module-settings-panel-settings"
              aria-labelledby="module-settings-tab-settings"
              className="space-y-4"
            >
              <label className="flex items-start gap-3 p-4 border rounded-lg hover:bg-surface-1 cursor-pointer">
                <input
                  type="checkbox"
                  checked={requireSequential}
                  onChange={e => setRequireSequential(e.target.checked)}
                  className="mt-0.5 rounded border-border-strong"
                />
                <div>
                  <div className="font-medium text-text-primary">Require students to complete items in the listed order</div>
                  <div className="text-sm text-text-tertiary mt-1">
                    Students will only be able to access the next module item after completing the previous one.
                  </div>
                </div>
              </label>
            </div>
          )}

          {activeTab === 'prerequisites' && (
            <div
              role="tabpanel"
              id="module-settings-panel-prerequisites"
              aria-labelledby="module-settings-tab-prerequisites"
              className="space-y-4"
            >
              <p className="text-sm text-text-tertiary">
                Students must complete these modules before they can access this one.
              </p>

              {prereqIds.length === 0 ? (
                <div className="text-sm text-text-secondary italic py-4 text-center">
                  No prerequisites set
                </div>
              ) : (
                <div className="space-y-2">
                  {prereqIds.map(id => {
                    const mod = modules.find(m => m.id === id);
                    return (
                      <div key={id} className="flex items-center justify-between bg-brand-50 border border-blue-200 rounded-lg px-4 py-2.5">
                        <div className="flex items-center gap-2">
                          <Lock className="w-4 h-4 text-brand-500" />
                          <span className="text-sm font-medium text-blue-900">
                            {mod ? mod.name : `Module ${id}`}
                          </span>
                        </div>
                        <button
                          onClick={() => removePrereq(id)}
                          className="p-1 text-blue-400 hover:text-accent-danger rounded"
                          title="Remove prerequisite"
                          aria-label={`Remove prerequisite ${mod ? mod.name : `Module ${id}`}`}
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}

              {availablePrereqs.length > 0 && (
                <select
                  className="w-full border border-border-strong rounded-lg px-3 py-2 text-sm bg-surface-0 focus:outline-none focus:ring-2 focus:ring-brand-500"
                  value=""
                  onChange={e => addPrereq(parseInt(e.target.value, 10))}
                  aria-label="Add prerequisite module"
                >
                  <option value="">+ Add prerequisite module...</option>
                  {availablePrereqs.map(m => (
                    <option key={m.id} value={m.id}>{m.name}</option>
                  ))}
                </select>
              )}
            </div>
          )}

          {activeTab === 'requirements' && (
            <div
              role="tabpanel"
              id="module-settings-panel-requirements"
              aria-labelledby="module-settings-tab-requirements"
              className="space-y-4"
            >
              <p className="text-sm text-text-tertiary">
                Set completion requirements for individual items. Students must fulfill these to complete the module.
              </p>

              {requireableItems.length === 0 ? (
                <div className="text-sm text-text-secondary italic py-4 text-center">
                  No items to set requirements on. Add items to this module first.
                </div>
              ) : (
                <div className="space-y-3">
                  {requireableItems.map(item => {
                    const req = itemRequirements[item.id] || { completion_type: '', min_score: '' };
                    const typeOptions = COMPLETION_TYPES[item.type] || COMPLETION_TYPES.Page;
                    return (
                      <div key={item.id} className="border rounded-lg p-3">
                        <div className="flex items-center justify-between gap-3">
                          <span className="text-sm font-medium text-text-primary truncate flex-1">
                            {item.title}
                          </span>
                          <span className="text-xs text-text-secondary flex-shrink-0">{item.type}</span>
                        </div>
                        <div className="mt-2 flex items-center gap-3">
                          <select
                            className="flex-1 border border-border-strong rounded px-2 py-1.5 text-sm bg-surface-0 focus:outline-none focus:ring-1 focus:ring-brand-500"
                            value={req.completion_type}
                            onChange={e => updateItemReq(item.id, 'completion_type', e.target.value)}
                            aria-label={`Completion requirement for ${item.title}`}
                          >
                            {typeOptions.map(opt => (
                              <option key={opt.value} value={opt.value}>{opt.label}</option>
                            ))}
                          </select>
                          {req.completion_type === 'min_score' && (
                            <div className="flex items-center gap-1">
                              <input
                                type="number"
                                min="0"
                                max="100"
                                step="1"
                                className="w-20 border border-border-strong rounded px-2 py-1.5 text-sm text-center focus:outline-none focus:ring-1 focus:ring-brand-500"
                                value={req.min_score}
                                onChange={e => updateItemReq(item.id, 'min_score', e.target.value)}
                                placeholder="Score"
                                aria-label={`Minimum score percentage for ${item.title}`}
                              />
                              <span className="text-sm text-text-tertiary">%</span>
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="border-t px-6 py-4 flex items-center justify-between">
          <div>
            {error && <span className="text-sm text-accent-danger" role="alert">{error}</span>}
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm text-text-secondary hover:bg-surface-2 rounded-md"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 text-sm font-medium text-white bg-brand-600 rounded-md hover:bg-brand-700 disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
        </div>
      </FocusTrap>
    </div>
  );
};

export default ModuleSettingsModal;
