import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ShieldCheck, AlertTriangle, KeyRound } from 'lucide-react';
import BrandLogo from '../components/brand/BrandLogo';

// MFAVerifyPage is the second-factor step at login. Reached when
// /api/v1/login responds with {pending_token, mfa_required: true}
// instead of {token, user}.
//
// Flow:
//   1. Read pending_token from sessionStorage (preferred) or query
//      string (?t=...) — the OIDC handler uses the query path; the
//      local-login handler stashes in sessionStorage from AuthContext.
//   2. User enters their 6-digit authenticator code.
//   3. POST /api/v1/auth/mfa/verify with {pending_token, code}.
//   4. On success: response contains {token, user} — the new session
//      cookie is set by the backend, so reload routes to the dashboard.
//   5. "I lost my device" → recovery-code mode → POST /auth/mfa/recovery.
//
// The pending token has a 5-minute TTL; if it expires the user has to
// log in again. The page surfaces this clearly instead of silently
// failing.
const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

export default function MFAVerifyPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [pendingToken, setPendingToken] = useState(null);
  const [code, setCode] = useState('');
  const [recoveryMode, setRecoveryMode] = useState(false);
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    const queryToken = params.get('t');
    const sessionToken = sessionStorage.getItem('mfa_pending_token');
    const tok = queryToken || sessionToken;
    if (!tok) {
      setError('No pending login found. Please log in again.');
      return;
    }
    setPendingToken(tok);
    // Clear from URL so a refresh doesn't reuse it (the token is
    // single-use anyway, but tidy URL).
    if (queryToken) {
      sessionStorage.setItem('mfa_pending_token', queryToken);
      window.history.replaceState({}, '', '/mfa/verify');
    }
  }, [params]);

  const submit = async (e) => {
    e.preventDefault();
    if (!pendingToken || !code) return;
    setSubmitting(true);
    setError(null);
    try {
      const endpoint = recoveryMode ? '/auth/mfa/recovery' : '/auth/mfa/verify';
      const res = await fetch(`${API_URL}${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ pending_token: pendingToken, code }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `request failed (${res.status})`);
      }
      // Backend set the paper_session cookie. Clear the pending token
      // and navigate to the dashboard. The AuthContext will refresh
      // the user on the next render.
      sessionStorage.removeItem('mfa_pending_token');
      window.location.href = '/';
    } catch (err) {
      setError(err.message || 'Verification failed.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-1 px-4">
      <div className="w-full max-w-md rounded-lg bg-surface-0 shadow-xl border border-surface-raised p-6 space-y-4">
        <div className="flex flex-col items-center mb-2">
          <BrandLogo size={48} />
          <h1 className="mt-3 text-xl font-bold text-text-primary">Two-factor sign in</h1>
        </div>

        {!recoveryMode ? (
          <p className="text-sm text-text-secondary text-center">
            Enter the 6-digit code from your authenticator app.
          </p>
        ) : (
          <p className="text-sm text-text-secondary text-center">
            Enter one of your saved recovery codes. Each code works once.
          </p>
        )}

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-accent-warning bg-accent-warning/10 p-3 text-sm text-accent-warning">
            <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={submit} className="space-y-3">
          <label className="block">
            <span className="sr-only">{recoveryMode ? 'Recovery code' : 'Authenticator code'}</span>
            <input
              type="text"
              inputMode={recoveryMode ? 'text' : 'numeric'}
              autoComplete="one-time-code"
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={recoveryMode ? 'XXXX-XXXX' : '123456'}
              className="w-full rounded border border-surface-raised bg-surface-1 px-3 py-2 text-text-primary text-center text-lg tracking-widest focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              disabled={submitting || !pendingToken}
            />
          </label>
          <button
            type="submit"
            disabled={submitting || !pendingToken || !code}
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium disabled:opacity-50"
          >
            {recoveryMode ? <KeyRound className="w-4 h-4" /> : <ShieldCheck className="w-4 h-4" />}
            {submitting ? 'Verifying…' : 'Verify'}
          </button>
        </form>

        <button
          type="button"
          onClick={() => { setRecoveryMode((m) => !m); setError(null); setCode(''); }}
          className="w-full text-xs text-text-secondary hover:text-text-primary"
        >
          {recoveryMode ? 'Have your authenticator? Enter the 6-digit code instead.' : 'I lost my device — use a recovery code.'}
        </button>

        <button
          type="button"
          onClick={() => { sessionStorage.removeItem('mfa_pending_token'); navigate('/login'); }}
          className="w-full text-xs text-text-secondary hover:text-text-primary"
        >
          Cancel and return to login
        </button>
      </div>
    </div>
  );
}
