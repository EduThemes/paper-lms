import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../services/api', () => ({
  api: {
    getPages: vi.fn(),
    getAssignments: vi.fn(),
    getModules: vi.fn(),
    getQuizzes: vi.fn(),
    getCourseFiles: vi.fn(),
    getFileDownloadUrl: (id) => `/api/v1/files/${id}/download`,
  },
}));

import { api } from '../../../services/api';
import { loadType } from '../ContentPicker';

const rows = (n, offset = 0) => Array.from({ length: n }, (_, i) => ({ id: offset + i + 1, title: `Item ${offset + i + 1}`, url: `item-${offset + i + 1}` }));

describe('ContentPicker loadType (B106)', () => {
  beforeEach(() => vi.clearAllMocks());

  it('loads every page of results, not just the first 10', async () => {
    api.getPages.mockImplementation(async (courseId, page) => {
      if (page === 1) return { data: rows(100), pagination: { next: '/next' } };
      if (page === 2) return { data: rows(40, 100), pagination: {} };
      return { data: [], pagination: {} };
    });
    const items = await loadType('pages', 7);
    expect(items).toHaveLength(140);
    expect(api.getPages).toHaveBeenCalledWith(7, 1, 100);
    expect(api.getPages).toHaveBeenCalledWith(7, 2, 100);
  });

  it('links quizzes to the Canvas-style quiz URL (routed to the quiz landing page)', async () => {
    api.getQuizzes.mockResolvedValue({ data: [{ id: 5, title: 'Unit quiz' }], pagination: {} });
    const [quiz] = await loadType('quizzes', 7);
    expect(quiz.href).toBe('/courses/7/quizzes/5');
  });
});
