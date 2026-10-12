import {
  NEW_CONTENT,
  buildModuleItemPayload,
  canCreateContent,
  fetchContentOptions,
  getModuleItemLink,
  isExternalItemLink,
  itemNeedsContent,
} from '../moduleItems';

describe('getModuleItemLink', () => {
  test('links content items to their course pages', () => {
    expect(getModuleItemLink({ type: 'Assignment', content_id: 5 }, 3)).toBe('/courses/3/assignments/5');
    expect(getModuleItemLink({ type: 'Quiz', content_id: 6 }, 3)).toBe('/courses/3/quizzes/6/take');
    expect(getModuleItemLink({ type: 'Discussion', content_id: 7 }, 3)).toBe('/courses/3/discussions/7');
    expect(getModuleItemLink({ type: 'Page', page_url: 'week-1' }, 3)).toBe('/courses/3/pages/week-1');
  });

  // B146: imported file and announcement items used to render as dead rows.
  test('links imported File and Announcement items', () => {
    expect(getModuleItemLink({ type: 'File', content_id: 9 }, 3)).toMatch(/\/files\/9\/download$/);
    expect(getModuleItemLink({ type: 'Attachment', content_id: 9 }, 3)).toMatch(/\/files\/9\/download$/);
    expect(getModuleItemLink({ type: 'Announcement', content_id: 4 }, 3)).toBe('/courses/3/announcements');
    expect(isExternalItemLink({ type: 'File' })).toBe(true);
    expect(isExternalItemLink({ type: 'Assignment' })).toBe(false);
  });

  test('returns null for unlinked content items and headers', () => {
    expect(getModuleItemLink({ type: 'Assignment' }, 3)).toBeNull();
    expect(getModuleItemLink({ type: 'SubHeader' }, 3)).toBeNull();
  });
});

describe('module item content picker', () => {
  const api = {
    getAssignments: vi.fn(),
    getQuizzes: vi.fn(),
    getDiscussionTopics: vi.fn(),
    getPages: vi.fn(),
    createAssignment: vi.fn(),
    createQuiz: vi.fn(),
    createDiscussionTopic: vi.fn(),
  };

  beforeEach(() => vi.clearAllMocks());

  test('content-linked types need content', () => {
    expect(itemNeedsContent('Assignment')).toBe(true);
    expect(itemNeedsContent('Quiz')).toBe(true);
    expect(itemNeedsContent('Discussion')).toBe(true);
    expect(itemNeedsContent('SubHeader')).toBe(false);
    expect(itemNeedsContent('ExternalUrl')).toBe(false);
  });

  test('loads options for each type', async () => {
    api.getAssignments.mockResolvedValue({ data: [{ id: 1, name: 'Essay 1' }] });
    api.getPages.mockResolvedValue({ data: [{ page_id: 2, title: 'Welcome', url: 'welcome' }] });
    expect(await fetchContentOptions(api, 3, 'Assignment')).toEqual([{ id: 1, label: 'Essay 1' }]);
    expect(await fetchContentOptions(api, 3, 'Page')).toEqual([{ id: 2, label: 'Welcome' }]);
    expect(api.getAssignments).toHaveBeenCalledWith(3, 1, 100);
  });

  // B042: an Assignment item is linked to the chosen assignment.
  test('existing selection becomes content_id', async () => {
    const payload = await buildModuleItemPayload(api, 3, { title: '', type: 'Assignment', selection: '12', position: 1 });
    expect(payload).toEqual({ title: '', type: 'Assignment', position: 1, content_id: 12 });
    expect(api.createAssignment).not.toHaveBeenCalled();
  });

  test('a new assignment is created and linked', async () => {
    api.createAssignment.mockResolvedValue({ id: 44 });
    const payload = await buildModuleItemPayload(api, 3, { title: 'Lab 2', type: 'Assignment', selection: NEW_CONTENT, position: 2 });
    expect(api.createAssignment).toHaveBeenCalledWith(3, { name: 'Lab 2', published: false });
    expect(payload.content_id).toBe(44);
  });

  test('a new page is left for the backend to create', async () => {
    const payload = await buildModuleItemPayload(api, 3, { title: 'Notes', type: 'Page', selection: '', position: 1 });
    expect(payload.content_id).toBeUndefined();
  });

  test('a new item without a title is refused', async () => {
    await expect(buildModuleItemPayload(api, 3, { title: ' ', type: 'Quiz', selection: '', position: 1 })).rejects.toThrow(/title/);
  });

  // G064: File and External Tool items link existing content only.
  test('file and external tool items', async () => {
    api.getCourseFiles = vi.fn().mockResolvedValue({ data: [{ id: 5, display_name: 'syllabus.pdf' }] });
    api.getExternalTools = vi.fn().mockResolvedValue({ data: [
      { id: 6, name: 'Lab Sim', workflow_state: 'active' },
      { id: 7, name: 'Retired', workflow_state: 'deleted' },
    ] });
    expect(itemNeedsContent('File')).toBe(true);
    expect(canCreateContent('File')).toBe(false);
    expect(canCreateContent('ExternalTool')).toBe(false);
    expect(canCreateContent('Assignment')).toBe(true);
    expect(await fetchContentOptions(api, 3, 'File')).toEqual([{ id: 5, label: 'syllabus.pdf' }]);
    expect(await fetchContentOptions(api, 3, 'ExternalTool')).toEqual([{ id: 6, label: 'Lab Sim' }]);

    expect(await buildModuleItemPayload(api, 3, { title: '', type: 'File', selection: '5', position: 1 }))
      .toEqual({ title: '', type: 'File', position: 1, content_id: 5 });
    const tool = await buildModuleItemPayload(api, 3, { title: '', type: 'ExternalTool', selection: '6', externalUrl: 'https://labsim.example.org/u/1', newTab: true, position: 2 });
    expect(tool).toEqual({ title: '', type: 'ExternalTool', position: 2, content_id: 6, external_url: 'https://labsim.example.org/u/1', new_tab: true });
    await expect(buildModuleItemPayload(api, 3, { title: 'x', type: 'File', selection: '', position: 1 })).rejects.toThrow(/file/);
    expect(getModuleItemLink({ type: 'ExternalTool', content_id: 6 }, 3)).toBeNull();
  });
});
