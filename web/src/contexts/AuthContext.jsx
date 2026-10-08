import React, { createContext, useState, useContext, useEffect } from 'react';
import i18n from 'i18next';
import { api } from '../services/api';
import { pickLocale } from '../utils/localePreference';

const AuthContext = createContext(null);

// Apply the signed-in user's locale. Priority (see pickLocale): the user's
// saved profile locale (B210 — it was saved in Account Settings but never
// applied, so it did not follow the user to other devices), then an
// explicit choice made on this device (localStorage 'paperlms_locale'),
// then the account's default_locale.
const readDeviceLocale = () => {
  try {
    return typeof localStorage !== 'undefined' ? localStorage.getItem('paperlms_locale') : null;
  } catch {
    return null;
  }
};

const applyAccountDefaultLocale = (data) => {
  if (!data) return;
  const locale = pickLocale({
    userLocale: data.locale,
    deviceLocale: readDeviceLocale(),
    accountLocale: data?.account?.default_locale || data?.account_default_locale,
  });
  if (!locale) return;
  if (data.locale && locale === pickLocale({ userLocale: data.locale })) {
    // Keep this device in sync with the profile choice.
    try { localStorage.setItem('paperlms_locale', locale); } catch { /* ignore */ }
  }
  if (locale !== i18n.language) {
    i18n.changeLanguage(locale);
  }
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // On mount, verify session by calling /users/self
    // The httpOnly cookie will be sent automatically
    api.getSelf()
      .then((data) => {
        setUser(data);
        applyAccountDefaultLocale(data);
      })
      .catch(() => {
        // Not authenticated or token expired — clear any stale localStorage
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        setUser(null);
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  // Listen for 401 responses from the API client to auto-logout
  useEffect(() => {
    const handleSessionExpired = () => {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      setUser(null);
    };
    window.addEventListener('auth:session-expired', handleSessionExpired);
    return () => window.removeEventListener('auth:session-expired', handleSessionExpired);
  }, []);

  const login = async (email, password) => {
    const data = await api.login(email, password);
    // Wave 1.6 follow-up: SIS / OneRoster-provisioned learners
    // carry RequiresPasswordReset=true. The backend returns
    // {pending_token, must_reset_password: true} instead of a
    // session; stash the token in sessionStorage (same convention
    // as MFA pending — clears on tab close) and let the caller
    // route to /auth/password-set.
    if (data.must_reset_password && data.pending_token) {
      sessionStorage.setItem('password_reset_pending_token', data.pending_token);
      return data;
    }
    // Phase 9-B: when MFA gates this login, the backend returns
    // {pending_token, mfa_required: true} instead of {token, user}.
    // Stash the pending token in sessionStorage (NOT localStorage —
    // clears on tab close) and let the caller route to /mfa/verify.
    if (data.mfa_required && data.pending_token) {
      sessionStorage.setItem('mfa_pending_token', data.pending_token);
      return data;
    }
    // Phase 9-B "must enroll" flag: real session issued, but tenant
    // policy requires the user to enroll before continuing. Caller
    // routes to /mfa/enroll based on this flag.
    setUser(data.user);
    applyAccountDefaultLocale(data.user);
    return data;
  };

  // Wave 1.6 follow-up — finalize the password-set step.
  // PasswordResetRequiredPage calls this with the pending token from
  // sessionStorage and the user's chosen new password. On success
  // the backend has set the paper_session cookie + returned the
  // user payload; mirror the login() success path so navigation can
  // proceed.
  const finalizePasswordReset = async (newPassword) => {
    const pendingToken = sessionStorage.getItem('password_reset_pending_token');
    if (!pendingToken) {
      throw new Error('No pending password-reset found. Please log in again.');
    }
    const data = await api.auth.setPassword({ pendingToken, newPassword });
    sessionStorage.removeItem('password_reset_pending_token');
    if (data?.user) {
      setUser(data.user);
      applyAccountDefaultLocale(data.user);
    }
    return data;
  };

  const register = async (name, email, password) => {
    const data = await api.register(name, email, password);
    // Auth is handled by httpOnly cookie set by the server — no localStorage needed
    setUser(data.user);
    return data;
  };

  const logout = async () => {
    try {
      await api.logout();
    } catch (_) {
      // Ignore errors on logout
    }
    // Clear any legacy localStorage data
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setUser(null);
  };

  // refreshUser re-fetches /users/self and updates the user context.
  // Used after masquerade start/end to reflect the new identity.
  const refreshUser = async () => {
    try {
      const data = await api.getSelf();
      setUser(data);
      applyAccountDefaultLocale(data);
      return data;
    } catch {
      setUser(null);
      return null;
    }
  };

  return (
    <AuthContext.Provider value={{ user, setUser, login, register, logout, loading, refreshUser, finalizePasswordReset }}>
      {children}
    </AuthContext.Provider>
  );
};

// useOptionalAuth returns the auth context, or null outside an AuthProvider
// (for components that also render standalone, e.g. in isolated tests).
export const useOptionalAuth = () => useContext(AuthContext);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
