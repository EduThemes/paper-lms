import React, { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { User, Settings, Shield, KeyRound, Bell, Key, ShieldCheck, LogOut } from 'lucide-react';

// Compact dropdown anchored to the user-icon in the global sidebar.
// All entries route into the account settings hub (or top-level pages
// that exist independently — passkeys/MFA were shipped pre-hub).
export default function UserMenu({ user, onLogout }) {
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e) => {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target)) {
        setOpen(false);
      }
    };
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const close = () => setOpen(false);

  const items = [
    { to: '/profile/settings', icon: Settings, label: 'Account settings' },
    { to: '/profile/settings?tab=password', icon: KeyRound, label: 'Change password' },
    { to: '/profile/settings?tab=security', icon: ShieldCheck, label: 'Two-factor & passkeys' },
    { to: '/settings/tokens', icon: Key, label: 'Access tokens' },
    { to: '/settings/notifications', icon: Bell, label: 'Notification preferences' },
    { to: '/profile/settings?tab=privacy', icon: Shield, label: 'Data & privacy' },
  ];

  return (
    <div ref={wrapperRef} className="relative flex items-center justify-center w-10 h-10">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Account menu"
        className="w-8 h-8 rounded-full bg-text-tertiary hover:ring-2 hover:ring-white/30 focus:ring-2 focus:ring-white/40 focus:outline-none flex items-center justify-center transition"
      >
        <User className="w-4 h-4 text-chrome-sidebar-fg" />
      </button>
      {open && (
        <div
          role="menu"
          className="absolute left-full ml-2 bottom-0 z-50 w-64 rounded-md border border-border-default bg-surface-0 text-text-primary shadow-lg overflow-hidden"
        >
          <div className="px-3 py-2 border-b border-border-default">
            <div className="text-sm font-semibold truncate">{user?.name || 'Account'}</div>
            <div className="text-xs text-text-secondary truncate">{user?.email}</div>
          </div>
          <ul className="py-1">
            {items.map(({ to, icon: Icon, label }) => (
              <li key={to + label}>
                <Link
                  to={to}
                  onClick={close}
                  role="menuitem"
                  className="flex items-center gap-2 px-3 py-2 text-sm hover:bg-surface-1 focus:bg-surface-1 focus:outline-none"
                >
                  <Icon className="w-4 h-4 text-text-secondary" />
                  <span>{label}</span>
                </Link>
              </li>
            ))}
          </ul>
          <div className="border-t border-border-default">
            <button
              type="button"
              onClick={() => { close(); onLogout(); }}
              role="menuitem"
              className="w-full flex items-center gap-2 px-3 py-2 text-sm text-danger hover:bg-surface-1 focus:bg-surface-1 focus:outline-none"
            >
              <LogOut className="w-4 h-4" />
              <span>Log out</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
