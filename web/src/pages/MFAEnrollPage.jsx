import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldCheck, AlertTriangle, Copy, Check } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import Layout from '../components/Layout';
import { getCSRFToken } from '../services/api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

// MFAEnrollPage walks the user through TOTP enrollment in three steps:
//   1. Password re-prompt (server requires this to start enrollment).
//   2. Display otpauth URL (rendered as a QR via an external CDN
//      script for now — full client-side QR rendering is a polish
//      task) + the plaintext secret + recovery codes ONCE.
//   3. User enters first 6-digit code from their app → verify.
//
// The page is intentionally one component with explicit step state
// rather than nested routes — losing your place mid-enrollment
// because of a back-button click would leave you with an
// in-progress enrollment + nowhere to verify from. Single page,
// single state machine.
export default function MFAEnrollPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState(1); // 1 password, 2 QR, 3 verify, 4 done
  const [password, setPassword] = useState('');
  const [enrollData, setEnrollData] = useState(null); // { otpauth_url, secret, recovery_codes }
  const [code, setCode] = useState('');
  const [savedCodes, setSavedCodes] = useState(false);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [copiedSecret, setCopiedSecret] = useState(false);

  const doEnroll = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/users/self/mfa/enroll`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        credentials: 'include',
        body: JSON.stringify({ password }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `enroll failed (${res.status})`);
      }
      const data = await res.json();
      setEnrollData(data);
      setStep(2);
    } catch (err) {
      setError(err.message || 'Enrollment failed.');
    } finally {
      setBusy(false);
    }
  };

  const doVerify = async (e) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/users/self/mfa/verify-enrollment`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        credentials: 'include',
        body: JSON.stringify({ code }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `verify failed (${res.status})`);
      }
      setStep(4);
    } catch (err) {
      setError(err.message || 'Code did not match. Try again.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Layout>
      <div className="max-w-xl mx-auto py-8 space-y-4">
        <header className="flex items-center gap-3">
          <ShieldCheck className="w-6 h-6 text-text-primary" aria-hidden="true" />
          <h1 className="text-2xl font-semibold text-text-primary">Set up two-factor sign in</h1>
        </header>

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-accent-warning bg-accent-warning/10 p-3 text-sm text-accent-warning">
            <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
            <span>{error}</span>
          </div>
        )}

        {step === 1 && (
          <form onSubmit={doEnroll} className="space-y-3 rounded-lg border border-surface-raised bg-surface-0 p-5">
            <p className="text-sm text-text-secondary">
              Confirm your password to begin. We need this so a stolen browser session can't enroll you and lock you out.
            </p>
            <label className="block">
              <span className="text-sm text-text-secondary">Password</span>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoFocus
                className="mt-1 w-full rounded border border-surface-raised bg-surface-1 px-3 py-2"
              />
            </label>
            <button
              type="submit"
              disabled={busy || !password}
              className="rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium disabled:opacity-50"
            >
              {busy ? 'Working…' : 'Continue'}
            </button>
          </form>
        )}

        {step === 2 && enrollData && (
          <div className="space-y-4 rounded-lg border border-surface-raised bg-surface-0 p-5">
            <div>
              <h2 className="text-base font-semibold text-text-primary">Scan this with your authenticator</h2>
              <p className="text-sm text-text-secondary mt-1">
                Use Google Authenticator, Authy, 1Password, Microsoft Authenticator, or any app that supports TOTP.
              </p>
            </div>
            <div className="flex justify-center bg-white p-4 rounded">
              {/* Inline SVG QR via qrcode.react — no external network
                  call (Sprint 10-A.3). The otpauth URL contains the
                  user's TOTP secret; sending it to a third-party QR
                  service would be a real data leak. */}
              <QRCodeSVG
                value={enrollData.otpauth_url}
                size={200}
                level="M"
                aria-label="TOTP QR code"
              />
            </div>
            <details className="text-xs text-text-secondary">
              <summary className="cursor-pointer">Can't scan? Enter this key manually.</summary>
              <div className="mt-2 flex items-center gap-2">
                <code className="bg-surface-1 px-2 py-1 rounded font-mono">{enrollData.secret}</code>
                <button
                  type="button"
                  onClick={() => { navigator.clipboard.writeText(enrollData.secret); setCopiedSecret(true); }}
                  className="inline-flex items-center gap-1 text-brand-700 hover:text-brand-800"
                >
                  {copiedSecret ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                  {copiedSecret ? 'Copied' : 'Copy'}
                </button>
              </div>
            </details>

            <div className="border-t border-surface-raised pt-4">
              <h3 className="text-sm font-semibold text-text-primary">Save these recovery codes</h3>
              <p className="text-xs text-text-secondary mt-1">
                Each code works exactly once. Store them somewhere safe (password manager, printed). If you lose your phone, these are how you get back in.
              </p>
              <pre className="mt-3 bg-surface-1 rounded p-3 text-sm font-mono whitespace-pre-wrap">{(enrollData.recovery_codes || []).join('\n')}</pre>
              <label className="mt-3 flex items-center gap-2 text-sm">
                <input type="checkbox" checked={savedCodes} onChange={(e) => setSavedCodes(e.target.checked)} />
                I've saved the recovery codes somewhere safe.
              </label>
            </div>

            <button
              type="button"
              disabled={!savedCodes}
              onClick={() => setStep(3)}
              className="rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium disabled:opacity-50"
            >
              Continue
            </button>
          </div>
        )}

        {step === 3 && (
          <form onSubmit={doVerify} className="space-y-3 rounded-lg border border-surface-raised bg-surface-0 p-5">
            <h2 className="text-base font-semibold text-text-primary">Confirm the setup worked</h2>
            <p className="text-sm text-text-secondary">Enter the current 6-digit code from your authenticator app.</p>
            <input
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="123456"
              className="w-full rounded border border-surface-raised bg-surface-1 px-3 py-2 text-center text-lg tracking-widest"
            />
            <button
              type="submit"
              disabled={busy || !code}
              className="rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium disabled:opacity-50"
            >
              {busy ? 'Verifying…' : 'Verify and enable'}
            </button>
          </form>
        )}

        {step === 4 && (
          <div className="rounded-lg border border-accent-success bg-accent-success/10 p-5 space-y-3">
            <div className="flex items-center gap-2 text-accent-success">
              <Check className="w-5 h-5" />
              <h2 className="text-base font-semibold">Two-factor sign in is on.</h2>
            </div>
            <p className="text-sm text-text-primary">
              Next time you sign in, you'll be asked for a code from your authenticator app.
            </p>
            <button
              type="button"
              onClick={() => navigate('/')}
              className="rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium"
            >
              Back to dashboard
            </button>
          </div>
        )}
      </div>
    </Layout>
  );
}
