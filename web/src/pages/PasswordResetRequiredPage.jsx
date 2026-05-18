import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ShieldCheck, AlertTriangle } from 'lucide-react';
import BrandLogo from '../components/brand/BrandLogo';
import { useAuth } from '../contexts/AuthContext';

// PasswordResetRequiredPage is the Wave 1.6 follow-up surface for
// SIS / OneRoster-provisioned learners. Reached when /api/v1/login
// responds with {pending_token, must_reset_password: true}.
//
// Flow:
//   1. AuthContext stashed the pending token in sessionStorage under
//      'password_reset_pending_token'. This page reads it; if absent,
//      the user lands here directly and we surface a clear "log in
//      again" error.
//   2. User chooses a new password + confirms it.
//   3. POST /api/v1/auth/password/set with Bearer pending-token and
//      {new_password}. On success, the backend clears the
//      RequiresPasswordReset flag, mints a real session JWT (sets
//      paper_session cookie + returns user payload), and we navigate
//      to "/".
//
// The pending token has a 10-minute TTL; if it expires the user has
// to log in again — same recovery surface as before, just clearer.
export default function PasswordResetRequiredPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { finalizePasswordReset } = useAuth();
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [hasPendingToken, setHasPendingToken] = useState(true);

  useEffect(() => {
    const tok = sessionStorage.getItem('password_reset_pending_token');
    if (!tok) {
      setHasPendingToken(false);
      setError(t('pages.passwordReset.errors.noPending', 'No pending password-reset found. Please log in again.'));
    }
  }, [t]);

  const submit = async (e) => {
    e.preventDefault();
    setError(null);
    if (newPassword.length < 8) {
      setError(t('pages.passwordReset.errors.tooShort'));
      return;
    }
    if (newPassword !== confirmPassword) {
      setError(t('pages.passwordReset.errors.mismatch'));
      return;
    }
    setSubmitting(true);
    try {
      await finalizePasswordReset(newPassword);
      navigate('/');
    } catch (err) {
      setError(err?.message || 'Failed to set new password.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-1 px-4">
      <div className="w-full max-w-md rounded-lg bg-surface-0 shadow-xl border border-surface-raised p-6 space-y-4">
        <div className="flex flex-col items-center mb-2">
          <BrandLogo size={48} />
          <h1 className="mt-3 text-xl font-bold text-text-primary">
            {t('pages.passwordReset.title')}
          </h1>
        </div>

        <p className="text-sm text-text-secondary text-center">
          {t('pages.passwordReset.subtitle')}
        </p>

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-accent-warning bg-accent-warning/10 p-3 text-sm text-accent-warning" role="alert">
            <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={submit} className="space-y-3">
          <label className="block">
            <span className="block text-sm text-text-secondary mb-1">
              {t('pages.passwordReset.newPasswordLabel')}
            </span>
            <input
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              disabled={submitting || !hasPendingToken}
              className="w-full rounded border border-surface-raised bg-surface-1 px-3 py-2 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              required
              minLength={8}
            />
          </label>
          <label className="block">
            <span className="block text-sm text-text-secondary mb-1">
              {t('pages.passwordReset.confirmLabel')}
            </span>
            <input
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={submitting || !hasPendingToken}
              className="w-full rounded border border-surface-raised bg-surface-1 px-3 py-2 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              required
              minLength={8}
            />
          </label>
          <button
            type="submit"
            disabled={submitting || !hasPendingToken || !newPassword || !confirmPassword}
            className="w-full inline-flex items-center justify-center gap-2 rounded-md bg-brand-600 hover:bg-brand-700 px-4 py-2 text-white font-medium disabled:opacity-50"
          >
            <ShieldCheck className="w-4 h-4" aria-hidden="true" />
            {submitting ? '…' : t('pages.passwordReset.submitButton')}
          </button>
        </form>

        <button
          type="button"
          onClick={() => { sessionStorage.removeItem('password_reset_pending_token'); navigate('/login'); }}
          className="w-full text-xs text-text-secondary hover:text-text-primary"
        >
          {t('pages.passwordReset.cancelButton', 'Cancel and return to login')}
        </button>
      </div>
    </div>
  );
}
