// React Query bindings for api_admin_accounts.js (account administration).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminAccountsApi } from './api_admin_accounts';

export const adminKeys = {
  all: ['adminAccounts'],
  user: (id) => ['adminAccounts', 'user', String(id)],
  userHistory: (id) => ['adminAccounts', 'user', String(id), 'history'],
  reports: ['adminAccounts', 'reports'],
  reportRuns: (report) => ['adminAccounts', 'reports', report],
  brand: ['adminAccounts', 'brand'],
  myBrand: ['brandConfig', 'self'],
  deletedCourses: (term) => ['adminAccounts', 'deletedCourses', term],
  auditLog: (page, filters) => ['adminAccounts', 'auditLog', page, filters],
  quotaDefaults: ['adminAccounts', 'quotaDefaults'],
  assignableRoles: (courseId) => ['adminAccounts', 'assignableRoles', String(courseId)],
  coursePermissions: (courseId) => ['coursePermissions', String(courseId)],
  sisImport: (id) => ['adminAccounts', 'sisImport', String(id)],
};

// Mutation helper: run fn, then invalidate the given keys.
function useInvalidatingMutation(mutationFn, keys, options = {}) {
  const queryClient = useQueryClient();
  const { onSuccess, ...rest } = options;
  return useMutation({
    mutationFn,
    ...rest,
    onSuccess: (...args) => {
      keys.forEach((queryKey) => queryClient.invalidateQueries({ queryKey }));
      onSuccess?.(...args);
    },
  });
}

// ---- Users -------------------------------------------------------------

export function useAdminUser(userId, options = {}) {
  return useQuery({
    queryKey: adminKeys.user(userId),
    queryFn: () => adminAccountsApi.getUserDetail(userId),
    enabled: !!userId,
    ...options,
  });
}

export function useAdminUserHistory(userId, options = {}) {
  return useQuery({
    queryKey: adminKeys.userHistory(userId),
    queryFn: () => adminAccountsApi.getUserHistory(userId),
    enabled: !!userId,
    ...options,
  });
}

export function useCreateAdminUser(options) {
  return useInvalidatingMutation((user) => adminAccountsApi.createUser(user), [['users']], options);
}

export function useUpdateAdminUser(userId, options) {
  return useInvalidatingMutation((user) => adminAccountsApi.updateUser(userId, user), [adminKeys.user(userId), ['users']], options);
}

export function useUserStatusMutation(userId, options) {
  return useInvalidatingMutation(
    ({ action }) => {
      if (action === 'suspend') return adminAccountsApi.setSuspended(userId, true);
      if (action === 'unsuspend') return adminAccountsApi.setSuspended(userId, false);
      return adminAccountsApi.forcePasswordReset(userId);
    },
    [adminKeys.user(userId), ['users']],
    options,
  );
}

export function useBulkSuspend(options) {
  return useInvalidatingMutation(({ userIds, suspended }) => adminAccountsApi.bulkSetSuspended(userIds, suspended), [['users']], options);
}

export function useLoginMutation(userId, options) {
  return useInvalidatingMutation(
    ({ action, loginId, login }) => {
      if (action === 'create') return adminAccountsApi.createLogin(userId, login);
      if (action === 'delete') return adminAccountsApi.deleteLogin(userId, loginId);
      return adminAccountsApi.updateLogin(userId, loginId, login);
    },
    [adminKeys.user(userId)],
    options,
  );
}

export function useObserverLinkMutation(userId, options) {
  return useInvalidatingMutation(
    ({ action, observerId, studentId }) => (action === 'unlink'
      ? adminAccountsApi.unlinkObservee(observerId, studentId)
      : adminAccountsApi.linkObservee(observerId, studentId)),
    [adminKeys.user(userId), adminKeys.all],
    options,
  );
}

// ---- Reports -----------------------------------------------------------

export function useAccountReports(options = {}) {
  return useQuery({ queryKey: adminKeys.reports, queryFn: () => adminAccountsApi.listReports(), ...options });
}

export function useReportRuns(report, options = {}) {
  return useQuery({
    queryKey: adminKeys.reportRuns(report),
    queryFn: () => adminAccountsApi.listReportRuns(report),
    enabled: !!report,
    ...options,
  });
}

export function useStartReport(options) {
  return useInvalidatingMutation(({ report, parameters }) => adminAccountsApi.startReport(report, parameters), [adminKeys.reports], options);
}

export function useDeleteReportRun(options) {
  return useInvalidatingMutation(({ report, id }) => adminAccountsApi.deleteReportRun(report, id), [adminKeys.reports], options);
}

// ---- Theme -------------------------------------------------------------

export function useBrandConfig(options = {}) {
  return useQuery({ queryKey: adminKeys.brand, queryFn: () => adminAccountsApi.getBrandConfig(), ...options });
}

export function useMyBrandConfig(options = {}) {
  return useQuery({
    queryKey: adminKeys.myBrand,
    queryFn: () => adminAccountsApi.getMyBrandConfig(),
    staleTime: 5 * 60 * 1000,
    retry: false,
    ...options,
  });
}

export function useBrandMutation(options) {
  return useInvalidatingMutation(
    ({ action, variables, file }) => {
      if (action === 'logo') return adminAccountsApi.uploadBrandLogo(file);
      if (action === 'deleteLogo') return adminAccountsApi.deleteBrandLogo();
      if (action === 'reset') return adminAccountsApi.resetBrandConfig();
      return adminAccountsApi.updateBrandConfig(variables);
    },
    [adminKeys.brand, adminKeys.myBrand],
    options,
  );
}

// ---- Admin tools ---------------------------------------------------------

export function useDeletedCourses(term, options = {}) {
  return useQuery({
    queryKey: adminKeys.deletedCourses(term),
    queryFn: () => adminAccountsApi.listDeletedCourses(term),
    ...options,
  });
}

export function useRestoreCourse(options) {
  return useInvalidatingMutation((courseId) => adminAccountsApi.restoreCourse(courseId), [['adminAccounts', 'deletedCourses'], ['courses']], options);
}

export function useAccountAuditLog(page, filters, options = {}) {
  return useQuery({
    queryKey: adminKeys.auditLog(page, filters),
    queryFn: () => adminAccountsApi.getAccountAuditLog(page, 25, filters),
    ...options,
  });
}

// ---- Quotas --------------------------------------------------------------

export function useQuotaDefaults(options = {}) {
  return useQuery({ queryKey: adminKeys.quotaDefaults, queryFn: () => adminAccountsApi.getQuotaDefaults(), ...options });
}

export function useUpdateQuotaDefaults(options) {
  return useInvalidatingMutation((defaults) => adminAccountsApi.updateQuotaDefaults(defaults), [adminKeys.quotaDefaults], options);
}

// ---- Roles / permissions -------------------------------------------------

export function useAssignableRoles(courseId, options = {}) {
  return useQuery({
    queryKey: adminKeys.assignableRoles(courseId),
    queryFn: () => adminAccountsApi.getAssignableRoles(courseId),
    enabled: !!courseId,
    retry: false,
    ...options,
  });
}

export function useCoursePermissions(courseId, options = {}) {
  return useQuery({
    queryKey: adminKeys.coursePermissions(courseId),
    queryFn: () => adminAccountsApi.getCoursePermissions(courseId),
    enabled: !!courseId,
    staleTime: 60 * 1000,
    retry: false,
    ...options,
  });
}
