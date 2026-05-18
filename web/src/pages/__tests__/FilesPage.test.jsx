import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
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

  test('renders inline error alert on fetch failure', async () => {
    api.getCourseFolders.mockRejectedValue(new Error('network down'));
    api.getCourseFiles.mockRejectedValue(new Error('network down'));

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('network down')).toBeInTheDocument();
    });
  });
});
