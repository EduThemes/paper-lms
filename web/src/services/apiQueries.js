// React Query bindings for `api.js`.
//
// Three layers:
//
//   1. `queryKeys` — a single source of truth for cache keys, domain-scoped
//      (e.g. `queryKeys.courses.detail(id)`). Use these everywhere — never
//      hand-type a key in a component. `queryClient.invalidateQueries({
//      queryKey: queryKeys.courses.all })` invalidates every variant.
//
//   2. `useXxx` hooks — `useQuery` wrappers that bind the key to the
//      matching `api.js` method. The returned `query.error` is an
//      `ApiError` instance (see api.js), so callers can branch on
//      `error.status` if they need to.
//
//      ```jsx
//      const query = useCourse(courseId);
//      return <Page query={query} title="Course">{(course) => ...}</Page>;
//      ```
//
//   3. `useXxxMutation` hooks — `useMutation` wrappers that handle the
//      typical "POST, then invalidate the list" pattern. Callers can
//      override `onSuccess` to redirect, reset form state, etc.
//
// The 99-page bulk migration will fill this out further; for now it
// only covers the three reference pages plus the obvious neighbors so
// follow-up PRs have an easy starting point.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from './api';

// ---------------------------------------------------------------------------
// queryKeys — single source of truth. Keys are arrays so partial matches
// work for invalidation (invalidating `['courses']` blows away every
// courses-* entry).
// ---------------------------------------------------------------------------

export const queryKeys = {
  courses: {
    all: ['courses'],
    list: (params = {}) => ['courses', 'list', params],
    listAll: (params = {}) => ['courses', 'listAll', params],
    detail: (id) => ['courses', 'detail', String(id)],
    modules: (courseId, params = {}) => ['courses', String(courseId), 'modules', params],
  },
  users: {
    all: ['users'],
    list: (params = {}) => ['users', 'list', params],
    search: (term, params = {}) => ['users', 'search', term, params],
    detail: (id) => ['users', 'detail', String(id)],
  },
  accounts: {
    all: ['accounts'],
    detail: (id) => ['accounts', 'detail', String(id)],
  },
  superAdminSettings: {
    all: ['superAdminSettings'],
    setting: (key, accountId) =>
      ['superAdminSettings', 'setting', String(key), accountId ? String(accountId) : 'instance'],
  },
};

// ---------------------------------------------------------------------------
// Query hooks
// ---------------------------------------------------------------------------

export function useCoursesAll(page = 1, perPage = 100, options = {}) {
  return useQuery({
    queryKey: queryKeys.courses.listAll({ page, perPage }),
    queryFn: () => api.getAllCourses(page, perPage),
    ...options,
  });
}

export function useCourse(courseId, options = {}) {
  return useQuery({
    queryKey: queryKeys.courses.detail(courseId),
    queryFn: () => api.getCourse(courseId),
    enabled: !!courseId,
    ...options,
  });
}

export function useCourseModules(courseId, page = 1, perPage = 100, includeItems = true, options = {}) {
  return useQuery({
    queryKey: queryKeys.courses.modules(courseId, { page, perPage, includeItems }),
    queryFn: () => api.getModules(courseId, page, perPage, includeItems),
    enabled: !!courseId,
    ...options,
  });
}

export function useAccount(accountId = 1, options = {}) {
  return useQuery({
    queryKey: queryKeys.accounts.detail(accountId),
    queryFn: () => api.getAccount(accountId),
    ...options,
  });
}

export function useUsersList(search = '', page = 1, perPage = 100, options = {}) {
  // Single hook handles both "list everyone" and "filter by search" —
  // the cache key changes when `search` changes, so react-query keeps
  // both result sets in parallel.
  const term = (search || '').trim();
  return useQuery({
    queryKey: term
      ? queryKeys.users.search(term, { page, perPage })
      : queryKeys.users.list({ page, perPage }),
    queryFn: () => (term ? api.searchUsers(term, page, perPage) : api.listUsers(page, perPage)),
    ...options,
  });
}

// ---------------------------------------------------------------------------
// Mutation hooks
// ---------------------------------------------------------------------------

export function useCreateCourse(options = {}) {
  const queryClient = useQueryClient();
  const { onSuccess: callerOnSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (course) => api.createCourse(course),
    ...rest,
    onSuccess: (...args) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.courses.all });
      callerOnSuccess?.(...args);
    },
  });
}

export function useUpdateUserRole(options = {}) {
  const queryClient = useQueryClient();
  const { onSuccess: callerOnSuccess, ...rest } = options;
  return useMutation({
    mutationFn: ({ userId, role }) => api.updateUserRole(userId, role),
    ...rest,
    onSuccess: (...args) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
      callerOnSuccess?.(...args);
    },
  });
}

// useEffectiveSetting — wraps `superAdminSettings.getSetting(key, accountId)`.
//
// A 403 from this endpoint is the "non-super-admin viewing the page"
// case, NOT a system failure. The Settings Engine catalog default is
// in effect; raising it is a super-admin operation. We translate the
// 403 into a synthetic `EffectiveValue` with `source:'default'` so
// the page renders normally without an error banner.
//
// Any other error (network, 500, etc.) propagates through react-query
// the usual way — callers wrapped in <Page> will see the error state.
export function useEffectiveSetting(key, accountId, options = {}) {
  return useQuery({
    queryKey: queryKeys.superAdminSettings.setting(key, accountId),
    queryFn: async () => {
      try {
        return await api.superAdminSettings.getSetting(key, accountId);
      } catch (err) {
        if (err?.status === 403 || /forbidden/i.test(err?.message || '')) {
          return { value: null, source: 'default', is_secret: false, forbidden: true };
        }
        throw err;
      }
    },
    enabled: !!key,
    ...options,
  });
}

// useUpdateSetting — wraps `superAdminSettings.setSetting(key, body)`.
// On success, seeds the cache with the response and invalidates the
// matching setting query so the form reads the refreshed effective
// value without an extra fetch.
export function useUpdateSetting(key, options = {}) {
  const queryClient = useQueryClient();
  const { onSuccess: callerOnSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (body) => api.superAdminSettings.setSetting(key, body),
    ...rest,
    onSuccess: (data, vars, ctx) => {
      // The PUT response shape is the same EffectiveValue as the GET,
      // so we can seed the cache directly. `scope_id` keys the cache
      // entry for account-scoped writes; instance-scope (scope_id=0)
      // shares the same key as the unscoped GET.
      const accountId = vars?.scope === 'account' ? vars.scope_id : undefined;
      queryClient.setQueryData(queryKeys.superAdminSettings.setting(key, accountId), data);
      queryClient.invalidateQueries({ queryKey: queryKeys.superAdminSettings.all });
      callerOnSuccess?.(data, vars, ctx);
    },
  });
}

export function useUpdateAccount(accountId = 1, options = {}) {
  const queryClient = useQueryClient();
  const { onSuccess: callerOnSuccess, ...rest } = options;
  return useMutation({
    mutationFn: (payload) => api.updateAccount(accountId, payload),
    ...rest,
    onSuccess: (data, ...args) => {
      // Optimistic-ish: seed the cache with the server's response so
      // the next render uses the updated row without an extra fetch.
      queryClient.setQueryData(queryKeys.accounts.detail(accountId), data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accounts.all });
      callerOnSuccess?.(data, ...args);
    },
  });
}
