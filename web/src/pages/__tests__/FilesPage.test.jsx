import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import FilesPage from '../FilesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCourseFolders: vi.fn(),
    getCourseFiles: vi.fn(),
    getSubfolders: vi.fn(),
    getFolderFiles: vi.fn(),
    getEnrollments: vi.fn(),
    uploadCourseFile: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/files']}>
      <Routes>
        <Route path="/courses/:courseId/files" element={<FilesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('FilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.getEnrollments.mockResolvedValue({ data: [] });
  });

  test('shows skeleton while folders and files are loading', () => {
    api.getCourseFolders.mockReturnValue(new Promise(() => {}));
    api.getCourseFiles.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  test('renders folders and files after fetch resolves', async () => {
    api.getCourseFolders.mockResolvedValue({
      data: [{ id: 10, name: 'Week 1' }],
    });
    api.getCourseFiles.mockResolvedValue({
      data: [
        { id: 99, display_name: 'syllabus.pdf', size: 1024, content_type: 'application/pdf', url: '/files/99' },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Week 1')).toBeInTheDocument();
    });
    expect(screen.getByText('syllabus.pdf')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Download/i })).toBeInTheDocument();
  });

  test('shows empty state when no folders or files', async () => {
    api.getCourseFolders.mockResolvedValue({ data: [] });
    api.getCourseFiles.mockResolvedValue({ data: [] });

    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No files or folders yet/i)).toBeInTheDocument();
    });
  });

  // B138: an upload while viewing a folder goes into that folder.
  test('uploads into the folder being viewed', async () => {
    api.getCourseFolders.mockResolvedValue({ data: [{ id: 10, name: 'Week 2' }] });
    api.getCourseFiles.mockResolvedValue({ data: [] });
    api.getSubfolders.mockResolvedValue({ data: [] });
    api.getFolderFiles.mockResolvedValue({ data: [] });
    api.uploadCourseFile.mockResolvedValue({ data: {} });

    const { container } = renderPage();
    fireEvent.click(await screen.findByText('Week 2'));
    await waitFor(() => expect(api.getFolderFiles).toHaveBeenCalledWith(10, 1, 50));

    const input = await waitFor(() => {
      const el = container.querySelector('input[type="file"]');
      expect(el).not.toBeNull();
      return el;
    });
    const file = new File(['hello'], 'handout.pdf', { type: 'application/pdf' });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(api.uploadCourseFile).toHaveBeenCalledWith('1', file, 10));
  });

  // B148: more files than one page → a pager instead of silent truncation.
  test('pages through course files', async () => {
    api.getCourseFolders.mockResolvedValue({ data: [] });
    api.getCourseFiles.mockImplementation((courseId, page) => Promise.resolve(page === 1
      ? { data: [{ id: 1, display_name: 'a.pdf', size: 1, content_type: 'application/pdf', url: '/f/1' }], pagination: { next: 'n' } }
      : { data: [{ id: 2, display_name: 'b.pdf', size: 1, content_type: 'application/pdf', url: '/f/2' }], pagination: { prev: 'p' } }));

    renderPage();
    expect(await screen.findByText('a.pdf')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Next/i }));
    expect(await screen.findByText('b.pdf')).toBeInTheDocument();
    expect(api.getCourseFiles).toHaveBeenLastCalledWith('1', 2, 50);
  });

  test('renders inline error alert on fetch failure', async () => {
    api.getCourseFolders.mockRejectedValue(new Error('network down'));
    api.getCourseFiles.mockRejectedValue(new Error('network down'));

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('network down')).toBeInTheDocument();
    });
  });
});
