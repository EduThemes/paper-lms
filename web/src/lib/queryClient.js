// Process-wide React Query client. One instance per browser tab — the
// 401 listener in api.js handles session-expired re-auth, so we don't
// need any retry-on-auth-error magic here.
//
// Defaults:
//   - `staleTime: 30s` — keeps the cache "fresh" for half a minute, so
//     tab switches and re-mounts inside the same flow don't refetch.
//     Mutations explicitly invalidate on success to push fresh data.
//   - `retry: 1` — one retry on transient network errors. The `ApiError`
//     thrown by api.js wraps any non-2xx status, so a 4xx still retries
//     once which is harmless (server returns the same 4xx) but avoids
//     a flicker on flaky links. If a route is provably expensive (an
//     export, a long report), the consumer can override per-query.

import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});
