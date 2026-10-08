import { useEffect, useState } from 'react';
import { api } from '../services/api';

// One lookup per user per page load: Layout re-mounts on every route, and
// the observer link should not refetch each time.
const cache = new Map();

export function hasObserveesFromResponse(data) {
  const list = Array.isArray(data) ? data : data?.data || [];
  return list.length > 0;
}

export function lookup(userId) {
  if (!cache.has(userId)) {
    cache.set(
      userId,
      Promise.resolve()
        .then(() => api.getObservees(userId))
        .then(hasObserveesFromResponse)
        .catch(() => false),
    );
  }
  return cache.get(userId);
}

/**
 * Reports whether the signed-in user is linked to any students as an
 * observer (parent). B099: '/observer' was only in the admin nav, so a
 * parent had no way to reach their dashboard.
 */
export default function useHasObservees(user) {
  const userId = user?.id;
  const [hasObservees, setHasObservees] = useState(false);

  useEffect(() => {
    if (!userId) {
      setHasObservees(false);
      return undefined;
    }
    let live = true;
    lookup(userId).then((v) => {
      if (live) setHasObservees(v);
    });
    return () => {
      live = false;
    };
  }, [userId]);

  return hasObservees;
}

// Test hook: drop cached lookups.
export function resetObserveeCache() {
  cache.clear();
}
