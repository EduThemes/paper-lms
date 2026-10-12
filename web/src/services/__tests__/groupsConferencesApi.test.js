import { api, uploadFile } from '../api';
import {
  providerFor,
  recordingLabel,
  importSummary,
  isLeader,
  safeHref,
  getConferenceRecordings,
  deleteConferenceRecording,
  importGroupSetCSV,
  setGroupLeader,
  getGroupSetUsers,
} from '../api_groups_conferences';

vi.mock('../api', () => ({
  api: { request: vi.fn() },
  requestRaw: vi.fn(),
  uploadFile: vi.fn(),
}));

describe('api_groups_conferences helpers', () => {
  beforeEach(() => vi.clearAllMocks());

  test('providerFor falls back to an unconfigured provider', () => {
    const providers = [{ type: 'BigBlueButton', configured: true, supports_recording: true }];
    expect(providerFor(providers, 'BigBlueButton').configured).toBe(true);
    expect(providerFor(providers, 'Zoom')).toEqual({ type: 'Zoom', configured: false, supports_recording: false });
    expect(providerFor(undefined, 'Zoom').configured).toBe(false);
  });

  test('recordingLabel includes the duration', () => {
    expect(recordingLabel({ duration_minutes: 42 })).toBe('42 min');
    expect(recordingLabel({})).toBe('Recording');
    expect(recordingLabel({ started_at: '2026-01-01T15:00:00Z', duration_minutes: 5 })).toMatch(/5 min$/);
  });

  test('importSummary pluralizes', () => {
    expect(importSummary({ memberships_created: 1 })).toBe('Imported 1 membership.');
    expect(importSummary({ memberships_created: 3, groups_created: 2, skipped_count: 1 }))
      .toBe('Imported 3 memberships, created 2 groups. Skipped 1 row.');
    expect(importSummary(null)).toBe('');
  });

  test('isLeader compares the leader with the member user id', () => {
    expect(isLeader({ leader_id: 51 }, { user_id: 51 })).toBe(true);
    expect(isLeader({ leader_id: '51' }, { user: { id: 51 } })).toBe(true);
    expect(isLeader({ leader_id: 51 }, { user_id: 52 })).toBe(false);
    expect(isLeader({ leader_id: null }, { user_id: 51 })).toBe(false);
  });

  test('safeHref only passes http(s) links', () => {
    expect(safeHref('https://bbb.example.com/p/1')).toBe('https://bbb.example.com/p/1');
    expect(safeHref('javascript:alert(1)')).toBe('');
    expect(safeHref(' JAVASCRIPT:alert(1)')).toBe('');
    expect(safeHref(undefined)).toBe('');
  });

  test('recording calls use the conference id and encode the recording id', async () => {
    api.request.mockResolvedValueOnce({ data: { recordings: [{ id: 'r1' }] } });
    expect(await getConferenceRecordings(42)).toEqual([{ id: 'r1' }]);
    expect(api.request).toHaveBeenCalledWith('/conferences/42/recordings');

    api.request.mockResolvedValueOnce({ data: { delete: true } });
    await deleteConferenceRecording(42, '/abc//def==');
    expect(api.request).toHaveBeenLastCalledWith('/conferences/42/recordings/%2Fabc%2F%2Fdef%3D%3D', { method: 'DELETE' });
  });

  test('group set calls', async () => {
    api.request.mockResolvedValueOnce({ data: [{ id: 1 }] });
    expect(await getGroupSetUsers(7, { unassigned: true })).toEqual([{ id: 1 }]);
    expect(api.request).toHaveBeenCalledWith('/group_categories/7/users?unassigned=true');

    api.request.mockResolvedValueOnce({ data: {} });
    await setGroupLeader(21, undefined);
    expect(api.request).toHaveBeenLastCalledWith('/groups/21', { method: 'PUT', body: JSON.stringify({ group: { leader_id: null } }) });

    uploadFile.mockResolvedValueOnce({ memberships_created: 2 });
    const file = new File(['login_id,group_name\n'], 'groups.csv', { type: 'text/csv' });
    expect(await importGroupSetCSV(7, file)).toEqual({ memberships_created: 2 });
    const [path, form] = uploadFile.mock.calls[0];
    expect(path).toBe('/group_categories/7/import');
    expect(form.get('attachment')).toBeInstanceOf(File);
  });
});
