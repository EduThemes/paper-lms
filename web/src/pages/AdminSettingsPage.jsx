import React, { useState, useEffect } from 'react';
import { Save } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

// SETTING_KEY is the catalog key in internal/service/settings/catalog.go.
// Bound at instance scope here; account-scoped overrides land in the
// Super-Admin → Settings panel (Quotas & limits group).
//
// Wave 4 repoint (chore/wave4-upload-size-catalog 2026-05-17): this
// page previously wrote to account.max_upload_size_mb directly, which
// made the catalog entry a no-op. The form now reads/writes through
// the Settings Engine — single source of truth, runtime configurable
// without restart, per-tenant overridable at account scope.
const SETTING_KEY = 'quotas.max_upload_size_mb';

// FALLBACK_DEFAULT_MB matches the catalog default. The framework-level
// BodyLimit is 5 GB, so this is the upper bound on what's actionable.
const FALLBACK_DEFAULT_MB = 5120;

const AdminSettingsPage = () => {
  const [effective, setEffective] = useState(null); // EffectiveValue from /superadmin/settings/:key
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState('');
  const [maxUploadMB, setMaxUploadMB] = useState('');

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const ev = await api.superAdminSettings.getSetting(SETTING_KEY);
        if (cancelled) return;
        setEffective(ev);
        const current = parseInt(ev?.value, 10);
        setMaxUploadMB(String(Number.isFinite(current) && current > 0 ? current : FALLBACK_DEFAULT_MB));
      } catch (err) {
        if (cancelled) return;
        // A non-super-admin viewing this page gets 403 from the
        // server. We surface a friendly, non-blocking message — the
        // catalog default is in effect; raising it is a super-admin
        // operation. Don't show a scary error block for what's a UX
        // path, not a system failure.
        if (err?.status === 403 || /forbidden/i.test(err?.message || '')) {
          setEffective({ value: String(FALLBACK_DEFAULT_MB), source: 'default', is_secret: false });
          setMaxUploadMB(String(FALLBACK_DEFAULT_MB));
        } else {
          setError(err.message);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => { cancelled = true; };
  }, []);

  const handleSave = async (e) => {
    e.preventDefault();
    const n = Number.parseInt(maxUploadMB, 10);
    if (Number.isNaN(n) || n < 1) {
      setError('Upload size must be a positive whole number of megabytes.');
      return;
    }
    if (n > FALLBACK_DEFAULT_MB) {
      setError(`Maximum is ${FALLBACK_DEFAULT_MB} MB (5 GB) — the framework-level safety net.`);
      return;
    }
    setSaving(true);
    setError(null);
    setStatusMsg('');
    try {
      // Bind at instance scope (the existing semantics). Super-admin
      // operators wanting per-district overrides do that in the
      // dedicated Super-Admin Settings panel.
      await api.superAdminSettings.setSetting(SETTING_KEY, {
        scope: 'instance',
        scope_id: 0,
        value: String(n),
      });
      const refreshed = await api.superAdminSettings.getSetting(SETTING_KEY);
      setEffective(refreshed);
      setMaxUploadMB(String(parseInt(refreshed.value, 10) || n));
      setStatusMsg('Settings saved');
      setTimeout(() => setStatusMsg(''), 1800);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const currentMB = parseInt(effective?.value, 10);
  const currentDisplay = Number.isFinite(currentMB) && currentMB > 0 ? currentMB : FALLBACK_DEFAULT_MB;
  const sourceLabel = effective?.source || 'default';

  return (
    <Layout>
      <div className="p-8 max-w-2xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-text-primary">Settings</h1>
          <p className="text-sm text-text-secondary mt-1">Account-level configuration.</p>
        </div>

        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : error && !effective ? (
          <div className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-4">
            <p className="text-sm text-accent-danger">{error}</p>
          </div>
        ) : (
          <form onSubmit={handleSave} className="space-y-6">
            <fieldset className="rounded-lg border border-border-default bg-surface-0 p-5">
              <legend className="px-2 text-sm font-semibold text-text-primary">Uploads</legend>

              <label htmlFor="max-upload" className="block text-sm font-medium text-text-secondary mt-2">
                Maximum upload size (MB)
              </label>
              <p className="text-xs text-text-tertiary mt-1 mb-2">
                Per-request cap for course files and content imports (.imscc, .zip).
                Allowed range: 1–{FALLBACK_DEFAULT_MB} MB. Changes take effect immediately
                without a server restart.
              </p>
              <input
                id="max-upload"
                type="number"
                min={1}
                max={FALLBACK_DEFAULT_MB}
                value={maxUploadMB}
                onChange={(e) => setMaxUploadMB(e.target.value)}
                className="w-40 rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm"
              />
              <p className="text-xs text-text-tertiary mt-2">
                Currently enforced: <span className="font-medium text-text-secondary">{currentDisplay} MB</span>
                {' '}<span className="text-text-tertiary">(source: {sourceLabel})</span>
              </p>
            </fieldset>

            <p className="text-xs text-text-tertiary">
              Looking for leaderboard / pseudonym / tenant-mode settings? They moved to
              {' '}<a href="/admin/gamification/settings" className="text-brand-600 hover:underline">Admin → Gamification → Settings</a>.
            </p>

            {error && (
              <div className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-3">
                <p className="text-sm text-accent-danger">{error}</p>
              </div>
            )}

            <div className="flex items-center gap-3">
              <button
                type="submit"
                disabled={saving}
                className="inline-flex items-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
              >
                <Save className="w-4 h-4" />
                {saving ? 'Saving…' : 'Save'}
              </button>
              {statusMsg && <span className="text-xs text-accent-success" role="status">{statusMsg}</span>}
            </div>
          </form>
        )}
      </div>
    </Layout>
  );
};

export default AdminSettingsPage;
