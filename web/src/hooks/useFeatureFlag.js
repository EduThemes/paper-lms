import { useEffect } from 'react';
import { useFeatureFlagContext } from '../contexts/FeatureFlagContext';

/**
 * useFeatureFlag — boolean check for a feature in any context.
 *
 *   const k2 = useFeatureFlag('k2_mode', 'Course', courseId);
 *   const tiptap = useFeatureFlag('tiptap_rce'); // user-level
 *
 * Returns `true` only when the resolved state is `on`. Returns `false`
 * during initial load (rather than `null`) so callers can use it directly
 * in JSX without nullish guards. If you need loading awareness, use
 * `useFeatureFlagContext()` directly.
 */
export default function useFeatureFlag(featureName, contextType, contextId) {
  const { userFlags, contextFlags, refreshContext } = useFeatureFlagContext();

  // Lazily fetch context-scoped flags the first time they're requested.
  useEffect(() => {
    if (!contextType || !contextId) return;
    const key = `${contextType}:${contextId}`;
    if (!contextFlags[key]) {
      refreshContext(contextType, contextId);
    }
  }, [contextType, contextId, contextFlags, refreshContext]);

  if (contextType && contextId) {
    const key = `${contextType}:${contextId}`;
    const flag = contextFlags[key]?.[featureName];
    return flag?.state === 'on';
  }
  return userFlags[featureName]?.state === 'on';
}
