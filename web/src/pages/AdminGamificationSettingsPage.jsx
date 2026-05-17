import React, { useState, useEffect } from 'react';
import { Save } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const ACCOUNT_ID = 1;

// Tenant mode is the single knob that drives the gamification RenderPolicy
// (see internal/service/gamification/render_policy.go): leaderboard visibility,
// pseudonym requirements, and real-name display all hang off it. Lives under
// Admin → Gamification so admins find it where they look for it, instead of
// buried in the generic /admin/settings page.
const TENANT_MODE_OPTIONS = [
  { value: 'k5', label: 'K-5 (Elementary)', description: 'Hides peer leaderboards from students. Pseudonyms required.' },
  { value: 'm68', label: 'Middle (6-8)', description: 'Hides peer leaderboards from students. Pseudonyms required.' },
  { value: 'h912', label: 'High School (9-12)', description: 'Top-5 students see their top-5 peers. Pseudonyms by default.' },
  { value: 'higher_ed', label: 'Higher Education', description: 'Real names; full leaderboards visible to enrolled students.' },
  { value: 'corp', label: 'Corporate Training', description: 'Real names; full leaderboards visible to enrolled learners.' },
  { value: 'pro', label: 'Professional Development', description: 'Real names; full leaderboards visible to enrolled learners.' },
];

const AdminGamificationSettingsPage = () => {
  const [account, setAccount] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState('');
  const [tenantMode, setTenantMode] = useState('higher_ed');

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const a = await api.getAccount(ACCOUNT_ID);
        if (cancelled) return;
        setAccount(a);
        setTenantMode(a.tenant_mode || 'higher_ed');
      } catch (err) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => { cancelled = true; };
  }, []);

  const handleSave = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setStatusMsg('');
    try {
      const updated = await api.updateAccount(ACCOUNT_ID, {
        tenant_mode: tenantMode,
      });
      setAccount(updated);
      setTenantMode(updated.tenant_mode || 'higher_ed');
      setStatusMsg('Settings saved');
      setTimeout(() => setStatusMsg(''), 1800);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Layout>
      <div className="p-8 max-w-2xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-text-primary">Gamification settings</h1>
          <p className="text-sm text-text-secondary mt-1">
            Tenant-wide defaults for leaderboards, pseudonyms, and name display.
          </p>
        </div>

        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : error && !account ? (
          <div className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-4">
            <p className="text-sm text-accent-danger">{error}</p>
          </div>
        ) : (
          <form onSubmit={handleSave} className="space-y-6">
            <fieldset className="rounded-lg border border-border-default bg-surface-0 p-5">
              <legend className="px-2 text-sm font-semibold text-text-primary">Tenant mode</legend>
              <p className="text-xs text-text-tertiary mt-1 mb-3">
                Drives every gamification + privacy default. Leaderboard visibility,
                pseudonym requirements, and real-name display all hang off this
                setting. Change with care — switching to a stricter mode immediately
                hides peer leaderboards from existing students.
              </p>
              <div className="space-y-2">
                {TENANT_MODE_OPTIONS.map((opt) => (
                  <label
                    key={opt.value}
                    className={`flex cursor-pointer items-start gap-3 rounded-md border p-3 text-sm ${
                      tenantMode === opt.value
                        ? 'border-brand-500 bg-brand-500/5'
                        : 'border-border-default bg-surface-0 hover:bg-surface-1'
                    }`}
                  >
                    <input
                      type="radio"
                      name="tenant_mode"
                      value={opt.value}
                      checked={tenantMode === opt.value}
                      onChange={(e) => setTenantMode(e.target.value)}
                      className="mt-1"
                    />
                    <div>
                      <div className="font-medium text-text-primary">{opt.label}</div>
                      <div className="text-xs text-text-tertiary mt-0.5">{opt.description}</div>
                    </div>
                  </label>
                ))}
              </div>
              <p className="text-xs text-text-tertiary mt-3">
                Currently active: <span className="font-medium text-text-secondary">{account?.tenant_mode || 'higher_ed'}</span>
              </p>
            </fieldset>

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

export default AdminGamificationSettingsPage;
