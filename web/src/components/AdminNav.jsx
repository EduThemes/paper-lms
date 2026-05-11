import React, { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  Home, BookOpen, Users, Flag, Sliders, MoreHorizontal,
  FileText, GraduationCap, ClipboardCheck, Upload, KeyRound,
  Shield, UserCog, RefreshCw, Key, Bell, Code,
} from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';

// The five admin pages teachers/admins reach daily live in the sidebar's
// top section. Everything else — compliance reports, identity providers,
// data sync, raw GraphiQL — is rare enough that it's hidden behind a
// "More…" popover so the sidebar stays scannable. Mirrors how Canvas tucks
// account-level admin behind their own sub-navigation.
const frequentLinks = [
  { to: '/admin',                icon: Home,     label: 'Home' },
  { to: '/admin/courses',        icon: BookOpen, label: 'Courses' },
  { to: '/admin/people',         icon: Users,    label: 'People' },
  { to: '/admin/settings',       icon: Sliders,  label: 'Settings' },
  { to: '/admin/feature_flags',  icon: Flag,     label: 'Feature Flags' },
];

const secondaryGroups = [
  {
    label: 'Compliance',
    items: [
      { to: '/admin/ferpa',           icon: FileText,       label: 'FERPA' },
      { to: '/admin/roles',           icon: UserCog,        label: 'Custom Roles' },
    ],
  },
  {
    label: 'Identity',
    items: [
      { to: '/admin/auth_providers',  icon: Shield,         label: 'Auth Providers' },
      { to: '/admin/developer_keys',  icon: KeyRound,       label: 'Developer Keys' },
      { to: '/settings/tokens',       icon: Key,            label: 'Access Tokens' },
    ],
  },
  {
    label: 'Data',
    items: [
      { to: '/admin/sis_import',      icon: Upload,         label: 'SIS Import' },
      { to: '/admin/oneroster',       icon: RefreshCw,      label: 'OneRoster' },
      { to: '/admin/terms',           icon: GraduationCap,  label: 'Terms' },
      { to: '/admin/grading_periods', icon: ClipboardCheck, label: 'Grading Periods' },
    ],
  },
  {
    label: 'Developer',
    items: [
      { to: '/graphiql',              icon: Code,           label: 'GraphiQL' },
      { to: '/settings/notifications', icon: Bell,          label: 'Notifications' },
    ],
  },
];

const allSecondaryRoutes = secondaryGroups.flatMap((g) => g.items.map((i) => i.to));

const AdminNav = () => {
  const location = useLocation();
  const [moreOpen, setMoreOpen] = useState(false);

  const isActive = (path) => location.pathname === path;
  const isInSecondary = allSecondaryRoutes.some((r) => location.pathname.startsWith(r));

  const linkClasses = (active) =>
    `flex items-center gap-3 px-4 py-2 text-sm transition-colors
      ${active
        ? 'border-l-[3px] border-brand-600 bg-brand-50 text-brand-700 font-semibold'
        : 'border-l-[3px] border-transparent text-text-secondary hover:bg-surface-1 hover:text-text-primary'
      }
    `;

  return (
    <aside
      className="fixed inset-y-0 left-16 z-20 w-[216px] bg-surface-0 border-r border-border-default overflow-y-auto"
      role="navigation"
      aria-label="Admin navigation"
    >
      <div className="px-4 py-4 border-b border-border-default">
        <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wider">Admin</h2>
      </div>
      <nav className="py-2">
        {frequentLinks.map(({ to, icon: Icon, label }) => (
          <Link key={to} to={to} className={linkClasses(isActive(to))}>
            <Icon className="w-4 h-4 flex-shrink-0" />
            <span>{label}</span>
          </Link>
        ))}

        <Popover open={moreOpen} onOpenChange={setMoreOpen}>
          <PopoverTrigger asChild>
            <button
              type="button"
              className={linkClasses(isInSecondary || moreOpen) + ' w-full text-left'}
              aria-label="More admin tools"
            >
              <MoreHorizontal className="w-4 h-4 flex-shrink-0" />
              <span>More…</span>
            </button>
          </PopoverTrigger>
          <PopoverContent
            side="right"
            align="start"
            sideOffset={8}
            className="w-64 max-w-[calc(100vw-2rem)] p-0 bg-surface-0 border border-border-default shadow-lg"
          >
            <div className="py-2">
              {secondaryGroups.map((group, idx) => (
                <div key={group.label} className={idx > 0 ? 'border-t border-border-default mt-1 pt-1' : ''}>
                  <div className="px-4 py-1 text-xs font-semibold text-text-tertiary uppercase tracking-wider">
                    {group.label}
                  </div>
                  {group.items.map(({ to, icon: Icon, label }) => (
                    <Link
                      key={to}
                      to={to}
                      onClick={() => setMoreOpen(false)}
                      className={`flex items-center gap-3 px-4 py-2 text-sm transition-colors ${
                        isActive(to)
                          ? 'bg-brand-50 text-brand-700 font-semibold'
                          : 'text-text-secondary hover:bg-surface-1 hover:text-text-primary'
                      }`}
                    >
                      <Icon className="w-4 h-4 flex-shrink-0" />
                      <span>{label}</span>
                    </Link>
                  ))}
                </div>
              ))}
            </div>
          </PopoverContent>
        </Popover>
      </nav>
    </aside>
  );
};

export default AdminNav;
