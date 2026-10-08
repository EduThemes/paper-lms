import React from 'react';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import FilesPage from '../FilesPage';
import { api } from '../../services/api';
import { pagesFilesApi } from '../../services/api_pages_files';

vi.mock('../../services/api', () => ({
  api: {
    uploadCourseFile: vi.fn(),
    createCourseFolder: vi.fn(),
    deleteFile: vi.fn(),
    deleteFolder: vi.fn(),
  },
}));
vi.mock('../../services/api_pages_files', () => ({
  pagesFilesApi: {
    listCourseFolderTree: vi.fn(),
    listFolderFiles: vi.fn(),
    getCourseQuota: vi.fn(),
    updateFile: vi.fn(),
    replaceFile: vi.fn(),
    updateFolder: vi.fn(),
  },
}));
let teacher = true;
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => teacher }));
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => <nav data-testid="course-nav" /> }));

const tree = [
  { id: 1, name: 'course files', parent_folder_id: null },
  { id: 10, name: 'Week 1', parent_folder_id: 1 },
  { id: 11, name: 'Readings', parent_folder_id: 10 },
];

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/1/files']}>
        <Routes>
          <Route path="/courses/:courseId/files" element={<FilesPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('FilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    teacher = true;
    pagesFilesApi.listCourseFolderTree.mockResolvedValue(tree);
    pagesFilesApi.getCourseQuota.mockResolvedValue({ quota: 500 * 1024 * 1024, quota_used: 1024 * 1024 });
    pagesFilesApi.listFolderFiles.mockResolvedValue({ data: [], pagination: {} });
  });

  test('shows skeleton while loading', () => {
    pagesFilesApi.listCourseFolderTree.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('shows the top level of the tree, its files and the storage used', async () => {
    pagesFilesApi.listFolderFiles.mockResolvedValue({
      data: [{ id: 99, display_name: 'syllabus.pdf', size: 1024, content_type: 'application/pdf', url: '/files/99' }],
      pagination: {},
    });
    renderPage();
    expect(await screen.findByText('Week 1')).toBeInTheDocument();
    expect(screen.queryByText('Readings')).toBeNull();
    expect(screen.queryByText('course files')).toBeNull();
    expect(screen.getByText('syllabus.pdf')).toBeInTheDocument();
    expect(pagesFilesApi.listFolderFiles).toHaveBeenCalledWith('1', null, 1, 50);
    expect(await screen.findByText(/1.0 MB of 500 MB used/)).toBeInTheDocument();
  });

  test('opening a folder lists its contents and uploads land there (B138)', async () => {
    api.uploadCourseFile.mockResolvedValue({});
    const { container } = renderPage();
    fireEvent.click(await screen.findByText('Week 1'));
    await waitFor(() => expect(pagesFilesApi.listFolderFiles).toHaveBeenCalledWith('1', 10, 1, 50));
    expect(screen.getByText('Readings')).toBeInTheDocument();
    const crumbs = screen.getByRole('navigation', { name: 'Folder path' });
    expect(within(crumbs).getByText('Week 1')).toBeInTheDocument();

    const input = container.querySelector('input[type="file"]');
    const file = new File(['hello'], 'handout.pdf', { type: 'application/pdf' });
    fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => expect(api.uploadCourseFile).toHaveBeenCalledWith('1', file, 10));
  });

  test('teachers publish files and edit their settings', async () => {
    pagesFilesApi.listFolderFiles.mockResolvedValue({
      data: [{ id: 5, display_name: 'key.pdf', size: 10, content_type: 'application/pdf', url: '/f/5', locked: true }],
      pagination: {},
    });
    pagesFilesApi.updateFile.mockResolvedValue({});
    renderPage();
    expect(await screen.findByText('Unpublished')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Publish key.pdf' }));
    await waitFor(() => expect(pagesFilesApi.updateFile).toHaveBeenCalledWith('1', 5, { locked: false }));

    fireEvent.click(screen.getByRole('button', { name: 'Settings for key.pdf' }));
    const dialog = await screen.findByRole('form', { name: 'File settings' });
    fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'answer-key.pdf' } });
    fireEvent.change(within(dialog).getByLabelText('Folder'), { target: { value: '11' } });
    fireEvent.click(within(dialog).getByLabelText(/Hidden/));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(pagesFilesApi.updateFile).toHaveBeenLastCalledWith('1', 5, expect.objectContaining({
      display_name: 'answer-key.pdf', parent_folder_id: 11, locked: true, hidden: true, unlock_at: null, lock_at: null,
    })));
  });

  test('a folder cannot be moved into its own subfolder', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Settings for folder Week 1' }));
    const dialog = await screen.findByRole('form', { name: 'Folder settings' });
    const options = within(dialog).getAllByRole('option').map((o) => o.textContent.trim());
    expect(options).toEqual(['Files (top level)']);
  });

  test('students see a date-locked file without a download link', async () => {
    teacher = false;
    pagesFilesApi.listFolderFiles.mockResolvedValue({
      data: [
        { id: 7, display_name: 'week-9.pdf', size: 10, content_type: 'application/pdf', url: '/f/7', locked_for_user: true, unlock_at: '2099-01-01T00:00:00Z' },
        { id: 8, display_name: 'open.pdf', size: 10, content_type: 'application/pdf', url: '/f/8' },
      ],
      pagination: {},
    });
    renderPage();
    expect(await screen.findByText('week-9.pdf')).toBeInTheDocument();
    expect(screen.getAllByRole('link', { name: 'Download' })).toHaveLength(1);
    expect(screen.getByText(/Available/)).toBeInTheDocument();
    expect(pagesFilesApi.getCourseQuota).not.toHaveBeenCalled();
    expect(screen.queryByText('Upload File')).toBeNull();
  });

  test('renders an error alert on fetch failure', async () => {
    pagesFilesApi.listFolderFiles.mockRejectedValue(new Error('network down'));
    renderPage();
    expect(await screen.findByText('network down')).toBeInTheDocument();
  });
});
