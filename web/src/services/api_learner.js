// API calls for the learner experience (learner-dashboard lane): planner /
// to-do list, dashboard course cards and customization, activity stream,
// missing submissions, the student grade summary, observer alerts and the
// user profile. Kept out of api.js so parallel work does not collide there;
// it reuses the shared request helper (CSRF, 401 handling, ApiError).

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, getCSRFToken, ApiError } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const get = async (path) => (await api.request(path)).data;
const send = async (method, path, body) =>
  (await api.request(path, { method, body: body === undefined ? undefined : JSON.stringify(body) })).data;

const qs = (params) => {
  const sp = new URLSearchParams();
  Object.entries(params || {}).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') sp.set(k, String(v));
  });
  const s = sp.toString();
  return s ? `?${s}` : '';
};

export const learnerApi = {
  // Planner
  plannerItems: ({ startDate, endDate, filter } = {}) =>
    get(`/planner/items${qs({ start_date: startDate, end_date: endDate, filter })}`),
  createPlannerNote: (note) => send('POST', '/planner_notes', note),
  updatePlannerNote: (id, note) => send('PUT', `/planner_notes/${id}`, note),
  deletePlannerNote: (id) => send('DELETE', `/planner_notes/${id}`),
  setPlannerOverride: ({ plannableType, plannableId, markedComplete, dismissed }) =>
    send('POST', '/planner/overrides', {
      plannable_type: plannableType,
      plannable_id: plannableId,
      marked_complete: !!markedComplete,
      dismissed: !!dismissed,
    }),

  // Dashboard
  dashboardCards: ({ all } = {}) => get(`/dashboard/dashboard_cards${qs({ all: all ? 'true' : '' })}`),
  setFavorite: (courseId, favorite) =>
    send(favorite ? 'POST' : 'DELETE', `/users/self/favorites/courses/${courseId}`),
  setNickname: (courseId, nickname) =>
    nickname
      ? send('PUT', `/users/self/course_nicknames/${courseId}`, { nickname })
      : send('DELETE', `/users/self/course_nicknames/${courseId}`),
  setColor: (courseId, hexcode) => send('PUT', `/users/self/colors/course_${courseId}`, { hexcode: hexcode || '' }),
  setPositions: (order) => send('PUT', '/dashboard/positions', { order }),
  activityStream: ({ days, perPage } = {}) =>
    get(`/users/self/activity_stream${qs({ days, per_page: perPage })}`),
  missingSubmissions: (userId = 'self') => get(`/users/${userId}/missing_submissions`),

  // Grades
  gradeSummary: (courseId, { studentId, gradingPeriodId } = {}) =>
    get(`/courses/${courseId}/grade_summary${qs({ student_id: studentId, grading_period_id: gradingPeriodId })}`),

  // Observer alerts
  alertThresholds: (studentId) => get(`/observer_alert_thresholds${qs({ student_id: studentId })}`),
  createAlertThreshold: ({ studentId, alertType, threshold }) =>
    send('POST', '/observer_alert_thresholds', {
      threshold: { user_id: studentId, alert_type: alertType, threshold: threshold ?? null },
    }),
  updateAlertThreshold: (id, threshold) =>
    send('PUT', `/observer_alert_thresholds/${id}`, { threshold: { threshold } }),
  deleteAlertThreshold: (id) => send('DELETE', `/observer_alert_thresholds/${id}`),
  observerAlerts: (studentId) => get(`/users/self/observer_alerts${qs({ student_id: studentId })}`),
  observerAlertsUnread: () => get('/users/self/observer_alerts/unread_count'),
  setAlertState: (id, state) => send('PUT', `/observer_alerts/${id}/${state}`),

  // Profile
  profileCard: (userId = 'self') => get(`/users/${userId}/profile_card`),
  updateProfile: (profile) => send('PUT', '/users/self/profile', profile),
  updateUser: (userId, user) => send('PUT', `/users/${userId}`, { user }),
  uploadAvatar: async (file) => {
    const fd = new FormData();
    fd.append('file', file);
    const res = await fetch(`${API_URL}/users/self/avatar`, {
      method: 'POST',
      body: fd,
      credentials: 'include',
      headers: { 'X-CSRF-Token': getCSRFToken() },
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (res.status === 401) window.dispatchEvent(new Event('auth:session-expired'));
      throw new ApiError({ status: res.status, body });
    }
    return body;
  },
  deleteAvatar: () => send('DELETE', '/users/self/avatar'),
};

// ---------------------------------------------------------------------------
// React Query bindings
// ---------------------------------------------------------------------------

export const learnerKeys = {
  all: ['learner'],
  planner: (params) => ['learner', 'planner', params],
  cards: (all) => ['learner', 'cards', !!all],
  activity: (params) => ['learner', 'activity', params || {}],
  missing: (userId) => ['learner', 'missing', String(userId)],
  gradeSummary: (courseId, params) => ['learner', 'grades', String(courseId), params || {}],
  thresholds: (studentId) => ['learner', 'thresholds', String(studentId ?? 'all')],
  alerts: (studentId) => ['learner', 'alerts', String(studentId ?? 'all')],
  alertsUnread: ['learner', 'alerts', 'unread'],
  profile: (userId) => ['learner', 'profile', String(userId)],
};

export function usePlannerItems(params, options = {}) {
  return useQuery({ queryKey: learnerKeys.planner(params), queryFn: () => learnerApi.plannerItems(params), ...options });
}

export function useDashboardCards(all = false, options = {}) {
  return useQuery({ queryKey: learnerKeys.cards(all), queryFn: () => learnerApi.dashboardCards({ all }), ...options });
}

export function useActivityStream(params, options = {}) {
  return useQuery({ queryKey: learnerKeys.activity(params), queryFn: () => learnerApi.activityStream(params), ...options });
}

export function useMissingSubmissions(userId = 'self', options = {}) {
  return useQuery({ queryKey: learnerKeys.missing(userId), queryFn: () => learnerApi.missingSubmissions(userId), ...options });
}

export function useGradeSummary(courseId, params, options = {}) {
  return useQuery({
    queryKey: learnerKeys.gradeSummary(courseId, params),
    queryFn: () => learnerApi.gradeSummary(courseId, params),
    enabled: !!courseId,
    ...options,
  });
}

export function useAlertThresholds(studentId, options = {}) {
  return useQuery({
    queryKey: learnerKeys.thresholds(studentId),
    queryFn: () => learnerApi.alertThresholds(studentId),
    enabled: !!studentId,
    ...options,
  });
}

export function useObserverAlerts(studentId, options = {}) {
  return useQuery({ queryKey: learnerKeys.alerts(studentId), queryFn: () => learnerApi.observerAlerts(studentId), ...options });
}

export function useProfileCard(userId = 'self', options = {}) {
  return useQuery({ queryKey: learnerKeys.profile(userId), queryFn: () => learnerApi.profileCard(userId), ...options });
}

// useLearnerMutation wraps a learnerApi call and invalidates the learner
// cache (or the given keys) when it succeeds.
export function useLearnerMutation(fn, { invalidate = [learnerKeys.all], ...options } = {}) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    ...options,
    onSuccess: (...args) => {
      invalidate.forEach((queryKey) => qc.invalidateQueries({ queryKey }));
      options.onSuccess?.(...args);
    },
  });
}
