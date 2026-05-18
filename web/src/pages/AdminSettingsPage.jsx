// Reference migration #3 — "load resource, edit form, save" shape.
//
// Before: ~50 lines of useState/useEffect/try/catch + manual `cancelled`
// guard + a separate `saving` flag and `statusMsg` flash + a bespoke
// `setError(err.message)` ladder.
//
// After: `useEffectiveSetting()` provides the initial fetch (with a
// 403 → "default" fallback so non-super-admin viewers don't see a
// scary error block). `useUpdateSetting()` wraps the PUT and seeds the
// cache with the response so the rendered value updates immediately.
// Form-level state (`maxUploadMB`, validation error) stays in
// `useState` because react-query isn't a form library — it just owns
// the network round-trip.
//
// Note how the validation-error case (the input is out of range) is
// distinct from the server-error case (the PUT failed). The first
// lives in local state; the second comes from `mutation.error`.
//
// Wave 4 repoint (chore/wave4-upload-size-catalog 2026-05-17): this
// page previously wrote to account.max_upload_size_mb directly, which
// made the catalog entry a no-op. The form now reads/writes through
// the Settings Engine — single source of truth, runtime configurable
// without restart, per-tenant overridable at account scope.

import React, { useState, useEffect } from 'react';
import { Save } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import { useEffectiveSetting, useUpdateSetting } from '../services/apiQueries';

// SETTING_KEY is the catalog key in internal/service/settings/catalog.go.
// Bound at instance scope here; account-scoped overrides land in the
// Super-Admin → Settings panel (Quotas & limits group).
const SETTING_KEY = 'quotas.max_upload_size_mb';

// FALLBACK_DEFAULT_MB matches the catalog default. The framework-level
// BodyLimit is 5 GB, so this is the upper bound on what's actionable.
const FALLBACK_DEFAULT_MB = 5120;

const AdminSettingsPage = () => {
  const query = useEffectiveSetting(SETTING_KEY);
  const updateSetting = useUpdateSetting(SETTING_KEY);

  const [maxUploadMB, setMaxUploadMB] = useState('');
  const [validationError, setValidationError] = useState(null);
  const [statusMsg, setStatusMsg] = useState('');

  // Sync the local input from the resolved server value once the
  // first fetch lands. Subsequent successful saves seed the cache
  // (see `useUpdateSetting`), so this also picks up the new value
  // after a save without an extra effect.
  useEffect(() => {
    if (query.data) {
      const current = parseInt(query.data.value, 10);
      setMaxUploadMB(
        String(Number.isFinite(current) && current > 0 ? current : FALLBACK_DEFAULT_MB),
      );
    }
  }, [query.data]);

  const handleSave = async (e) => {
    e.preventDefault();
    setValidationError(null);
    const n = Number.parseInt(maxUploadMB, 10);
    if (Number.isNaN(n) || n < 1) {
      setValidationError('Upload size must be a positive whole number of megabytes.');
      return;
    }
    if (n > FALLBACK_DEFAULT_MB) {
      setValidationError(
        `Maximum is ${FALLBACK_DEFAULT_MB} MB (5 GB) — the framework-level safety net.`,
      );
      return;
    }
    try {
      // Bind at instance scope (the existing semantics). Super-admin
      // operators wanting per-district overrides do that in the
      // dedicated Super-Admin Settings panel.
      await updateSetting.mutateAsync({
        scope: 'instance',
        scope_id: 0,
        value: String(n),
      });
      setStatusMsg('Settings saved');
      setTimeout(() => setStatusMsg(''), 1800);
    } catch {
      // Surfaced via updateSetting.error below.
    }
  };

  return (
    <Page query={query} title="Settings">
      {(effective) => {
        const currentMB = parseInt(effective?.value, 10);
        const currentDisplay =
          Number.isFinite(currentMB) && currentMB > 0 ? currentMB : FALLBACK_DEFAULT_MB;
        const sourceLabel = effective?.source || 'default';

        return (
          <Layout>
            <div className="p-8 max-w-2xl">
              <div className="mb-6">
                <h1 className="text-2xl font-bold text-text-primary">Settings</h1>
                <p className="text-sm text-text-secondary mt-1">Account-level configuration.</p>
              </div>

              <form onSubmit={handleSave} className="space-y-6">
                <fieldset className="rounded-lg border border-border-default bg-surface-0 p-5">
                  <legend className="px-2 text-sm font-semibold text-text-primary">Uploads</legend>

                  <label
                    htmlFor="max-upload"
                    className="block text-sm font-medium text-text-secondary mt-2"
                  >
                    Maximum upload size (MB)
                  </label>
                  <p className="text-xs text-text-tertiary mt-1 mb-2">
                    Per-request cap for course files and content imports (.imscc, .zip). Allowed
                    range: 1–{FALLBACK_DEFAULT_MB} MB. Changes take effect immediately without a
                    server restart.
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
                    Currently enforced:{' '}
                    <span className="font-medium text-text-secondary">{currentDisplay} MB</span>{' '}
                    <span className="text-text-tertiary">(source: {sourceLabel})</span>
                  </p>
                </fieldset>

                <p className="text-xs text-text-tertiary">
                  Looking for leaderboard / pseudonym / tenant-mode settings? They moved to{' '}
                  <a
                    href="/admin/gamification/settings"
                    className="text-brand-600 hover:underline"
                  >
                    Admin → Gamification → Settings
                  </a>
                  .
                </p>

                {(validationError || updateSetting.isError) && (
                  <div
                    className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-3"
                    role="alert"
                  >
                    <p className="text-sm text-accent-danger">
                      {validationError || updateSetting.error?.message}
                    </p>
                  </div>
                )}

                <div className="flex items-center gap-3">
                  <button
                    type="submit"
                    disabled={updateSetting.isPending}
                    className="inline-flex items-center gap-2 rounded-md bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
                  >
                    <Save className="w-4 h-4" />
                    {updateSetting.isPending ? 'Saving…' : 'Save'}
                  </button>
                  {statusMsg && (
                    <span className="text-xs text-accent-success" role="status">
                      {statusMsg}
                    </span>
                  )}
                </div>
              </form>
            </div>
          </Layout>
        );
      }}
    </Page>
  );
};

export default AdminSettingsPage;
