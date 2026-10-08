import React, { useEffect, useRef, useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import { ACTION_LABELS } from '../../utils/enrollmentLifecycle';

/**
 * Per-row "Manage" menu on the People page. `actions` comes from
 * enrollmentActions(); `onAction(action)` runs it.
 */
export default function EnrollmentActionsMenu({ actions, personName, onAction, disabled }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);

  useEffect(() => {
    if (!open) return undefined;
    const close = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setOpen(false);
    };
    const onKey = (e) => { if (e.key === 'Escape') setOpen(false); };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  if (!actions || actions.length === 0) return null;

  return (
    <div className="relative inline-block" ref={ref}>
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Manage ${personName}`}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="p-1.5 rounded-md text-text-secondary hover:bg-surface-2 disabled:opacity-50"
      >
        <MoreHorizontal className="w-4 h-4" />
      </button>
      {open && (
        <ul role="menu" className="absolute end-0 z-20 mt-1 w-56 rounded-md border border-border-default bg-surface-0 py-1 shadow-lg">
          {actions.map((action) => (
            <li key={action} role="none">
              <button
                type="button"
                role="menuitem"
                onClick={() => { setOpen(false); onAction(action); }}
                className={`w-full px-3 py-2 text-start text-sm hover:bg-surface-1 ${action === 'delete' ? 'text-accent-danger' : 'text-text-primary'}`}
              >
                {ACTION_LABELS[action]}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
