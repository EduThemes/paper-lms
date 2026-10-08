// Modules-lane API calls and react-query bindings: module progression
// (G061/G062), External Tool item launches (G064) and course pacing
// blackout dates / timeline (G079). Kept out of api.js so lanes merge
// cleanly; uses the shared request helper.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { request } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

export const moduleKeys = {
  progress: (courseId, userId) => ['courses', String(courseId), 'module_progress', userId ? String(userId) : 'self'],
  progressions: (courseId) => ['courses', String(courseId), 'module_progressions'],
  blackoutDates: (courseId) => ['courses', String(courseId), 'blackout_dates'],
};

// --- plain calls ----------------------------------------------------------

export async function getModuleProgress(courseId, userId) {
  const q = userId ? `?user_id=${encodeURIComponent(userId)}` : '';
  const { data } = await request(`/courses/${courseId}/module_progress${q}`);
  return data;
}

export async function getModuleProgressions(courseId) {
  const { data } = await request(`/courses/${courseId}/module_progressions`);
  return data;
}

export async function markModuleItemRead(courseId, moduleId, itemId) {
  await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}/mark_read`, { method: 'POST' });
}

export async function setModuleItemDone(courseId, moduleId, itemId, done) {
  await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}/done`, {
    method: done ? 'PUT' : 'DELETE',
  });
}

export async function getModuleItemLaunch(courseId, moduleId, itemId) {
  const { data } = await request(`/courses/${courseId}/modules/${moduleId}/items/${itemId}/launch`);
  return data;
}

export async function getCoursePaceTimeline(courseId, paceId) {
  const { data } = await request(`/courses/${courseId}/course_pacing/${paceId}/timeline`);
  return data || [];
}

export async function getBlackoutDates(courseId) {
  const { data } = await request(`/courses/${courseId}/blackout_dates`);
  return data || [];
}

export async function createBlackoutDate(courseId, blackoutDate) {
  const { data } = await request(`/courses/${courseId}/blackout_dates`, {
    method: 'POST',
    body: JSON.stringify({ blackout_date: blackoutDate }),
  });
  return data;
}

export async function deleteBlackoutDate(courseId, id) {
  await request(`/courses/${courseId}/blackout_dates/${id}`, { method: 'DELETE' });
}

// buildToolLaunchForm returns a detached <form> that starts an LTI 1.3
// launch: it posts the login-initiation parameters to the platform's
// /lti/oidc/login, which redirects to the tool. The session cookie
// authenticates the post.
export function buildToolLaunchForm(launch, { newTab = false, doc = document } = {}) {
  const form = doc.createElement('form');
  form.method = 'POST';
  form.action = `${API_URL}/lti/oidc/login`;
  form.target = newTab ? '_blank' : '_self';
  form.style.display = 'none';
  const fields = {
    client_id: launch.client_id,
    target_link_uri: launch.target_link_uri,
    lti_message_hint: launch.lti_message_hint,
  };
  Object.entries(fields).forEach(([name, value]) => {
    if (value === undefined || value === null || value === '') return;
    const input = doc.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = String(value);
    form.appendChild(input);
  });
  return form;
}

// Launching an External Tool item: the module launch endpoint checks the
// student's progression (a locked item can't be launched) and records the
// view, then the course tool launch page (LTI lane, G131) launches through
// the item's resource link. The legacy login-form post remains for a
// server without launch_path.
export async function launchModuleTool(courseId, moduleId, item, { win = window } = {}) {
  const launch = await getModuleItemLaunch(courseId, moduleId, item.id);
  if (launch?.launch_path) {
    if (item.new_tab) win.open(launch.launch_path, '_blank', 'noopener');
    else win.location.assign(launch.launch_path);
    return;
  }
  const form = buildToolLaunchForm(launch, { newTab: !!item.new_tab });
  document.body.appendChild(form);
  form.submit();
  form.remove();
}

// --- react-query hooks ----------------------------------------------------

export function useModuleProgress(courseId, { userId, enabled = true } = {}) {
  return useQuery({
    queryKey: moduleKeys.progress(courseId, userId),
    queryFn: () => getModuleProgress(courseId, userId),
    enabled: !!courseId && enabled,
  });
}

export function useModuleProgressions(courseId) {
  return useQuery({
    queryKey: moduleKeys.progressions(courseId),
    queryFn: () => getModuleProgressions(courseId),
    enabled: !!courseId,
  });
}

export function useMarkModuleItemDone(courseId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ moduleId, itemId, done }) => setModuleItemDone(courseId, moduleId, itemId, done),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['courses', String(courseId), 'module_progress'] }),
  });
}

export function useMarkModuleItemRead(courseId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ moduleId, itemId }) => markModuleItemRead(courseId, moduleId, itemId),
    onSettled: () => qc.invalidateQueries({ queryKey: ['courses', String(courseId), 'module_progress'] }),
  });
}

export function useBlackoutDates(courseId) {
  return useQuery({
    queryKey: moduleKeys.blackoutDates(courseId),
    queryFn: () => getBlackoutDates(courseId),
    enabled: !!courseId,
  });
}

export function useCreateBlackoutDate(courseId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b) => createBlackoutDate(courseId, b),
    onSuccess: () => qc.invalidateQueries({ queryKey: moduleKeys.blackoutDates(courseId) }),
  });
}

export function useDeleteBlackoutDate(courseId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id) => deleteBlackoutDate(courseId, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: moduleKeys.blackoutDates(courseId) }),
  });
}
