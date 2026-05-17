import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

/**
 * SuperAdminGate hides routes that should only be reachable by the
 * platform operator (users.role === 'super_admin'). Sits INSIDE a
 * <ProtectedRoute> chain — that handles the "no session at all" path
 * with a redirect to /login; this gate handles "session, but wrong
 * role" with a clean 403-style state instead of a redirect loop.
 *
 * The browser-side check is defense-in-depth ONLY — the server's
 * RequireSuperAdmin middleware is the authoritative gate (every
 * /api/v1/superadmin/* endpoint re-checks the DB on every request).
 * A user who tampered with their local AuthContext to claim
 * role='super_admin' would still 403 at the server.
 *
 * Mirrors the role-literal-match contract from the server side: only
 * the exact string 'super_admin' passes. No case folding, no trim.
 * The case-sensitivity is locked in
 * internal/api/v1/handlers/super_admin_isolation_test.go to defend
 * against the Canvas-LMS site_admin CVE class.
 */
export default function SuperAdminGate({ children }) {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center">
        <div className="text-text-secondary">Loading…</div>
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  if (user.role !== 'super_admin') {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center px-4">
        <div className="max-w-md text-center">
          <h1 className="text-2xl font-semibold text-text-primary mb-2">
            Platform operator access required
          </h1>
          <p className="text-text-secondary">
            This area manages deployment-wide settings (SMTP, file storage,
            authentication, AI). Account administrators can manage their own
            tenant from{' '}
            <a className="underline" href="/account/settings">
              Account settings
            </a>
            .
          </p>
        </div>
      </div>
    );
  }

  return children;
}
