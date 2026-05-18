import React, { useCallback, useEffect, useState } from 'react';
import { Trophy, Check, AlertTriangle } from 'lucide-react';
import { Trans, useTranslation } from 'react-i18next';
import Layout from '../components/Layout';
import { api } from '../services/api';

// GamificationPreferencesPage hosts the learner-facing privacy toggles
// for the gamification system. W2-C ships a single toggle —
// leaderboard opt-out — but the page is plural-named on purpose so
// future Wave 3 prefs (streak settings, friend visibility, etc.)
// land in the same place rather than fragmenting across menus.
//
// Privacy contract per SYNTHESIS §5: opting out hides the learner from
// public leaderboard surfaces. It does NOT zero XP, awards, mastery —
// the page copy makes this contract loud so a learner can opt out
// without fearing they'll lose progress.
export default function GamificationPreferencesPage() {
  const { t } = useTranslation();
  const [optOut, setOptOut] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [savedAt, setSavedAt] = useState(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const prefs = await api.gamification.getMyPreferences();
      setOptOut(!!prefs.leaderboard_opt_out);
    } catch (err) {
      console.error('GamificationPreferencesPage: failed to load', err);
      setError(err.message || t('gamificationPreferencesPage.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleToggle = async (e) => {
    const next = e.target.checked;
    setOptOut(next);
    setSaving(true);
    setError(null);
    try {
      const prefs = await api.gamification.updateMyPreferences({ leaderboard_opt_out: next });
      setOptOut(!!prefs.leaderboard_opt_out);
      setSavedAt(Date.now());
    } catch (err) {
      // Revert the optimistic toggle on failure so the UI doesn't
      // diverge from the server.
      setOptOut(!next);
      setError(err.message || t('gamificationPreferencesPage.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Layout>
      <div className="max-w-2xl mx-auto py-6 space-y-6">
        <header>
          <h1 className="text-xl font-semibold text-text-primary flex items-center gap-2">
            <Trophy className="w-5 h-5" /> {t('gamificationPreferencesPage.title')}
          </h1>
          <p className="text-sm text-text-secondary mt-1">
            {t('gamificationPreferencesPage.subtitle')}
          </p>
        </header>

        {loading ? (
          <div className="text-sm text-text-tertiary">{t('common.loading')}</div>
        ) : (
          <section className="border border-surface-raised rounded-lg bg-surface-0">
            <div className="px-5 py-4 border-b border-surface-raised">
              <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wider">
                {t('gamificationPreferencesPage.privacy')}
              </h2>
            </div>
            <div className="px-5 py-4">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={optOut}
                  onChange={handleToggle}
                  disabled={saving}
                  className="mt-1 accent-brand-600"
                />
                <div className="flex-1">
                  <div className="text-sm font-medium text-text-primary">
                    {t('gamificationPreferencesPage.hideFromLeaderboards')}
                  </div>
                  <p className="text-xs text-text-tertiary mt-1">
                    <Trans
                      i18nKey="gamificationPreferencesPage.hideDescription"
                      components={{ strong: <strong /> }}
                    />
                  </p>
                </div>
              </label>

              {error && (
                <div className="mt-3 flex items-start gap-2 text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2">
                  <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                  <span>{error}</span>
                </div>
              )}
              {!error && savedAt && (
                <div className="mt-3 flex items-center gap-1.5 text-xs text-accent-success">
                  <Check className="w-3.5 h-3.5" />
                  <span>{t('gamificationPreferencesPage.saved')}</span>
                </div>
              )}
            </div>
          </section>
        )}
      </div>
    </Layout>
  );
}
