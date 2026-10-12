import React, { useEffect, useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

// OAuth2 consent hand-off across sign-in (B076).
//
// GET /api/v1/login/oauth2/auth sends the browser to the SPA consent
// screen. A user who is not signed in yet would otherwise be bounced to
// /login and, after signing in by any method (password, SSO, passkey,
// MFA), land on the dashboard with the application's request lost. The
// gate remembers the consent URL for this tab; the dashboard route sends
// the user back to it once after sign-in.

const PENDING_KEY = 'paper_oauth_consent_pending';
const PENDING_TTL_MS = 10 * 60 * 1000; // an authorization code's lifetime
const CONSENT_PATH = '/login/oauth2/confirm';

export function rememberPendingConsent(pathWithSearch) {
  try {
    sessionStorage.setItem(PENDING_KEY, JSON.stringify({ path: pathWithSearch, at: Date.now() }));
  } catch {
    // Storage unavailable: the user can restart the authorization.
  }
}

export function peekPendingConsent() {
  try {
    const raw = sessionStorage.getItem(PENDING_KEY);
    if (!raw) return null;
    const { path, at } = JSON.parse(raw);
    // Only ever resume to the consent screen itself (never an arbitrary
    // path), and only while the request is still fresh.
    if (typeof path !== 'string' || !path.startsWith(`${CONSENT_PATH}?`)) return null;
    if (typeof at !== 'number' || Date.now() - at > PENDING_TTL_MS) return null;
    return path;
  } catch {
    return null;
  }
}

export function clearPendingConsent() {
  try {
    sessionStorage.removeItem(PENDING_KEY);
  } catch {
    // ignore
  }
}

// Route guard for the consent screen: like ProtectedRoute, but remembers
// the request before sending a signed-out user to /login.
export const OAuthConsentGate = ({ children }) => {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center">
        <div className="text-text-secondary">Loading...</div>
      </div>
    );
  }
  if (!user) {
    rememberPendingConsent(`${location.pathname}${location.search}`);
    return <Navigate to="/login" replace />;
  }
  return children;
};

// Wraps the post-sign-in landing page: if an OAuth consent was pending in
// this tab, go back to it (once) instead of rendering the page.
export const ResumePendingOAuthConsent = ({ children }) => {
  const [target] = useState(() => peekPendingConsent());

  useEffect(() => {
    clearPendingConsent();
  }, []);

  if (target) {
    return <Navigate to={target} replace />;
  }
  return children;
};
