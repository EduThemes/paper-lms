import React, { useState, useEffect } from 'react';
import { Save } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { Skeleton } from '@/components/ui/skeleton';

const ACCOUNT_ID = 1;

const AdminSettingsPage = () => {
  const [account, setAccount] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState('');
  const [maxUploadMB, setMaxUploadMB] = useState('');

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const a = await api.getAccount(ACCOUNT_ID);
        if (cancelled) return;
        setAccount(a);
        setMaxUploadMB(String(a.max_upload_size_mb ?? 500));
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
    const n = Number.parseInt(maxUploadMB, 10);
    if (Number.isNaN(n) || n < 1) {
      setError('Upload size must be a positive whole number of megabytes.');
      return;
    }
    if (n > 5120) {
      setError('Maximum is 5120 MB (5 GB) — the framework-level safety net.');
      return;
    }
    setSaving(true);
    setError(null);
    setStatusMsg('');
    try {
      const updated = await api.updateAccount(ACCOUNT_ID, {
        settings: { max_upload_size_mb: n },
      });
      setAccount(updated);
      setMaxUploadMB(String(updated.max_upload_size_mb));
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
          <h1 className="text-2xl font-bold text-text-primary">Settings</h1>
          <p className="text-sm text-text-secondary mt-1">Account-level configuration.</p>
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
              <legend className="px-2 text-sm font-semibold text-text-primary">Uploads</legend>

              <label htmlFor="max-upload" className="block text-sm font-medium text-text-secondary mt-2">
                Maximum upload size (MB)
              </label>
              <p className="text-xs text-text-tertiary mt-1 mb-2">
                Per-request cap for course files and content imports (.imscc, .zip).
                Allowed range: 1–5120 MB. Changes take effect within 30 seconds without
                a server restart.
              </p>
              <input
                id="max-upload"
                type="number"
                min={1}
                max={5120}
                value={maxUploadMB}
                onChange={(e) => setMaxUploadMB(e.target.value)}
                className="w-40 rounded-md border border-border-strong bg-surface-0 px-3 py-2 text-sm"
              />
              <p className="text-xs text-text-tertiary mt-2">
                Currently enforced: <span className="font-medium text-text-secondary">{account?.max_upload_size_mb ?? 500} MB</span>
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

export default AdminSettingsPage;
