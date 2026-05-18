import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Bell, Settings, Save, Check, AlertTriangle, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

const POLICY_VALUES = ['immediately', 'daily', 'weekly', 'never'];

const NOTIFICATION_KEYS = [
  'notify_new_message',
  'notify_event_start',
  'notify_submission_grade',
  'notify_new_announcement',
];

const NotificationPreferencesPage = () => {
  const { t } = useTranslation();
  const { user } = useAuth();

  const POLICY_OPTIONS = useMemo(() => POLICY_VALUES.map((value) => ({
    value,
    label: t(`notificationPreferences.policy.${value}`),
  })), [t]);

  const NOTIFICATION_TYPES = useMemo(() => NOTIFICATION_KEYS.map((key) => ({
    key,
    label: t(`notificationPreferences.types.${key}.label`),
    description: t(`notificationPreferences.types.${key}.description`),
  })), [t]);
  const [preferences, setPreferences] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);

  const fetchPreferences = useCallback(async () => {
    try {
      const result = await api.getNotificationPreferences();
      setPreferences(result.notification_preferences);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPreferences();
  }, [fetchPreferences]);

  const handlePolicyChange = (e) => {
    setPreferences((prev) => ({ ...prev, policy: e.target.value }));
    setSuccess(false);
  };

  const handleToggle = (key) => {
    setPreferences((prev) => ({ ...prev, [key]: !prev[key] }));
    setSuccess(false);
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      const result = await api.updateNotificationPreferences({
        policy: preferences.policy,
        notify_new_message: preferences.notify_new_message,
        notify_event_start: preferences.notify_event_start,
        notify_submission_grade: preferences.notify_submission_grade,
        notify_new_announcement: preferences.notify_new_announcement,
      });
      setPreferences(result.notification_preferences);
      setSuccess(true);
      setTimeout(() => setSuccess(false), 3000);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  {t('notificationPreferences.loading')}
</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-3xl mx-auto">
        <div className="flex items-center gap-3 mb-6">
          <div className="bg-brand-100 p-2 rounded-lg">
            <Bell className="w-6 h-6 text-brand-600" />
          </div>
          <div>
            <h2 className="text-2xl font-bold text-text-primary">{t('notificationPreferences.title')}</h2>
            <p className="text-text-secondary mt-0.5 text-sm">
              {t('notificationPreferences.subtitle')}
            </p>
          </div>
        </div>

        {error && (
          <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0" />
            {error}
            <button onClick={() => setError(null)} className="ml-auto">
              <X className="w-4 h-4" />
            </button>
          </div>
        )}

        {success && (
          <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
            <Check className="w-4 h-4 flex-shrink-0" />
            {t('notificationPreferences.savedSuccess')}
          </div>
        )}

        {preferences && (
          <div className="space-y-6">
            {/* Delivery Policy */}
            <div className="bg-surface-0 rounded-lg shadow p-6">
              <div className="flex items-center gap-2 mb-4">
                <Settings className="w-5 h-5 text-text-tertiary" />
                <h3 className="text-lg font-semibold text-text-primary">{t('notificationPreferences.deliveryPolicy')}</h3>
              </div>
              <p className="text-sm text-text-secondary mb-4">
                {t('notificationPreferences.deliveryPolicyDescription')}
              </p>
              <select
                value={preferences.policy}
                onChange={handlePolicyChange}
                className="w-full sm:w-64 rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
              >
                {POLICY_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Notification Types */}
            <div className="bg-surface-0 rounded-lg shadow p-6">
              <div className="flex items-center gap-2 mb-4">
                <Bell className="w-5 h-5 text-text-tertiary" />
                <h3 className="text-lg font-semibold text-text-primary">{t('notificationPreferences.notificationTypes')}</h3>
              </div>
              <p className="text-sm text-text-secondary mb-4">
                {t('notificationPreferences.notificationTypesDescription')}
              </p>
              <div className="divide-y divide-gray-100">
                {NOTIFICATION_TYPES.map((type) => (
                  <div key={type.key} className="flex items-center justify-between py-4 first:pt-0 last:pb-0">
                    <div>
                      <p className="text-sm font-medium text-text-primary">{type.label}</p>
                      <p className="text-xs text-text-tertiary mt-0.5">{type.description}</p>
                    </div>
                    <button
                      onClick={() => handleToggle(type.key)}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 ${
                        preferences[type.key] ? 'bg-brand-600' : 'bg-border-default'
                      }`}
                      role="switch"
                      aria-checked={preferences[type.key]}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-surface-0 transition-transform ${
                          preferences[type.key] ? 'translate-x-6' : 'translate-x-1'
                        }`}
                      />
                    </button>
                  </div>
                ))}
              </div>
            </div>

            {/* Save Button */}
            <div className="flex justify-end">
              <button
                onClick={handleSave}
                disabled={saving}
                className="flex items-center gap-2 bg-brand-600 text-white px-6 py-2.5 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50 transition-colors"
              >
                {saving ? (
                  <>{t('common.saving')}</>
                ) : success ? (
                  <>
                    <Check className="w-4 h-4" />
                    {t('notificationPreferences.saved')}
                  </>
                ) : (
                  <>
                    <Save className="w-4 h-4" />
                    {t('notificationPreferences.savePreferences')}
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default NotificationPreferencesPage;
