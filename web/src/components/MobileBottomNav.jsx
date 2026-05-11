import React from 'react';
import { NavLink } from 'react-router-dom';
import { Home, BookOpen, Inbox, Calendar, User } from 'lucide-react';

/**
 * MobileBottomNav
 *
 * Thumb-zone bottom navigation bar for screens < md.
 * - Five primary destinations with 56x56 hit targets.
 * - Active state via NavLink + a 2px top accent line.
 * - Optional notificationCount renders a red dot on the Inbox item.
 * - Honors prefers-reduced-motion (no bouncy transitions).
 */
const ITEMS = [
  { to: '/', label: 'Home', Icon: Home, end: true },
  { to: '/courses', label: 'Courses', Icon: BookOpen },
  { to: '/inbox', label: 'Inbox', Icon: Inbox, key: 'inbox' },
  { to: '/calendar', label: 'Calendar', Icon: Calendar },
  { to: '/account', label: 'Account', Icon: User },
];

const MobileBottomNav = ({ notificationCount = 0 }) => {
  return (
    <nav
      role="navigation"
      aria-label="Primary"
      className="fixed inset-x-0 bottom-0 z-40 h-16 bg-surface-0 border-t border-border-default shadow-md md:hidden"
    >
      <ul className="flex h-full items-stretch justify-around px-1">
        {ITEMS.map(({ to, label, Icon, end, key }) => {
          const showDot = key === 'inbox' && notificationCount > 0;
          return (
            <li key={to} className="flex-1">
              <NavLink
                to={to}
                end={end}
                aria-label={
                  showDot
                    ? `${label} (${notificationCount} unread)`
                    : label
                }
                className={({ isActive }) =>
                  [
                    'relative mx-auto flex h-14 w-14 flex-col items-center justify-center',
                    'rounded-md select-none',
                    'transition-colors duration-base ease-emphatic',
                    'motion-reduce:transition-none',
                    'focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500',
                    isActive ? 'text-brand-600' : 'text-text-tertiary',
                  ].join(' ')
                }
              >
                {({ isActive }) => (
                  <>
                    {isActive && (
                      <span
                        aria-hidden="true"
                        className="absolute top-0 start-1/2 -translate-x-1/2 h-[2px] w-8 rounded-b-full bg-brand-600"
                      />
                    )}
                    <span className="relative">
                      <Icon className="h-6 w-6" aria-hidden="true" />
                      {showDot && (
                        <span
                          aria-hidden="true"
                          className="absolute -top-0.5 -end-0.5 h-2 w-2 rounded-full bg-accent-danger ring-2 ring-surface-0"
                        />
                      )}
                    </span>
                    <span className="text-[10px] mt-0.5 font-medium leading-none">
                      {label}
                    </span>
                  </>
                )}
              </NavLink>
            </li>
          );
        })}
      </ul>
    </nav>
  );
};

export default MobileBottomNav;
