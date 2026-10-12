import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { rememberReturnTo, consumeReturnTo, peekReturnTo } from '../utils/returnTo';

const ProtectedRoute = ({ children }) => {
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
    // G160: remember the deep link so sign-in (password, MFA, passkey or
    // an SSO round trip) lands back here instead of on the dashboard.
    // (sessionStorage, not router state: state survives neither the SSO
    // round trip nor a reload, and a fresh state object per render would
    // re-trigger the redirect.)
    rememberReturnTo(location);
    return <Navigate to="/login" replace />;
  }

  // Every sign-in flow ends on "/"; finish the trip to the remembered page.
  if (location.pathname === '/' && peekReturnTo()) {
    const target = consumeReturnTo();
    if (target) return <Navigate to={target} replace />;
  }

  return children;
};

export default ProtectedRoute;
