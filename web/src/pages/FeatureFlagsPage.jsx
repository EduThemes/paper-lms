import React, { useEffect, useMemo, useState } from 'react';
import { Flag, Lock, RefreshCw, AlertCircle } from 'lucide-react';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

/**
 * Admin-only page that lists every feature flag for the current account
 * (default account_id=1 in single-tenant Paper LMS) and lets the admin
 * toggle each one between allowed / on / off.
 *
 * The grid mirrors Canvas's /accounts/:id/settings#tab-features layout
 * but cleaner: release-stage badges, a search, and inline state pills.
 */

const STATE_ORDER = ['off', 'allowed', 'on'];

const STATE_STYLES = {
  on:      'bg-accent-success/20 text-accent-success border-accent-success/30',
  off:     'bg-surface-2 text-text-secondary border-border-default',
  allowed: 'bg-brand-50  text-brand-800  border-brand-100',
  hidden:  'bg-purple-100 text-purple-800 border-purple-200',
};

const STAGE_STYLES = {
  released: 'bg-accent-success/10 text-accent-success ring-1 ring-accent-success/30',
  beta:     'bg-accent-warning/10  text-accent-warning  ring-1 ring-accent-warning/30',
  hidden:   'bg-slate-100 text-slate-700  ring-1 ring-slate-200',
};

const Spinner = () => (
  <svg className="animate-spin h-5 w-5 text-brand-600" viewBox="0 0 24 24" fill="none">
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
    <path
      className="opacity-75"
      fill="currentColor"
      d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"
    />
  </svg>
);

const StateToggle = ({ flag, onChange, disabled }) => {
  const idx = STATE_ORDER.indexOf(flag.state);
  return (
    <div className="inline-flex rounded-lg border border-border-default overflow-hidden text-xs">
      {STATE_ORDER.map((s, i) => (
        <button
          key={s}
          disabled={disabled}
          onClick={() => onChange(s)}
          className={[
            'px-3 py-1.5 font-medium transition-colors',
            i === idx
              ? `${STATE_STYLES[s]} border-l ${i === 0 ? 'border-l-0' : ''}`
              : 'bg-surface-0 text-text-tertiary hover:bg-surface-1',
            disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
          ].join(' ')}
        >
          {s}
        </button>
      ))}
    </div>
  );
};

const FeatureFlagsPage = () => {
  const { user } = useAuth();
  const [flags, setFlags] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [search, setSearch] = useState('');
  const [busyFeature, setBusyFeature] = useState(null);

  const accountId = 1; // single-tenant default

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAccountFeatureFlags(accountId);
      const data = result.data || result || [];
      data.sort((a, b) => a.display_name.localeCompare(b.display_name));
      setFlags(data);
    } catch (err) {
      setError(err.message || 'Failed to load feature flags');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const setState = async (feature, state) => {
    setBusyFeature(feature);
    try {
      await api.setAccountFeatureFlag(accountId, feature, state);
      await load();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusyFeature(null);
    }
  };

  const reset = async (feature) => {
    setBusyFeature(feature);
    try {
      await api.resetAccountFeatureFlag(accountId, feature);
      await load();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusyFeature(null);
    }
  };

  const filtered = useMemo(() => {
    if (!search.trim()) return flags;
    const q = search.toLowerCase();
    return flags.filter(
      f =>
        f.feature.toLowerCase().includes(q) ||
        f.display_name.toLowerCase().includes(q) ||
        (f.description || '').toLowerCase().includes(q)
    );
  }, [flags, search]);

  const isAdmin = user?.role === 'admin';

  return (
    <Layout>
      <div className="max-w-5xl mx-auto p-6">
        <div className="flex items-center gap-3 mb-2">
          <Flag className="h-6 w-6 text-brand-600" />
          <h1 className="text-2xl font-semibold text-text-primary">Feature Flags</h1>
        </div>
        <p className="text-sm text-text-secondary mb-6">
          Toggle experimental and released features for this account. Course-level
          flags inherit from the account unless overridden.
        </p>

        {!isAdmin && (
          <div className="mb-4 flex gap-2 items-start rounded-md border border-accent-warning/30 bg-accent-warning/10 p-3 text-sm text-accent-warning">
            <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
            You are viewing in read-only mode. Admin permission is required to change feature states.
          </div>
        )}

        <div className="flex gap-3 mb-4">
          <input
            type="text"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search features..."
            className="flex-1 rounded-md border border-border-strong px-3 py-2 text-sm focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
          />
          <button
            onClick={load}
            className="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-md border border-border-strong bg-surface-0 hover:bg-surface-1"
          >
            <RefreshCw className="h-4 w-4" />
            Reload
          </button>
        </div>

        {error && (
          <div className="mb-4 rounded-md border border-accent-danger/30 bg-accent-danger/10 p-3 text-sm text-accent-danger flex justify-between items-center">
            <span>{error}</span>
            <button onClick={load} className="underline">Try Again</button>
          </div>
        )}

        {loading ? (
          <div className="flex justify-center py-16">
            <Spinner />
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-text-tertiary text-sm">No matching features.</div>
        ) : (
          <ul className="divide-y divide-border-default rounded-lg border border-border-default bg-surface-0">
            {filtered.map(flag => (
              <li key={flag.feature} className="p-4 flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <h3 className="text-sm font-semibold text-text-primary">
                      {flag.display_name}
                    </h3>
                    <span
                      className={`text-[10px] uppercase tracking-wide font-semibold px-2 py-0.5 rounded ${STAGE_STYLES[flag.release_stage] || ''}`}
                    >
                      {flag.release_stage}
                    </span>
                    {flag.locked && (
                      <span className="inline-flex items-center gap-1 text-[10px] uppercase tracking-wide font-semibold px-2 py-0.5 rounded bg-rose-50 text-rose-800 ring-1 ring-rose-200">
                        <Lock className="h-3 w-3" />
                        locked by {flag.parent_context_type}
                      </span>
                    )}
                  </div>
                  <code className="text-[11px] text-text-tertiary font-mono">{flag.feature}</code>
                  <p className="text-sm text-text-secondary mt-1">{flag.description}</p>
                </div>
                <div className="flex flex-col items-end gap-2">
                  {busyFeature === flag.feature ? (
                    <Spinner />
                  ) : (
                    <StateToggle
                      flag={flag}
                      disabled={!isAdmin || flag.locked}
                      onChange={s => setState(flag.feature, s)}
                    />
                  )}
                  {isAdmin && !flag.locked && (
                    <button
                      onClick={() => reset(flag.feature)}
                      className="text-xs text-text-tertiary hover:text-text-secondary underline"
                    >
                      Reset to inherited
                    </button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Layout>
  );
};

export default FeatureFlagsPage;
