import React, { createContext, useState, useContext, useEffect } from 'react';
import { api } from '../services/api';

const AuthContext = createContext(null);

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // On mount, verify session by calling /users/self
    // The httpOnly cookie will be sent automatically
    api.getSelf()
      .then((data) => {
        setUser(data);
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
      return data;
    } catch {
      setUser(null);
      return null;
    }
  };

  return (
    <AuthContext.Provider value={{ user, setUser, login, register, logout, loading, refreshUser }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
