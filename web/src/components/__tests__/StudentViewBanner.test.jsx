import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import StudentViewBanner, { useStudentViewStatus } from '../StudentViewBanner';
import { pagesFilesApi } from '../../services/api_pages_files';

vi.mock('../../services/api_pages_files', () => ({
  pagesFilesApi: { resetStudentView: vi.fn(), leaveStudentView: vi.fn(), getStudentViewStatus: vi.fn() },
}));

function wrap(ui) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

function StatusProbe({ enabled }) {
  const courseId = useStudentViewStatus(enabled);
  return <p>course:{courseId ?? 'none'}</p>;
}

describe('StudentViewBanner (G076)', () => {
  const assign = vi.fn();
  const realLocation = window.location;
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window, 'location', { configurable: true, value: { ...realLocation, assign } });
  });
  afterEach(() => {
    Object.defineProperty(window, 'location', { configurable: true, value: realLocation });
  });

  test('Reset Student starts over and returns to the course', async () => {
    pagesFilesApi.resetStudentView.mockResolvedValue({ reset: true, student_view: true });
    wrap(<StudentViewBanner courseId={9} />);
    expect(screen.getByText(/viewing this course as a test student/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Reset Student/i }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/courses/9'));
    expect(pagesFilesApi.resetStudentView).toHaveBeenCalledWith(9);
  });

  test('Leave Student View ends the session', async () => {
    pagesFilesApi.leaveStudentView.mockResolvedValue({});
    wrap(<StudentViewBanner courseId={9} />);

    fireEvent.click(screen.getByRole('button', { name: /Leave Student View/i }));
    await waitFor(() => expect(pagesFilesApi.leaveStudentView).toHaveBeenCalled());
    await waitFor(() => expect(assign).toHaveBeenCalledWith('/courses/9'));
  });

  test('useStudentViewStatus asks the server only during a masquerade', async () => {
    pagesFilesApi.getStudentViewStatus.mockResolvedValue({ student_view: true, course_id: 9 });
    const { unmount } = wrap(<StatusProbe enabled={false} />);
    expect(screen.getByText('course:none')).toBeInTheDocument();
    expect(pagesFilesApi.getStudentViewStatus).not.toHaveBeenCalled();
    unmount();

    wrap(<StatusProbe enabled />);
    expect(await screen.findByText('course:9')).toBeInTheDocument();
  });
});
