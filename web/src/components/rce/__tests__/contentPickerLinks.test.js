import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../services/api', () => ({
  api: {
    getDiscussionTopics: vi.fn(),
    getCourseAnnouncements: vi.fn(),
    getModules: vi.fn(),
    getCourseFiles: vi.fn(),
    getFileDownloadUrl: (id) => `/api/v1/files/${id}/download`,
  },
}));

import { api } from '../../../services/api';
import { loadType, fileEmbedKind } from '../ContentPicker';

describe('ContentPicker link targets (G011)', () => {
  beforeEach(() => vi.clearAllMocks());

  it('links discussions to the topic and announcements to their anchor', async () => {
    api.getDiscussionTopics.mockResolvedValue({ data: [{ id: 4, title: 'Week 1' }], pagination: {} });
    api.getCourseAnnouncements.mockResolvedValue({ data: [{ id: 9, title: 'Welcome' }], pagination: {} });
    expect((await loadType('discussions', 7))[0]).toMatchObject({ label: 'Week 1', href: '/courses/7/discussions/4' });
    expect((await loadType('announcements', 7))[0]).toMatchObject({ label: 'Welcome', href: '/courses/7/announcements#announcement-9' });
  });

  it('links modules to the module anchor', async () => {
    api.getModules.mockResolvedValue({ data: [{ id: 3, name: 'Unit 1' }], pagination: {} });
    expect((await loadType('modules', 7))[0].href).toBe('/courses/7/modules#module-3');
  });

  it('offers embedding for images, audio and video files only', async () => {
    api.getCourseFiles.mockResolvedValue({
      data: [
        { id: 1, display_name: 'diagram.png', content_type: 'image/png' },
        { id: 2, display_name: 'lecture.mp4', content_type: 'video/mp4' },
        { id: 3, display_name: 'notes.pdf', content_type: 'application/pdf' },
        { id: 4, display_name: 'logo.svg', content_type: 'image/svg+xml' },
      ],
      pagination: {},
    });
    const items = await loadType('files', 7);
    expect(items.map((i) => i.embed)).toEqual(['image', 'video', null, null]);
    expect(fileEmbedKind({ filename: 'interview.m4a', content_type: '' })).toBe('audio');
  });
});
