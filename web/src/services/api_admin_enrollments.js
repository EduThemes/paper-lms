// API calls for the admin-enrollments lane: enrollment lifecycle (edit,
// conclude, deactivate, delete, re-activate), course invitations, section
// management and cross-listing, and course date-access settings.
//
// Uses the shared request helper on `api` (lazily, so a test that mocks
// `api` without it only fails the call, not the import).

import { api } from './api';

const req = (path, options) => api.request(path, options);
const json = (body) => JSON.stringify(body);

// Walks every page of a list endpoint (the server caps per_page at 100).
async function allPages(path) {
  const sep = path.includes('?') ? '&' : '?';
  const rows = [];
  for (let page = 1; page <= 200; page += 1) {
    const { data, pagination } = await req(`${path}${sep}page=${page}&per_page=100`);
    const batch = Array.isArray(data) ? data : [];
    rows.push(...batch);
    if (batch.length === 0) break;
    if (pagination?.next) continue;
    if (batch.length < 100) break;
  }
  return rows;
}

export function rosterPath(courseId, states = []) {
  const qs = states.map((s) => `state[]=${encodeURIComponent(s)}`).join('&');
  return `/courses/${courseId}/enrollments${qs ? `?${qs}` : ''}`;
}

export const enrollmentsAdminApi = {
  // The whole roster, optionally filtered by state (staff only see
  // inactive / concluded / rejected people).
  listRoster: (courseId, states = []) => allPages(rosterPath(courseId, states)),

  createEnrollment: async (courseId, enrollment) => {
    const { data } = await req(`/courses/${courseId}/enrollments`, { method: 'POST', body: json({ enrollment }) });
    return data;
  },

  updateEnrollment: async (courseId, enrollmentId, patch) => {
    const { data } = await req(`/courses/${courseId}/enrollments/${enrollmentId}`, {
      method: 'PUT',
      body: json({ enrollment: patch }),
    });
    return data;
  },

  // task: conclude | deactivate | delete
  applyTask: async (courseId, enrollmentId, task) => {
    const { data } = await req(`/courses/${courseId}/enrollments/${enrollmentId}?task=${encodeURIComponent(task)}`, {
      method: 'DELETE',
    });
    return data;
  },

  reactivate: async (courseId, enrollmentId) => {
    const { data } = await req(`/courses/${courseId}/enrollments/${enrollmentId}/reactivate`, { method: 'PUT' });
    return data;
  },

  listInvitations: async () => {
    const { data } = await req('/users/self/course_invitations');
    return Array.isArray(data) ? data : [];
  },

  acceptInvitation: async (courseId, enrollmentId) => {
    const { data } = await req(`/courses/${courseId}/enrollments/${enrollmentId}/accept`, { method: 'POST' });
    return data;
  },

  rejectInvitation: async (courseId, enrollmentId) => {
    const { data } = await req(`/courses/${courseId}/enrollments/${enrollmentId}/reject`, { method: 'POST' });
    return data;
  },

  // Sections
  listSections: (courseId) => allPages(`/courses/${courseId}/sections`),

  createSection: async (courseId, section) => {
    const { data } = await req(`/courses/${courseId}/sections`, { method: 'POST', body: json({ course_section: section }) });
    return data;
  },

  updateSection: async (sectionId, patch) => {
    const { data } = await req(`/sections/${sectionId}`, { method: 'PUT', body: json({ course_section: patch }) });
    return data;
  },

  deleteSection: async (sectionId) => {
    const { data } = await req(`/sections/${sectionId}`, { method: 'DELETE' });
    return data;
  },

  crosslistSection: async (sectionId, newCourseId) => {
    const { data } = await req(`/sections/${sectionId}/crosslist/${newCourseId}`, { method: 'POST' });
    return data;
  },

  uncrosslistSection: async (sectionId) => {
    const { data } = await req(`/sections/${sectionId}/crosslist`, { method: 'DELETE' });
    return data;
  },

  // Courses the caller can see, for the cross-list target picker.
  listMyCourses: () => allPages('/courses'),

  // G106 course settings: restrict_enrollments_to_course_dates,
  // restrict_student_future_view, restrict_student_past_view.
  updateCourseDateAccess: async (courseId, settings) => {
    const { data } = await req(`/courses/${courseId}`, { method: 'PUT', body: json({ course: settings }) });
    return data;
  },
};
