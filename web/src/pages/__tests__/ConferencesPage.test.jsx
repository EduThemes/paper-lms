import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
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
    createConference: vi.fn(),
    // api_groups_conferences.js goes through api.request (G098).
    request: vi.fn(),
  },
  requestRaw: vi.fn(),
  uploadFile: vi.fn(),
}));

let mockProviders = [];
let mockRecordings = [];
function routeRequest(path, options = {}) {
  if (path.endsWith('/conferences/providers')) return Promise.resolve({ data: { providers: mockProviders } });
  if (/\/conferences\/\d+\/recordings$/.test(path)) return Promise.resolve({ data: { recordings: mockRecordings } });
  if (options.method === 'DELETE') return Promise.resolve({ data: { delete: true } });
  return Promise.resolve({ data: null });
}

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
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/courses/1/conferences']}>
        <Routes>
          <Route path="/courses/:courseId/conferences" element={<ConferencesPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ConferencesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsTeacher = true;
    mockProviders = [];
    mockRecordings = [];
    api.request.mockImplementation(routeRequest);
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

  // G098: provider recordings are listed with playback links; teachers
  // can delete one.
  test('recordings panel lists provider recordings and deletes one', async () => {
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'BigBlueButton', started_at: null, ended_at: null }],
    });
    mockRecordings = [{
      id: 'rec-1', title: 'Office Hours', duration_minutes: 55, published: true,
      playback_url: 'https://bbb.example.com/playback/rec-1', formats: [],
    }, {
      id: 'rec-2', title: 'Bad link', playback_url: 'javascript:alert(1)', formats: [],
    }];
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Recordings'));

    const watch = await screen.findAllByRole('link', { name: /Watch/ });
    expect(watch).toHaveLength(1);
    expect(watch[0]).toHaveAttribute('href', 'https://bbb.example.com/playback/rec-1');
    expect(screen.getByText(/55 min/)).toBeInTheDocument();

    fireEvent.click(screen.getAllByTitle('Delete recording')[0]);
    await waitFor(() => expect(api.request).toHaveBeenCalledWith('/conferences/42/recordings/rec-1', { method: 'DELETE' }));
  });

  test('students cannot delete recordings', async () => {
    mockIsTeacher = false;
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'BigBlueButton', started_at: null, ended_at: null }],
    });
    mockRecordings = [{ id: 'rec-1', title: 'Office Hours', playback_url: 'https://bbb.example.com/p/1', formats: [] }];
    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Recordings'));
    await screen.findByRole('link', { name: /Watch/ });
    expect(screen.queryByTitle('Delete recording')).not.toBeInTheDocument();
  });

  // G098: with BigBlueButton configured the form offers recording.
  test('recording option appears when the provider supports it and is sent on create', async () => {
    mockProviders = [{ type: 'BigBlueButton', configured: true, supports_recording: true }, { type: 'Zoom', configured: false }];
    api.getConferences.mockResolvedValue({ data: [] });
    api.createConference.mockResolvedValue({ id: 1 });

    renderPage();
    await waitFor(() => expect(screen.getByText('No conferences yet.')).toBeInTheDocument());
    fireEvent.click(screen.getByText('New Conference'));
    const box = await screen.findByLabelText('Allow this conference to be recorded');
    fireEvent.click(box);
    fireEvent.change(screen.getAllByRole('textbox')[0], { target: { value: 'Lab review' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create Conference' }));

    await waitFor(() => expect(api.createConference).toHaveBeenCalled());
    expect(api.createConference.mock.calls[0][1]).toMatchObject({ title: 'Lab review', recording_enabled: true });
  });

  test('a failed join shows inline and keeps the list', async () => {
    mockIsTeacher = false;
    api.getConferences.mockResolvedValue({
      data: [{ id: 42, title: 'Office Hours', conference_type: 'BigBlueButton', started_at: '2026-01-01T10:00:00Z', ended_at: null }],
    });
    api.joinConference.mockRejectedValue(new Error('the conference is not running right now'));

    renderPage();
    await waitFor(() => expect(screen.getByText('Office Hours')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Join Conference'));

    expect(await screen.findByRole('alert')).toHaveTextContent('not running');
    expect(screen.getByText('Office Hours')).toBeInTheDocument();
  });
});
