import React, { createContext, useContext, useEffect, useState, useCallback, useMemo } from 'react';
import { api } from '../services/api';

/**
 * FeatureFlagContext caches the per-user effective flag list once on mount.
 * Course- and account-level flags are still fetched on-demand by the
 * FeatureFlagsPage / useFeatureFlag(featureName, contextType, contextId).
 *
 * This keeps the common case — `useFeatureFlag("k2_mode")` from any page —
 * a synchronous lookup against an in-memory map.
 */
const FeatureFlagContext = createContext({
  userFlags: {},
  contextFlags: {},
  loading: true,
  refresh: () => {},
  refreshContext: async () => {},
});

export const FeatureFlagProvider = ({ children }) => {
  const [userFlags, setUserFlags] = useState({});
  const [contextFlags, setContextFlags] = useState({}); // keyed by `${type}:${id}`
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      if (api.listUserFeatureFlags) {
        const result = await api.listUserFeatureFlags();
        const flags = result.data || result || [];
        const map = {};
        for (const f of flags) map[f.feature] = f;
        setUserFlags(map);
      }
    } catch {
      // Silent fail — flags default to off when unreachable.
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshContext = useCallback(async (contextType, contextId) => {
    const key = `${contextType}:${contextId}`;
    try {
      let result;
      if (contextType === 'Course' && api.listCourseFeatureFlags) {
        result = await api.listCourseFeatureFlags(contextId);
      } else if (contextType === 'Account' && api.listAccountFeatureFlags) {
        result = await api.listAccountFeatureFlags(contextId);
      } else {
        return;
      }
      const flags = result.data || result || [];
      const map = {};
      for (const f of flags) map[f.feature] = f;
      setContextFlags(prev => ({ ...prev, [key]: map }));
    } catch {
      // Ignore.
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const value = useMemo(
    () => ({ userFlags, contextFlags, loading, refresh, refreshContext }),
    [userFlags, contextFlags, loading, refresh, refreshContext]
  );

  return (
    <FeatureFlagContext.Provider value={value}>
      {children}
    </FeatureFlagContext.Provider>
  );
};

export const useFeatureFlagContext = () => useContext(FeatureFlagContext);

export default FeatureFlagContext;
