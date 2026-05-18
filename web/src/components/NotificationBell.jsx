import React, { useState, useEffect, useRef, useCallback, useId } from 'react';
import { Bell, BellOff, Check, CheckCheck, Moon, X } from 'lucide-react';
import { Link } from 'react-router-dom';
import { api } from '../services/api';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import FocusTrap from './FocusTrap';
import { useLiveRegion } from './LiveRegion';
import { useClickOutside } from '../hooks/useClickOutside';

const NOTIFICATION_ICONS = {
  submission_grade: '📝',
  new_message: '💬',
  new_announcement: '📢',
  event_start: '📅',
  enrollment: '👤',
  discussion_reply: '💬',
  submission_comment: '💬',
};

const POLL_INTERVAL = 60000; // 1 minute
const DND_KEY = 'paper.notifications.dnd';

// DND helpers — value is either falsy (off), "on" (on indefinitely),
// or an ISO timestamp string indicating the moment DND expires.
const isDndActive = () => {
  try {
    const raw = localStorage.getItem(DND_KEY);
    if (!raw) return false;
    if (raw === 'on') return true;
    const ts = Date.parse(raw);
    if (Number.isNaN(ts)) return false;
    if (Date.now() < ts) return true;
    // Expired — clear it
    localStorage.removeItem(DND_KEY);
    return false;
  } catch {
    return false;
  }
};

// Returns the next "tomorrow morning" (8am local time) as ISO string.
const tomorrowMorningIso = () => {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  d.setHours(8, 0, 0, 0);
  return d.toISOString();
};

const NotificationBell = ({ position = 'sidebar' }) => {
  const [notifications, setNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [isOpen, setIsOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [dndActive, setDndActive] = useState(() => isDndActive());

  const containerRef = useRef(null);
  const triggerRef = useRef(null);
  const previouslyFocusedRef = useRef(null);
  const dialogId = useId();

  const { announce } = useLiveRegion();

  const fetchUnreadCount = useCallback(async () => {
    try {
      const result = await api.getNotifications(1, 1, true);
      const total = parseInt(result.pagination?.totalCount || '0', 10);
      setUnreadCount(total);
    } catch {
      // Silently fail — notifications are non-critical
    }
  }, []);

  const fetchNotifications = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.getNotifications(1, 20, false);
      setNotifications(result.data || []);
      const unread = (result.data || []).filter((n) => !n.is_read).length;
      setUnreadCount((prev) => Math.max(prev, unread));
    } catch {
      // Silently fail
    } finally {
      setLoading(false);
    }
  }, []);

  // Poll for unread count — pause when tab is hidden (Page Visibility API).
  useEffect(() => {
    let interval = null;

    const start = () => {
      if (interval) return;
      // Refresh immediately when becoming visible / on mount.
      fetchUnreadCount();
      interval = setInterval(fetchUnreadCount, POLL_INTERVAL);
    };
    const stop = () => {
      if (interval) {
        clearInterval(interval);
        interval = null;
      }
    };

    const handleVisibility = () => {
      if (document.hidden) stop();
      else start();
    };

    if (!document.hidden) start();
    document.addEventListener('visibilitychange', handleVisibility);

    return () => {
      stop();
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  }, [fetchUnreadCount]);

  // Re-evaluate DND on mount + when storage changes in another tab + on
  // visibility change (so an expired window is cleared).
  useEffect(() => {
    const recheck = () => setDndActive(isDndActive());
    recheck();
    window.addEventListener('storage', recheck);
    document.addEventListener('visibilitychange', recheck);
    const interval = setInterval(recheck, 60000);
    return () => {
      window.removeEventListener('storage', recheck);
      document.removeEventListener('visibilitychange', recheck);
      clearInterval(interval);
    };
  }, []);

  // Fetch full list when dropdown opens
  useEffect(() => {
    if (isOpen) fetchNotifications();
  }, [isOpen, fetchNotifications]);

  // Click outside / Escape dismissal (skipped for mobile-sheet — it has its
  // own backdrop + FocusTrap handles Escape). Hook attaches a no-op when
  // dismissal shouldn't apply.
  const handleOutside = useCallback(() => {
    if (isOpen && position !== 'mobile-sheet') setIsOpen(false);
  }, [isOpen, position]);
  useClickOutside(containerRef, handleOutside);

  // Open / close lifecycle: track previously focused element + restore on close.
  const open = useCallback(() => {
    previouslyFocusedRef.current = document.activeElement;
    setIsOpen(true);
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
    requestAnimationFrame(() => {
      const target = previouslyFocusedRef.current;
      if (target && typeof target.focus === 'function') {
        target.focus();
      } else if (triggerRef.current) {
        triggerRef.current.focus();
      }
    });
  }, []);

  const toggle = useCallback(() => {
    if (isOpen) close();
    else open();
  }, [isOpen, open, close]);

  const handleMarkAsRead = async (id, e) => {
    if (e) e.stopPropagation();
    try {
      await api.markNotificationAsRead(id);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
      announce('Notification marked as read');
    } catch {
      // Silently fail
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await api.markAllNotificationsAsRead();
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnreadCount(0);
      announce('All notifications marked as read');
    } catch {
      // Silently fail
    }
  };

  const handleToggleDnd = () => {
    try {
      if (dndActive) {
        localStorage.removeItem(DND_KEY);
        setDndActive(false);
        announce('Notifications resumed');
      } else {
        const until = tomorrowMorningIso();
        localStorage.setItem(DND_KEY, until);
        setDndActive(true);
        announce('Notifications paused until tomorrow morning');
      }
    } catch {
      // localStorage may be unavailable (private mode) — no-op
    }
  };

  const getNotificationLink = (n) => {
    if (n.context_type === 'Course' && n.context_id) {
      if (n.notification_type === 'submission_grade') {
        return `/courses/${n.context_id}/grades`;
      }
      if (n.notification_type === 'new_announcement') {
        return `/courses/${n.context_id}/announcements`;
      }
      if (n.notification_type === 'discussion_reply') {
        return `/courses/${n.context_id}/discussions`;
      }
      return `/courses/${n.context_id}`;
    }
    if (n.notification_type === 'new_message') return '/inbox';
    if (n.notification_type === 'event_start') return '/calendar';
    return null;
  };

  const formatTime = (dateStr) => {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    const diffDay = Math.floor(diffHr / 24);
    if (diffDay < 7) return `${diffDay}d ago`;
    return date.toLocaleDateString();
  };

  // Show badge unless DND is on
  const showBadge = unreadCount > 0 && !dndActive;
  const ariaLabel =
    unreadCount > 0
      ? `Notifications, ${unreadCount} unread${dndActive ? ', do not disturb on' : ''}`
      : `Notifications${dndActive ? ', do not disturb on' : ''}`;

  // ----- Panel contents (shared across positions) -----
  const panelBody = (
    <>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
        <h3 className="font-semibold text-text-primary text-sm" id={`${dialogId}-title`}>
          Notifications
        </h3>
        <div className="flex items-center gap-2">
          {unreadCount > 0 && (
            <button
              onClick={handleMarkAllRead}
              className="text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
              title="Mark all as read"
            >
              <CheckCheck className="w-3.5 h-3.5" />
              Mark all read
            </button>
          )}
          <button
            onClick={close}
            className="p-1 text-text-disabled hover:text-text-secondary rounded"
            aria-label="Close notifications"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* DND quick toggle */}
      <div className="px-4 py-2 border-b border-border-subtle bg-surface-1/60">
        <button
          onClick={handleToggleDnd}
          className="w-full flex items-center justify-between gap-2 text-xs text-text-secondary hover:text-text-primary"
        >
          <span className="flex items-center gap-2">
            <Moon className="w-3.5 h-3.5" />
            {dndActive
              ? 'Notifications paused until tomorrow morning'
              : 'Pause notifications until tomorrow morning'}
          </span>
          <span
            className={`text-[11px] font-semibold uppercase tracking-wide ${dndActive ? 'text-accent-warning' : 'text-brand-600'}`}
          >
            {dndActive ? 'Resume' : 'Pause'}
          </span>
        </button>
      </div>

      {/* Notification list */}
      <div className="flex-1 overflow-y-auto">
        {loading && notifications.length === 0 ? (
          <div className="flex items-center justify-center py-8 text-text-disabled text-sm gap-2">
            <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
              />
            </svg>
            Loading...
          </div>
        ) : notifications.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-text-disabled">
            <Bell className="w-8 h-8 mb-2 opacity-50" />
            <p className="text-sm">No notifications</p>
          </div>
        ) : (
          notifications.map((n) => {
            const link = getNotificationLink(n);
            const icon = NOTIFICATION_ICONS[n.notification_type] || '🔔';
            const Wrapper = link ? Link : 'div';
            const wrapperProps = link
              ? {
                  to: link,
                  onClick: () => {
                    if (!n.is_read) {
                      api.markNotificationAsRead(n.id);
                      setNotifications((prev) =>
                        prev.map((x) =>
                          x.id === n.id ? { ...x, is_read: true } : x
                        )
                      );
                      setUnreadCount((prev) => Math.max(0, prev - 1));
                      announce('Notification marked as read');
                    }
                    close();
                  },
                }
              : {};

            return (
              <Wrapper
                key={n.id}
                className={`flex items-start gap-3 px-4 py-3 border-b border-border-subtle hover:bg-surface-1 transition-colors cursor-pointer ${!n.is_read ? 'bg-brand-50/50' : ''}`}
                {...wrapperProps}
              >
                <span className="text-lg flex-shrink-0 mt-0.5">{icon}</span>
                <div className="flex-1 min-w-0">
                  <p
                    className={`text-sm leading-snug ${!n.is_read ? 'font-medium text-text-primary' : 'text-text-secondary'}`}
                  >
                    {n.title}
                  </p>
                  {n.message && (
                    <p className="text-xs text-text-tertiary mt-0.5 truncate">
                      {n.message}
                    </p>
                  )}
                  <p className="text-xs text-text-disabled mt-1">
                    {formatTime(n.created_at)}
                  </p>
                </div>
                {!n.is_read && (
                  <button
                    onClick={(e) => handleMarkAsRead(n.id, e)}
                    className="p-1 text-text-disabled hover:text-brand-600 rounded flex-shrink-0"
                    aria-label="Mark as read"
                    title="Mark as read"
                  >
                    <Check className="w-3.5 h-3.5" />
                  </button>
                )}
              </Wrapper>
            );
          })
        )}
      </div>

      {/* Footer */}
      {notifications.length > 0 && (
        <div className="border-t border-border-subtle px-4 py-2">
          <Link
            to="/notifications"
            onClick={close}
            className="text-xs text-brand-600 hover:text-brand-800 font-medium"
          >
            View all notifications
          </Link>
        </div>
      )}
    </>
  );

  // ----- Position-specific dropdown wrappers -----
  const panelClassByPosition = {
    // Existing behavior — opens to right of bell, anchored to bottom.
    sidebar:
      'absolute start-full ms-2 bottom-0 w-80 max-h-[480px] bg-surface-0 rounded-lg shadow-xl border border-border-default z-50 flex flex-col',
    // Header variant — drops down below the trigger, right-aligned.
    header:
      'absolute end-0 top-full mt-2 w-80 max-h-[480px] bg-surface-0 rounded-lg shadow-xl border border-border-default z-50 flex flex-col',
  };

  const renderPanel = () => {
    if (!isOpen) return null;

    if (position === 'mobile-sheet') {
      return (
        <div
          className="fixed inset-0 z-50 flex items-end justify-center"
          role="presentation"
        >
          {/* Backdrop */}
          <button
            type="button"
            aria-label="Close notifications"
            onClick={close}
            className="absolute inset-0 bg-black/40 animate-in fade-in"
          />
          {/* Sheet */}
          <FocusTrap active={isOpen} onEscape={close}>
            <div
              id={dialogId}
              role="dialog"
              aria-modal="true"
              aria-labelledby={`${dialogId}-title`}
              className="relative w-full max-w-2xl h-[70vh] bg-surface-0 rounded-t-2xl shadow-2xl border-t border-border-default flex flex-col animate-in slide-in-from-bottom duration-200"
            >
              {/* drag handle */}
              <div className="flex justify-center pt-2 pb-1">
                <span className="block w-10 h-1.5 rounded-full bg-border-strong" />
              </div>
              {panelBody}
            </div>
          </FocusTrap>
        </div>
      );
    }

    const className =
      panelClassByPosition[position] || panelClassByPosition.sidebar;

    return (
      <FocusTrap active={isOpen} onEscape={close}>
        <div
          id={dialogId}
          className={className}
          role="dialog"
          aria-modal="true"
          aria-labelledby={`${dialogId}-title`}
        >
          {panelBody}
        </div>
      </FocusTrap>
    );
  };

  // Tooltip side depends on position
  const tooltipSide = position === 'sidebar' ? 'right' : 'bottom';

  return (
    <div
      className={position === 'mobile-sheet' ? 'inline-flex' : 'relative'}
      ref={containerRef}
    >
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            ref={triggerRef}
            onClick={toggle}
            className="relative flex items-center justify-center w-10 h-10 rounded-md text-gray-300 hover:bg-surface-0/10 hover:text-white transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
            aria-label={ariaLabel}
            aria-haspopup="dialog"
            aria-expanded={isOpen}
            aria-controls={isOpen ? dialogId : undefined}
          >
            {dndActive ? <BellOff className="w-5 h-5" /> : <Bell className="w-5 h-5" />}
            {showBadge && (
              <span
                className="absolute top-1 end-1 flex items-center justify-center min-w-[16px] h-4 px-1 text-[11px] font-semibold text-white bg-accent-danger rounded-full leading-none"
                aria-hidden="true"
              >
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
            {dndActive && (
              <span
                className="absolute -bottom-0.5 -end-0.5 flex items-center justify-center px-1 h-3.5 text-[9px] font-semibold text-white bg-accent-warning rounded-full leading-none"
                aria-hidden="true"
                title="Do Not Disturb"
              >
                DND
              </span>
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side={tooltipSide}>
          {dndActive ? 'Notifications (DND on)' : 'Notifications'}
        </TooltipContent>
      </Tooltip>

      {renderPanel()}
    </div>
  );
};

export default NotificationBell;
