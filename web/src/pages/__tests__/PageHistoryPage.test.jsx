import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import PageHistoryPage from '../PageHistoryPage';
import { api } from '../../services/api';
import { pagesFilesApi } from '../../services/api_pages_files';

vi.mock('../../services/api', () => ({ api: { getPage: vi.fn() } }));
vi.mock('../../services/api_pages_files', () => ({
  pagesFilesApi: { listPageRevisions: vi.fn(), restorePageRevision: vi.fn() },
}));
let teacher = true;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => teacher }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav /> }));
vi.mock('../../components/RichContentViewer', () => ({ default: ({ content }) => <div data-testid="viewer">{content}</div> }));

const revisions = [
  { revision_id: 2, title: 'Week 1', body: '<p>Read chapter two</p>', latest: true, updated_at: '2026-10-02T10:00:00Z' },
  { revision_id: 1, title: 'Week 1', body: '<p>Read chapter one</p>', latest: false, updated_at: '2026-10-01T10:00:00Z' },
];

function renderHistory() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/3/pages/week-1/revisions']}>
        <Routes>
          <Route path="/courses/:courseId/pages/:slug/revisions" element={<PageHistoryPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('PageHistoryPage (G065)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    teacher = true;
    api.getPage.mockResolvedValue({ page_id: 9, title: 'Week 1', url: 'week-1', can_edit: true });
    pagesFilesApi.listPageRevisions.mockResolvedValue(revisions);
  });

  test('shows the newest revision as a diff against the one before', async () => {
    renderHistory();
    const diff = await screen.findByTestId('revision-diff');
    expect(diff.querySelector('del').textContent).toBe('one');
    expect(diff.querySelector('ins').textContent).toBe('two');
    // The current revision can't be restored onto itself.
    expect(screen.queryByRole('button', { name: /Restore this revision/i })).toBeNull();
  });

  test('a teacher restores an older revision', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    pagesFilesApi.restorePageRevision.mockResolvedValue({ revision_id: 3, latest: true });
    renderHistory();
    fireEvent.click(await screen.findByRole('button', { name: /Revision 1/ }));
    fireEvent.click(screen.getByRole('button', { name: /Restore this revision/i }));
    await waitFor(() => expect(pagesFilesApi.restorePageRevision).toHaveBeenCalledWith('3', 'week-1', 1));
    expect(await screen.findByRole('status')).toHaveTextContent(/revision 3/);
  });

  test('a student editor can browse but not restore', async () => {
    teacher = false;
    renderHistory();
    fireEvent.click(await screen.findByRole('button', { name: /Revision 1/ }));
    expect(screen.queryByRole('button', { name: /Restore this revision/i })).toBeNull();
  });

  test('without edit rights the history is not requested', async () => {
    api.getPage.mockResolvedValue({ page_id: 9, title: 'Week 1', url: 'week-1', can_edit: false });
    renderHistory();
    expect(await screen.findByText(/don't have permission/)).toBeInTheDocument();
    expect(pagesFilesApi.listPageRevisions).not.toHaveBeenCalled();
  });
});
