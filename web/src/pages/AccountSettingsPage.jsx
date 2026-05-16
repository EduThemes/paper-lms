import React, { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  User, KeyRound, ShieldCheck, Bell, Key, Shield, Save, Check, AlertTriangle,
  Smartphone, Fingerprint, ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

const TABS = [
  { id: 'profile', label: 'Profile', icon: User },
  { id: 'password', label: 'Password', icon: KeyRound },
  { id: 'security', label: 'Security', icon: ShieldCheck },
  { id: 'notifications', label: 'Notifications', icon: Bell },
  { id: 'tokens', label: 'Access tokens', icon: Key },
  { id: 'privacy', label: 'Data & privacy', icon: Shield },
];

function Banner({ kind, children }) {
  if (!children) return null;
  const palette = kind === 'success'
    ? 'bg-success/10 text-success border-success/30'
    : 'bg-danger/10 text-danger border-danger/30';
  const Icon = kind === 'success' ? Check : AlertTriangle;
  return (
    <div className={`flex items-start gap-2 rounded-md border px-3 py-2 text-sm ${palette}`}>
      <Icon className="w-4 h-4 mt-0.5 shrink-0" />
      <div>{children}</div>
    </div>
  );
}

function ProfileTab({ user, onSaved }) {
  const [name, setName] = useState(user?.name || '');
  const [locale, setLocale] = useState(user?.locale || 'en');
  const [timeZone, setTimeZone] = useState(user?.time_zone || '');
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const submit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError('');
    setSuccess('');
    try {
      await api.updateSelf(user.id, { name, locale, time_zone: timeZone });
      setSuccess('Profile updated.');
      onSaved?.();
    } catch (err) {
      setError(err?.message || 'Could not save profile');
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-4 max-w-xl">
      <h2 className="text-lg font-semibold">Profile</h2>
      <Banner kind="success">{success}</Banner>
      <Banner kind="error">{error}</Banner>
      <div>
        <label className="block text-sm font-medium mb-1">Name</label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
        />
      </div>
      <div>
        <label className="block text-sm font-medium mb-1">Email</label>
        <input
          type="email"
          value={user?.email || ''}
          disabled
          className="w-full rounded-md border border-border-default bg-surface-1 px-3 py-2 text-sm text-text-secondary"
        />
        <p className="mt-1 text-xs text-text-secondary">
          Email changes go through your administrator.
        </p>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium mb-1">Locale</label>
          <select
            value={locale}
            onChange={(e) => setLocale(e.target.value)}
            className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
          >
            <option value="en">English</option>
            <option value="es">Español</option>
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium mb-1">Time zone</label>
          <input
            type="text"
            value={timeZone}
            onChange={(e) => setTimeZone(e.target.value)}
            placeholder="America/Chicago"
            className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
          />
        </div>
      </div>
      <button
        type="submit"
        disabled={saving}
        className="inline-flex items-center gap-2 rounded-md bg-brand-primary text-white px-4 py-2 text-sm font-medium disabled:opacity-50"
      >
        <Save className="w-4 h-4" />
        {saving ? 'Saving…' : 'Save profile'}
      </button>
    </form>
  );
}

function PasswordTab() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const submit = async (e) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    if (next.length < 8) {
      setError('New password must be at least 8 characters.');
      return;
    }
    if (next !== confirm) {
      setError('New passwords do not match.');
      return;
    }
    setSaving(true);
    try {
      await api.changePassword(current, next);
      setSuccess('Password changed. Existing sessions remain valid until they expire.');
      setCurrent(''); setNext(''); setConfirm('');
    } catch (err) {
      setError(err?.message || 'Could not change password');
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-4 max-w-md">
      <h2 className="text-lg font-semibold">Change password</h2>
      <Banner kind="success">{success}</Banner>
      <Banner kind="error">{error}</Banner>
      <div>
        <label className="block text-sm font-medium mb-1">Current password</label>
        <input
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          required
          className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
        />
      </div>
      <div>
        <label className="block text-sm font-medium mb-1">New password</label>
        <input
          type="password"
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          required
          minLength={8}
          className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
        />
        <p className="mt-1 text-xs text-text-secondary">At least 8 characters.</p>
      </div>
      <div>
        <label className="block text-sm font-medium mb-1">Confirm new password</label>
        <input
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
          minLength={8}
          className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm"
        />
      </div>
      <button
        type="submit"
        disabled={saving}
        className="inline-flex items-center gap-2 rounded-md bg-brand-primary text-white px-4 py-2 text-sm font-medium disabled:opacity-50"
      >
        <Save className="w-4 h-4" />
        {saving ? 'Updating…' : 'Update password'}
      </button>
    </form>
  );
}

function LinkCard({ to, icon: Icon, title, description }) {
  return (
    <Link
      to={to}
      className="flex items-center gap-3 rounded-md border border-border-default bg-surface-0 px-4 py-3 hover:bg-surface-1 transition"
    >
      <Icon className="w-5 h-5 text-text-secondary shrink-0" />
      <div className="flex-1">
        <div className="text-sm font-medium">{title}</div>
        <div className="text-xs text-text-secondary">{description}</div>
      </div>
      <ChevronRight className="w-4 h-4 text-text-secondary" />
    </Link>
  );
}

function SecurityTab() {
  return (
    <div className="space-y-3 max-w-xl">
      <h2 className="text-lg font-semibold">Security</h2>
      <p className="text-sm text-text-secondary">
        Add a second factor so a stolen password isn't enough.
      </p>
      <LinkCard
        to="/mfa/enroll"
        icon={Smartphone}
        title="Authenticator app (TOTP)"
        description="Use Google Authenticator, 1Password, Authy, or any TOTP app."
      />
      <LinkCard
        to="/profile/passkeys"
        icon={Fingerprint}
        title="Passkeys"
        description="Sign in with Touch ID, Windows Hello, or a hardware key. No password needed."
      />
    </div>
  );
}

function NotificationsTab() {
  return (
    <div className="space-y-3 max-w-xl">
      <h2 className="text-lg font-semibold">Notifications</h2>
      <p className="text-sm text-text-secondary">
        Pick how often each kind of notification reaches you.
      </p>
      <LinkCard
        to="/settings/notifications"
        icon={Bell}
        title="Notification preferences"
        description="Set immediate, daily, weekly, or never per notification type."
      />
      <LinkCard
        to="/settings/notification_deliveries"
        icon={Bell}
        title="Recent deliveries & channels"
        description="See what's been sent and manage email/SMS channels."
      />
    </div>
  );
}

function TokensTab() {
  return (
    <div className="space-y-3 max-w-xl">
      <h2 className="text-lg font-semibold">Access tokens</h2>
      <p className="text-sm text-text-secondary">
        Generate personal access tokens for the Canvas-compatible API or third-party integrations.
      </p>
      <LinkCard
        to="/settings/tokens"
        icon={Key}
        title="Manage access tokens"
        description="Create, rename, and revoke API tokens."
      />
    </div>
  );
}

function PrivacyTab() {
  return (
    <div className="space-y-3 max-w-xl">
      <h2 className="text-lg font-semibold">Data &amp; privacy</h2>
      <p className="text-sm text-text-secondary">
        Manage what we keep about you and request a copy or deletion of your data.
      </p>
      <LinkCard
        to="/profile/gamification"
        icon={Shield}
        title="Leaderboard &amp; gamification visibility"
        description="Opt out of class leaderboards or change your displayed name."
      />
      <div className="rounded-md border border-border-default bg-surface-0 px-4 py-3">
        <div className="text-sm font-medium">Export my data</div>
        <p className="text-xs text-text-secondary mt-1">
          Email your administrator to request a full export. Self-service export is planned for a future release.
        </p>
      </div>
      <div className="rounded-md border border-danger/30 bg-danger/5 px-4 py-3">
        <div className="text-sm font-medium text-danger">Delete my account</div>
        <p className="text-xs text-text-secondary mt-1">
          Account deletion is admin-mediated for FERPA / minor-account safety. Contact your administrator to start the process.
        </p>
      </div>
    </div>
  );
}

const AccountSettingsPage = () => {
  const { user, refreshUser } = useAuth();
  const [params, setParams] = useSearchParams();
  const tab = useMemo(() => {
    const t = params.get('tab') || 'profile';
    return TABS.find((x) => x.id === t)?.id || 'profile';
  }, [params]);

  const setTab = (id) => {
    const next = new URLSearchParams(params);
    if (id === 'profile') next.delete('tab'); else next.set('tab', id);
    setParams(next, { replace: true });
  };

  useEffect(() => {
    // Make sure we have the latest user shape when this page mounts.
    refreshUser?.();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!user) {
    return (
      <Layout>
        <div className="p-6 text-text-secondary">Loading account…</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="p-6 max-w-5xl mx-auto">
        <h1 className="text-2xl font-bold mb-1">Account settings</h1>
        <p className="text-sm text-text-secondary mb-6">
          Manage your profile, password, and security options.
        </p>
        <div className="grid grid-cols-1 md:grid-cols-[200px_1fr] gap-6">
          <nav aria-label="Settings sections" className="space-y-1">
            {TABS.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                type="button"
                onClick={() => setTab(id)}
                className={`w-full flex items-center gap-2 rounded-md px-3 py-2 text-sm transition ${
                  tab === id
                    ? 'bg-brand-primary/10 text-brand-primary font-medium'
                    : 'text-text-primary hover:bg-surface-1'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{label}</span>
              </button>
            ))}
          </nav>
          <section>
            {tab === 'profile' && <ProfileTab user={user} onSaved={() => refreshUser?.()} />}
            {tab === 'password' && <PasswordTab />}
            {tab === 'security' && <SecurityTab />}
            {tab === 'notifications' && <NotificationsTab />}
            {tab === 'tokens' && <TokensTab />}
            {tab === 'privacy' && <PrivacyTab />}
          </section>
        </div>
      </div>
    </Layout>
  );
};

export default AccountSettingsPage;
