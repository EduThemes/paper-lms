import React, { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';
import BrandLogo from '../components/brand/BrandLogo';
import { b64urlToBytes, bytesToB64url } from '../lib/webauthn';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

/* ─── Inline SVG Icons ─── */

const GoogleIcon = () => (
  <svg width="20" height="20" viewBox="0 0 48 48" aria-hidden="true">
    <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
    <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
    <path fill="#FBBC05" d="M10.53 28.59a14.5 14.5 0 0 1 0-9.18l-7.98-6.19a24.04 24.04 0 0 0 0 21.56l7.98-6.19z" />
    <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
  </svg>
);

const MicrosoftIcon = () => (
  <svg width="20" height="20" viewBox="0 0 21 21" aria-hidden="true">
    <rect x="1" y="1" width="9" height="9" fill="#F25022" />
    <rect x="11" y="1" width="9" height="9" fill="#7FBA00" />
    <rect x="1" y="11" width="9" height="9" fill="#00A4EF" />
    <rect x="11" y="11" width="9" height="9" fill="#FFB900" />
  </svg>
);

const CleverIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="12" fill="#4274F6" />
    <path d="M15.5 8.5 L10 12 L15.5 15.5" stroke="#fff" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

const ClassLinkIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="12" fill="#2E7D32" />
    <path d="M7 12 L12 7 L17 12 L12 17 Z" fill="#fff" />
    <circle cx="12" cy="12" r="2" fill="#2E7D32" />
  </svg>
);

// Apple Sign-In glyph — official Apple-recommended monochrome variant.
const AppleIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.81-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z"/>
  </svg>
);

const DefaultProviderIcon = () => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
    <polyline points="10 17 15 12 10 7" />
    <line x1="15" y1="12" x2="3" y2="12" />
  </svg>
);

/* ─── Provider Styling ─── */
// Brand colors below (#2F2F2F, #4274F6, #2E7D32, etc.) are intentional vendor SSO button colors
// per Microsoft/Clever/ClassLink brand guidelines. Do not replace with theme tokens.

const PROVIDER_STYLES = {
  google: {
    bg: 'bg-surface-0',
    border: 'border border-border-strong',
    text: 'text-text-secondary',
    hover: 'hover:bg-surface-1 hover:shadow-sm',
    icon: GoogleIcon,
    label: 'Sign in with Google',
  },
  microsoft: {
    // Brand color: vendor SSO button (Microsoft)
    bg: 'bg-[#2F2F2F]',
    border: 'border border-[#2F2F2F]',
    text: 'text-white',
    hover: 'hover:bg-[#1a1a1a]',
    icon: MicrosoftIcon,
    label: 'Sign in with Microsoft',
  },
  clever: {
    // Brand color: vendor SSO button (Clever)
    bg: 'bg-[#4274F6]',
    border: 'border border-[#4274F6]',
    text: 'text-white',
    hover: 'hover:bg-[#3361D8]',
    icon: CleverIcon,
    label: 'Sign in with Clever',
  },
  classlink: {
    // Brand color: vendor SSO button (ClassLink)
    bg: 'bg-[#2E7D32]',
    border: 'border border-[#2E7D32]',
    text: 'text-white',
    hover: 'hover:bg-[#1B5E20]',
    icon: ClassLinkIcon,
    label: 'Sign in with ClassLink',
  },
  apple: {
    // Brand color: Apple Sign-In monochrome (Apple HIG recommends
    // black bg + white glyph + "Sign in with Apple" wordmark).
    bg: 'bg-black',
    border: 'border border-black',
    text: 'text-white',
    hover: 'hover:bg-neutral-900',
    icon: AppleIcon,
    label: 'Sign in with Apple',
  },
};

const getProviderStyle = (provider) => {
  const key = (provider.name || '').toLowerCase().replace(/\s+/g, '');
  for (const [name, style] of Object.entries(PROVIDER_STYLES)) {
    if (key.includes(name)) return style;
  }
  // Brand color: vendor SSO button (generic fallback — neutral dark button for unknown providers)
  return {
    bg: 'bg-text-primary',
    border: 'border border-text-primary',
    text: 'text-white',
    hover: 'hover:bg-text-secondary',
    icon: DefaultProviderIcon,
    label: `Sign in with ${provider.name}`,
  };
};

/* ─── Paper LMS Logo ─── */

const PaperLogo = () => (
  <div className="flex flex-col items-center mb-2">
    <div className="mb-3">
      <BrandLogo size={72} />
    </div>
    <h1 className="text-2xl font-bold text-text-primary">Paper LMS</h1>
  </div>
);

/* ─── Main Component ─── */

const LoginPageSSO = () => {
  const [error, setError] = useState(null);
  const [successMsg, setSuccessMsg] = useState(null);
  const [isRegister, setIsRegister] = useState(false);
  const [view, setView] = useState('login'); // 'login' | 'forgotPassword' | 'resetPassword'
  const [resetToken, setResetToken] = useState('');
  const [providers, setProviders] = useState([]);
  const [providersLoading, setProvidersLoading] = useState(true);
  const { login, register } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  // Check for reset token in URL query param
  useEffect(() => {
    const token = searchParams.get('reset_token');
    if (token) {
      setResetToken(token);
      setView('resetPassword');
    }
  }, [searchParams]);

  // Fetch configured SSO providers
  useEffect(() => {
    const fetchProviders = async () => {
      try {
        const res = await fetch(`${API_URL}/auth/providers`, {
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
        });
        if (res.ok) {
          const data = await res.json();
          const list = Array.isArray(data) ? data : data.data || [];
          // Sort by position
          list.sort((a, b) => (a.position || 0) - (b.position || 0));
          setProviders(list);
        }
      } catch {
        // Silently fail — SSO buttons simply won't appear
      } finally {
        setProvidersLoading(false);
      }
    };
    fetchProviders();
  }, []);

  // Phase 10-B — passkey-as-primary. Click → discoverable login;
  // the browser dialog offers all passkeys for this site. On
  // success, server mints a session cookie and we route to /.
  const passkeySupported = typeof window !== 'undefined' && window.PublicKeyCredential;
  const [passkeyBusy, setPasskeyBusy] = useState(false);
  const handlePasskeyLogin = async () => {
    setError(null);
    setPasskeyBusy(true);
    try {
      const beginRes = await fetch(`${API_URL}/auth/passkey/begin`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
      });
      if (!beginRes.ok) throw new Error(`could not start passkey login (${beginRes.status})`);
      const { options } = await beginRes.json();
      const publicKey = {
        ...options.publicKey,
        challenge: b64urlToBytes(options.publicKey.challenge),
        allowCredentials: (options.publicKey.allowCredentials || []).map((c) => ({
          ...c,
          id: b64urlToBytes(c.id),
        })),
      };
      const cred = await navigator.credentials.get({ publicKey });
      if (!cred) throw new Error('No credential returned by the authenticator.');
      const payload = {
        id: cred.id,
        rawId: bytesToB64url(cred.rawId),
        type: cred.type,
        response: {
          authenticatorData: bytesToB64url(cred.response.authenticatorData),
          clientDataJSON: bytesToB64url(cred.response.clientDataJSON),
          signature: bytesToB64url(cred.response.signature),
          userHandle: cred.response.userHandle ? bytesToB64url(cred.response.userHandle) : null,
        },
        clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
      };
      const finishRes = await fetch(`${API_URL}/auth/passkey/finish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload),
      });
      if (!finishRes.ok) {
        const body = await finishRes.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || `passkey login failed (${finishRes.status})`);
      }
      // Session cookie is set; the AuthContext will pick up the user
      // on its next refresh. Navigate to the dashboard.
      navigate('/');
      // Hard reload so AuthContext re-reads /users/self.
      window.location.reload();
    } catch (err) {
      setError(err.message || 'Passkey login failed.');
    } finally {
      setPasskeyBusy(false);
    }
  };

  const handleSSOLogin = (provider) => {
    // Route by auth_type to the protocol-specific begin endpoint.
    // The pre-9 codebase had a "/auth/sso/:id/login" placeholder that
    // never existed server-side — every protocol has its own URL.
    const id = provider.id;
    const t = provider.auth_type;
    if (t === 'saml') {
      window.location.href = `${API_URL}/auth/saml/login?provider_id=${id}`;
    } else if (t === 'cas') {
      window.location.href = `${API_URL}/auth/cas/login?provider_id=${id}`;
    } else if (t === 'oidc') {
      window.location.href = `${API_URL}/auth/oidc/login?provider_id=${id}`;
    } else if (t === 'ldap') {
      // LDAP login is a POST with credentials; the button should
      // expand a username/password panel rather than redirect.
      // For now, route to a stub; LDAP buttons are usually unused
      // (LDAP is typically the local-password fallback for schools
      // already running OpenLDAP / AD).
      window.location.href = `${API_URL}/auth/ldap/login?provider_id=${id}`;
    } else {
      // Fallback: best-effort SAML route (the most common SSO type
      // configured against legacy Canvas-imported providers).
      window.location.href = `${API_URL}/auth/saml/login?provider_id=${id}`;
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);
    setSuccessMsg(null);

    const formData = new FormData(e.target);
    const email = formData.get('email');
    const password = formData.get('password');

    try {
      if (isRegister) {
        const name = formData.get('name');
        await register(name, email, password);
        navigate('/');
        return;
      }
      // Phase 9-B: login() returns the raw response. If the tenant
      // requires MFA, the response is {mfa_required: true, pending_token}
      // — AuthContext stashes the pending token; we route to the
      // verify page. If must_enroll_mfa is set, real session was
      // issued but the user needs to enroll before continuing.
      const data = await login(email, password);
      if (data?.mfa_required) {
        navigate('/mfa/verify');
        return;
      }
      if (data?.must_enroll_mfa) {
        navigate('/mfa/enroll');
        return;
      }
      navigate('/');
    } catch (err) {
      setError(err.message);
    }
  };

  const handleForgotPassword = async (e) => {
    e.preventDefault();
    setError(null);
    setSuccessMsg(null);
    const formData = new FormData(e.target);
    const email = formData.get('email');
    try {
      await api.requestPasswordReset(email);
      setSuccessMsg('If an account exists with that email, a password reset link has been sent. Check your email or contact your administrator.');
    } catch (err) {
      setError(err.message);
    }
  };

  const handleResetPassword = async (e) => {
    e.preventDefault();
    setError(null);
    setSuccessMsg(null);
    const formData = new FormData(e.target);
    const token = formData.get('token');
    const newPassword = formData.get('new_password');
    const confirmPassword = formData.get('confirm_password');
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    try {
      await api.resetPassword(token, newPassword);
      setSuccessMsg('Password has been reset successfully. You can now log in.');
      setView('login');
      setResetToken('');
    } catch (err) {
      setError(err.message);
    }
  };

  const toggleMode = () => {
    setIsRegister(!isRegister);
    setView('login');
    setError(null);
    setSuccessMsg(null);
  };

  const goToForgotPassword = () => {
    setView('forgotPassword');
    setError(null);
    setSuccessMsg(null);
    setIsRegister(false);
  };

  const goToLogin = () => {
    setView('login');
    setError(null);
    setSuccessMsg(null);
    setIsRegister(false);
  };

  return (
    <div className="min-h-screen bg-surface-1 flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-md">
        <div className="bg-surface-0 rounded-2xl shadow-xl p-8">
          <PaperLogo />
          <p className="text-center text-text-tertiary text-sm mb-6">
            {view === 'forgotPassword' ? 'Reset your password' :
             view === 'resetPassword' ? 'Set a new password' :
             isRegister ? 'Create your account' : 'Sign in to continue'}
          </p>

          {/* ── Passkey Sign-in (Phase 10-B) ── */}
          {view === 'login' && !isRegister && passkeySupported && (
            <div className="mb-4">
              <button
                type="button"
                onClick={handlePasskeyLogin}
                disabled={passkeyBusy}
                className="w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg font-medium text-sm transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 bg-surface-0 border border-border-strong text-text-secondary hover:bg-surface-1 hover:shadow-sm disabled:opacity-50"
                aria-label="Sign in with a passkey"
              >
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <circle cx="8" cy="11" r="5" />
                  <path d="M13 11h7" />
                  <path d="M20 11v3" />
                  <path d="M17 14v-3" />
                </svg>
                <span>{passkeyBusy ? 'Waiting for device…' : 'Sign in with a passkey'}</span>
              </button>
            </div>
          )}

          {/* ── SSO Provider Buttons ── */}
          {view === 'login' && !isRegister && providers.length > 0 && (
            <div className="space-y-3 mb-6" role="group" aria-label="Single sign-on options">
              {providers.map((provider) => {
                const style = getProviderStyle(provider);
                const IconComponent = style.icon;
                return (
                  <button
                    key={provider.id}
                    type="button"
                    onClick={() => handleSSOLogin(provider)}
                    className={`
                      w-full flex items-center justify-center gap-3 px-4 py-2.5 rounded-lg
                      font-medium text-sm transition-all duration-150
                      focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2
                      ${style.bg} ${style.border} ${style.text} ${style.hover}
                    `}
                    aria-label={style.label}
                  >
                    {provider.icon_url ? (
                      <img
                        src={provider.icon_url}
                        alt=""
                        className="w-5 h-5"
                        aria-hidden="true"
                      />
                    ) : (
                      <IconComponent />
                    )}
                    <span>{style.label}</span>
                  </button>
                );
              })}
            </div>
          )}

          {/* ── Divider ── */}
          {view === 'login' && !isRegister && providers.length > 0 && (
            <div className="relative my-6" role="separator">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-border-default" />
              </div>
              <div className="relative flex justify-center text-sm">
                <span className="bg-surface-0 px-4 text-text-disabled">or sign in with email</span>
              </div>
            </div>
          )}

          {/* ── Loading SSO providers ── */}
          {view === 'login' && !isRegister && providersLoading && (
            <div className="flex justify-center mb-4">
              <div className="h-5 w-5 border-2 border-brand-600 border-t-transparent rounded-full animate-spin" aria-label="Loading sign-in options" role="status" />
            </div>
          )}

          {/* ── Success Message ── */}
          {successMsg && (
            <div
              className="mb-4 p-3 bg-accent-success/10 border border-accent-success/30 rounded-lg text-accent-success text-sm text-center"
              role="status"
              aria-live="polite"
            >
              {successMsg}
            </div>
          )}

          {/* ── Error Display ── */}
          {error && (
            <div
              className="mb-4 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg text-accent-danger text-sm text-center"
              role="alert"
              aria-live="assertive"
            >
              {error}
            </div>
          )}

          {/* ── Forgot Password Form ── */}
          {view === 'forgotPassword' && (
            <>
              <p className="text-sm text-text-tertiary mb-4 text-center">
                Enter your email address and we'll send you instructions to reset your password.
              </p>
              <form onSubmit={handleForgotPassword} noValidate>
                <div className="space-y-4">
                  <div>
                    <label htmlFor="reset-email" className="block text-sm font-medium text-text-secondary mb-1">
                      Email Address
                    </label>
                    <input
                      id="reset-email"
                      type="email"
                      name="email"
                      autoComplete="email"
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                      placeholder="you@school.edu"
                      required
                      aria-required="true"
                    />
                  </div>
                  <button
                    type="submit"
                    className="w-full bg-brand-600 text-white py-2.5 px-4 rounded-lg font-medium hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 transition-colors"
                  >
                    Request Password Reset
                  </button>
                </div>
              </form>
              <p className="mt-5 text-center text-sm text-text-tertiary">
                <button type="button" onClick={goToLogin} className="text-brand-600 font-medium hover:underline focus:outline-none focus:underline">
                  Back to sign in
                </button>
              </p>
            </>
          )}

          {/* ── Reset Password Form (with token) ── */}
          {view === 'resetPassword' && (
            <>
              <form onSubmit={handleResetPassword} noValidate>
                <div className="space-y-4">
                  <div>
                    <label htmlFor="reset-token" className="block text-sm font-medium text-text-secondary mb-1">
                      Reset Token
                    </label>
                    <input
                      id="reset-token"
                      type="text"
                      name="token"
                      defaultValue={resetToken}
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors font-mono text-sm"
                      placeholder="Paste your reset token"
                      required
                      aria-required="true"
                    />
                  </div>
                  <div>
                    <label htmlFor="new-password" className="block text-sm font-medium text-text-secondary mb-1">
                      New Password
                    </label>
                    <input
                      id="new-password"
                      type="password"
                      name="new_password"
                      autoComplete="new-password"
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                      placeholder="At least 8 characters"
                      required
                      aria-required="true"
                      minLength={8}
                    />
                  </div>
                  <div>
                    <label htmlFor="confirm-password" className="block text-sm font-medium text-text-secondary mb-1">
                      Confirm Password
                    </label>
                    <input
                      id="confirm-password"
                      type="password"
                      name="confirm_password"
                      autoComplete="new-password"
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                      placeholder="Re-enter your new password"
                      required
                      aria-required="true"
                      minLength={8}
                    />
                  </div>
                  <button
                    type="submit"
                    className="w-full bg-brand-600 text-white py-2.5 px-4 rounded-lg font-medium hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 transition-colors"
                  >
                    Reset Password
                  </button>
                </div>
              </form>
              <p className="mt-5 text-center text-sm text-text-tertiary">
                <button type="button" onClick={goToLogin} className="text-brand-600 font-medium hover:underline focus:outline-none focus:underline">
                  Back to sign in
                </button>
              </p>
            </>
          )}

          {/* ── Email / Password Form ── */}
          {view === 'login' && (
            <>
              <form onSubmit={handleSubmit} noValidate>
                <div className="space-y-4">
                  {isRegister && (
                    <div>
                      <label htmlFor="sso-name" className="block text-sm font-medium text-text-secondary mb-1">
                        Full Name
                      </label>
                      <input
                        id="sso-name"
                        type="text"
                        name="name"
                        autoComplete="name"
                        className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                        placeholder="Jane Doe"
                        required
                        aria-required="true"
                      />
                    </div>
                  )}
                  <div>
                    <label htmlFor="sso-email" className="block text-sm font-medium text-text-secondary mb-1">
                      Email Address
                    </label>
                    <input
                      id="sso-email"
                      type="email"
                      name="email"
                      autoComplete="email"
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                      placeholder="you@school.edu"
                      required
                      aria-required="true"
                    />
                  </div>
                  <div>
                    <label htmlFor="sso-password" className="block text-sm font-medium text-text-secondary mb-1">
                      Password
                    </label>
                    <input
                      id="sso-password"
                      type="password"
                      name="password"
                      autoComplete={isRegister ? 'new-password' : 'current-password'}
                      className="block w-full rounded-lg border border-border-strong px-3 py-2.5 text-text-primary placeholder:text-text-disabled focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-colors"
                      placeholder="Enter your password"
                      required
                      aria-required="true"
                    />
                  </div>
                  {!isRegister && (
                    <div className="text-right">
                      <button
                        type="button"
                        onClick={goToForgotPassword}
                        className="text-sm text-brand-600 hover:underline focus:outline-none focus:underline"
                      >
                        Forgot password?
                      </button>
                    </div>
                  )}
                  <button
                    type="submit"
                    className="w-full bg-brand-600 text-white py-2.5 px-4 rounded-lg font-medium hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 transition-colors"
                  >
                    {isRegister ? 'Create Account' : 'Log In'}
                  </button>
                </div>
              </form>

              {/* ── Toggle Register / Login ── */}
              <p className="mt-5 text-center text-sm text-text-tertiary">
                {isRegister ? 'Already have an account? ' : "Don't have an account? "}
                <button
                  type="button"
                  onClick={toggleMode}
                  className="text-brand-600 font-medium hover:underline focus:outline-none focus:underline"
                >
                  {isRegister ? 'Sign in' : 'Register'}
                </button>
              </p>
            </>
          )}

          {/* ── COPPA Notice ── */}
          <div className="mt-6 pt-5 border-t border-border-subtle">
            <p className="text-xs text-text-disabled text-center leading-relaxed">
              By signing in, you agree to our{' '}
              <a href="/privacy" className="text-brand-500 hover:underline focus:underline focus:outline-none">
                Privacy Policy
              </a>{' '}
              and{' '}
              <a href="/terms" className="text-brand-500 hover:underline focus:underline focus:outline-none">
                Terms of Service
              </a>.
              <br />
              Students under 13 require parental consent.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LoginPageSSO;
