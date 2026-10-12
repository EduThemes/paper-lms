import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ConferencesPage from '../ConferencesPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getConferences: vi.fn(),
    deleteConference: vi.fn(),
    startConference: vi.fn(),
    joinConference: vi.fn(),
    endConference: vi.fn(),
    updateConference: vi.fn(),
    getConferenceRecordings: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: 'Test User', role: 'admin' },
    loading: false,
  }),
}));

let mockIsTeacher = true;
vi.mock('../../hooks/useIsTeacher', () => ({
  default: () => mockIsTeacher,
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/conferences']}>
      <Routes>
        <Route path="/courses/:courseId/conferences" element={<ConferencesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ConferencesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = true;
  });

  test('shows loading spinner while fetching conferences', () => {
    api.getConferences.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();
    expect(container.querySelector('.animate-spin')).toBeInTheDocument();
    expect(screen.getByText(/Loading conferences/i)).toBeInTheDocument();
  });

  test('renders conferences when api resolves', async () => {
    api.getConferences.mockResolvedValue({
      data: [
        { id: 1, title: 'Office Hours', description: 'Weekly', conference_type: 'BigBlueButton', duration: 60, started_at: null, ended_at: null },
        { id: 2, title: 'Midterm Review', description: '', conference_type: 'BigBlueButton', duration: 90, started_at: null, ended_at: null },
      ],
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText('Office Hours')).toBeInTheDocument();
    });
    expect(screen.getByText('Midterm Review')).toBeInTheDocument();
  });

  test('shows error UI when api rejects', async () => {
    api.getConferences.mockRejectedValue(new Error('Unavailable'));
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Unavailable')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Try Again/i })).toBeInTheDocument();
  });

  // B056: the conference helpers take the conference id, never courseId.
  test('delete sends the conference id, not the course id', async () => {
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'Zoom', started_at: null, ended_at: null }],
    });
    api.deleteConference.mockResolvedValue({ delete: true });
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Delete'));

    await waitFor(() => expect(api.deleteConference).toHaveBeenCalled());
    expect(api.deleteConference).toHaveBeenCalledWith(42);
  });

  // B058: Start calls the start endpoint, then joins with the real URL.
  test('teacher Start starts the conference then opens the join URL', async () => {
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'Zoom', started_at: null, ended_at: null }],
    });
    api.startConference.mockResolvedValue({ id: 42 });
    api.joinConference.mockResolvedValue({ join_url: 'https://zoom.example.com/j/123' });
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);

    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Start and Join'));

    await waitFor(() => expect(api.joinConference).toHaveBeenCalledWith(42));
    expect(api.startConference).toHaveBeenCalledWith(42);
    expect(open).toHaveBeenCalledWith('https://zoom.example.com/j/123', '_blank', 'noopener,noreferrer');
  });

  test('students do not see Start, Edit or Delete', async () => {
    mockIsTeacher = false;
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'Zoom', started_at: null, ended_at: null }],
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    expect(screen.queryByTitle('Start and Join')).not.toBeInTheDocument();
    expect(screen.queryByTitle('Delete')).not.toBeInTheDocument();
    expect(screen.queryByTitle('Edit')).not.toBeInTheDocument();
  });
});
