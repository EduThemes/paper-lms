import React, { useState, useEffect, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { Bell, Check, CheckCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';

const NOTIFICATION_ICONS = {
  submission_grade: '📝',
  new_message: '💬',
  new_announcement: '📢',
  event_start: '📅',
  enrollment: '👤',
  discussion_reply: '💬',
  submission_comment: '💬',
};

const NotificationsPage = () => {
  const { t } = useTranslation();
  const [notifications, setNotifications] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [filter, setFilter] = useState('all'); // all | unread

  const fetchNotifications = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.getNotifications(1, 100, filter === 'unread');
      setNotifications(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  const handleMarkAsRead = async (id) => {
    try {
      await api.markNotificationAsRead(id);
      setNotifications(prev => prev.map(n =>
        n.id === id ? { ...n, is_read: true } : n
      ));
    } catch (err) {
      setError(err.message);
    }
  };

  const handleMarkAllRead = async () => {
    try {
      await api.markAllNotificationsAsRead();
      setNotifications(prev => prev.map(n => ({ ...n, is_read: true })));
    } catch (err) {
      setError(err.message);
    }
  };

  const getNotificationLink = (n) => {
    if (n.context_type === 'Course' && n.context_id) {
      if (n.notification_type === 'submission_grade') return `/courses/${n.context_id}/grades`;
      if (n.notification_type === 'new_announcement') return `/courses/${n.context_id}/announcements`;
      if (n.notification_type === 'discussion_reply') return `/courses/${n.context_id}/discussions`;
      return `/courses/${n.context_id}`;
    }
    if (n.notification_type === 'new_message') return '/inbox';
    if (n.notification_type === 'event_start') return '/calendar';
    return null;
  };

  const formatTime = (dateStr) => {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    return date.toLocaleDateString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    });
  };

  const unreadCount = notifications.filter(n => !n.is_read).length;

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
          <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
          {t('notificationsPage.loading')}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <h2 className="text-2xl font-bold text-text-primary">{t('notificationsPage.title')}</h2>
          <div className="flex items-center gap-3">
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="border border-border-strong rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="all">{t('notificationsPage.all')}</option>
              <option value="unread">{t('notificationsPage.unreadOnly')}</option>
            </select>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="inline-flex items-center gap-1.5 text-sm text-brand-600 hover:text-brand-800 font-medium"
              >
                <CheckCheck className="w-4 h-4" />
                {t('notificationsPage.markAllRead')}
              </button>
            )}
          </div>
        </div>
        {unreadCount > 0 && (
          <p className="text-sm text-text-tertiary mt-1">{t('notificationsPage.unreadCount', { count: unreadCount })}</p>
        )}
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm">
          {error}
          <button onClick={() => { setError(null); fetchNotifications(); }} className="ml-2 text-accent-danger hover:text-accent-danger font-bold">{t('common.tryAgain')}</button>
        </div>
      )}

      {notifications.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Bell className="w-12 h-12 text-gray-300 mx-auto mb-3" />
          <p className="text-text-tertiary text-lg mb-1">
            {filter === 'unread' ? t('notificationsPage.noUnreadNotifications') : t('notificationsPage.noNotifications')}
          </p>
          <p className="text-text-disabled text-sm">
            {t('notificationsPage.emptyHint')}
          </p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow divide-y divide-gray-100">
          {notifications.map((n) => {
            const link = getNotificationLink(n);
            const icon = NOTIFICATION_ICONS[n.notification_type] || '🔔';

            return (
              <div
                key={n.id}
                className={`flex items-start gap-4 px-6 py-4 ${!n.is_read ? 'bg-brand-50/30' : ''}`}
              >
                <span className="text-2xl flex-shrink-0 mt-0.5">{icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      {link ? (
                        <Link
                          to={link}
                          className={`text-sm hover:text-brand-600 ${!n.is_read ? 'font-semibold text-text-primary' : 'text-text-secondary'}`}
                          onClick={() => !n.is_read && handleMarkAsRead(n.id)}
                        >
                          {n.title}
                        </Link>
                      ) : (
                        <p className={`text-sm ${!n.is_read ? 'font-semibold text-text-primary' : 'text-text-secondary'}`}>
                          {n.title}
                        </p>
                      )}
                      {n.message && (
                        <p className="text-sm text-text-tertiary mt-0.5">{n.message}</p>
                      )}
                      <p className="text-xs text-text-disabled mt-1">{formatTime(n.created_at)}</p>
                    </div>
                    {!n.is_read && (
                      <button
                        onClick={() => handleMarkAsRead(n.id)}
                        className="p-1.5 text-text-disabled hover:text-brand-600 hover:bg-brand-50 rounded flex-shrink-0"
                        title={t('notificationsPage.markAsRead')}
                      >
                        <Check className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Layout>
  );
};

export default NotificationsPage;
