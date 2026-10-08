// API + react-query bindings for the groups-conferences lane:
//   - G098 conferences: provider discovery, provider recordings.
//   - G100 group sets: unassigned roster, auto-assign, bulk creation,
//     CSV import / export, group leaders.
//
// Kept out of api.js so the lane's calls live in one module.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, requestRaw, uploadFile } from './api';

// ---------------------------------------------------------------------------
// Conferences
// ---------------------------------------------------------------------------

export async function getConferenceProviders(courseId) {
  const { data } = await api.request(`/courses/${courseId}/conferences/providers`);
  return data?.providers || [];
}

export async function getConferenceRecordings(conferenceId) {
  const { data } = await api.request(`/conferences/${conferenceId}/recordings`);
  return Array.isArray(data?.recordings) ? data.recordings : [];
}

export async function deleteConferenceRecording(conferenceId, recordingId) {
  const { data } = await api.request(
    `/conferences/${conferenceId}/recordings/${encodeURIComponent(recordingId)}`,
    { method: 'DELETE' },
  );
  return data;
}

// ---------------------------------------------------------------------------
// Group sets
// ---------------------------------------------------------------------------

export async function getGroupSetUsers(categoryId, { unassigned = false } = {}) {
  const q = unassigned ? '?unassigned=true' : '';
  const { data } = await api.request(`/group_categories/${categoryId}/users${q}`);
  return Array.isArray(data) ? data : [];
}

export async function assignUnassignedMembers(categoryId, { groupBySection = false } = {}) {
  const { data } = await api.request(`/group_categories/${categoryId}/assign_unassigned_members`, {
    method: 'POST',
    body: JSON.stringify({ group_by_section: groupBySection }),
  });
  return data;
}

export async function createGroupsInSet(categoryId, { count = 0, memberCount = 0, assign = false, groupBySection = false } = {}) {
  const { data } = await api.request(`/group_categories/${categoryId}/create_groups`, {
    method: 'POST',
    body: JSON.stringify({
      count,
      member_count: memberCount,
      assign_unassigned_members: assign,
      group_by_section: groupBySection,
    }),
  });
  return data;
}

export async function importGroupSetCSV(categoryId, file) {
  const formData = new FormData();
  formData.append('attachment', file);
  return uploadFile(`/group_categories/${categoryId}/import`, formData);
}

// Downloads the set's memberships as a CSV file.
export async function downloadGroupSetCSV(categoryId, filename = 'groups.csv') {
  const response = await requestRaw(`/group_categories/${categoryId}/export`);
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// userId = null clears the leader.
export async function setGroupLeader(groupId, userId) {
  const { data } = await api.request(`/groups/${groupId}`, {
    method: 'PUT',
    body: JSON.stringify({ group: { leader_id: userId ?? null } }),
  });
  return data;
}

// ---------------------------------------------------------------------------
// Pure helpers (unit-tested)
// ---------------------------------------------------------------------------

// providerFor returns the provider entry for a conference type, or a
// not-configured placeholder.
export function providerFor(providers, type) {
  return (providers || []).find((p) => p.type === type) || { type, configured: false, supports_recording: false };
}

// safeHref returns the URL when it is an absolute http(s) link, else ''.
export function safeHref(url) {
  return typeof url === 'string' && /^https?:\/\//i.test(url.trim()) ? url.trim() : '';
}

// recordingLabel is a short human description of a recording.
export function recordingLabel(rec) {
  const parts = [];
  if (rec?.started_at) {
    parts.push(new Date(rec.started_at).toLocaleString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit',
    }));
  }
  if (rec?.duration_minutes > 0) parts.push(`${rec.duration_minutes} min`);
  return parts.join(' · ') || 'Recording';
}

// importSummary turns an import result into one sentence.
export function importSummary(result) {
  if (!result) return '';
  const n = (count, one, many) => `${count} ${count === 1 ? one : many}`;
  let text = `Imported ${n(result.memberships_created || 0, 'membership', 'memberships')}`;
  if (result.groups_created) text += `, created ${n(result.groups_created, 'group', 'groups')}`;
  text += '.';
  if (result.skipped_count) text += ` Skipped ${n(result.skipped_count, 'row', 'rows')}.`;
  return text;
}

// isLeader reports whether a membership's user leads the group.
export function isLeader(group, member) {
  if (!group || group.leader_id == null || !member) return false;
  return Number(group.leader_id) === Number(member.user_id ?? member.user?.id);
}

// ---------------------------------------------------------------------------
// react-query hooks
// ---------------------------------------------------------------------------

export const groupsConferencesKeys = {
  providers: (courseId) => ['conferences', 'providers', String(courseId)],
  recordings: (conferenceId) => ['conferences', 'recordings', String(conferenceId)],
  groupSetUsers: (categoryId, unassigned) => ['groupSets', String(categoryId), 'users', unassigned ? 'unassigned' : 'all'],
};

export function useConferenceProviders(courseId) {
  return useQuery({
    queryKey: groupsConferencesKeys.providers(courseId),
    queryFn: () => getConferenceProviders(courseId),
    enabled: !!courseId,
    staleTime: 5 * 60 * 1000,
  });
}

export function useConferenceRecordings(conferenceId) {
  return useQuery({
    queryKey: groupsConferencesKeys.recordings(conferenceId),
    queryFn: () => getConferenceRecordings(conferenceId),
    enabled: !!conferenceId,
  });
}

export function useDeleteConferenceRecording(conferenceId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (recordingId) => deleteConferenceRecording(conferenceId, recordingId),
    onSuccess: () => qc.invalidateQueries({ queryKey: groupsConferencesKeys.recordings(conferenceId) }),
  });
}

export function useUnassignedStudents(categoryId, enabled = true) {
  return useQuery({
    queryKey: groupsConferencesKeys.groupSetUsers(categoryId, true),
    queryFn: () => getGroupSetUsers(categoryId, { unassigned: true }),
    enabled: !!categoryId && enabled,
  });
}

// useGroupSetMutation wraps a group-set action: on success it refreshes
// the unassigned roster and calls onChanged so the page reloads groups.
export function useGroupSetMutation(categoryId, mutationFn, onChanged) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ['groupSets', String(categoryId)] });
      if (onChanged) onChanged(data);
    },
  });
}
