import React from 'react';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import CollaborationsPage, { safeExternalUrl } from '../CollaborationsPage';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({
  api: {
    getCollaborations: vi.fn(),
    deleteCollaboration: vi.fn(),
    updateCollaboration: vi.fn(),
    createCollaboration: vi.fn(),
  },
}));

vi.mock('../../contexts/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: 'Test User', role: 'user' }, loading: false }),
}));

vi.mock('../../components/Layout', () => ({
  default: ({ children }) => <div>{children}</div>,
}));

vi.mock('../../components/CourseNav', () => ({
  default: () => <nav data-testid="course-nav" />,
}));

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/7/collaborations']}>
      <Routes>
        <Route path="/courses/:courseId/collaborations" element={<CollaborationsPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('CollaborationsPage', () => {
  beforeEach(() => vi.clearAllMocks());

  // B057: helpers take the collaboration id, never the course id.
  test('delete sends the collaboration id, not the course id', async () => {
    api.getCollaborations.mockResolvedValue({
      data: [{ id: 31, title: 'Lab notes', collaboration_type: 'google_docs', url: 'https://docs.example.com/d/1' }],
    });
    api.deleteCollaboration.mockResolvedValue({ delete: true });
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderPage();
    await waitFor(() => expect(screen.getByText('Lab notes')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Delete'));

    await waitFor(() => expect(api.deleteCollaboration).toHaveBeenCalled());
    expect(api.deleteCollaboration).toHaveBeenCalledWith(31);
  });

  test('update sends (id, formData)', async () => {
    api.getCollaborations.mockResolvedValue({
      data: [{ id: 31, title: 'Lab notes', collaboration_type: 'google_docs', url: 'https://docs.example.com/d/1' }],
    });
    api.updateCollaboration.mockResolvedValue({});

    renderPage();
    await waitFor(() => expect(screen.getByText('Lab notes')).toBeInTheDocument());
    fireEvent.click(screen.getByTitle('Edit'));
    fireEvent.click(screen.getByRole('button', { name: /Update Collaboration/i }));

    await waitFor(() => expect(api.updateCollaboration).toHaveBeenCalled());
    const [id, body] = api.updateCollaboration.mock.calls[0];
    expect(id).toBe(31);
    expect(body).toMatchObject({ title: 'Lab notes' });
  });

  // B154: a stored javascript: URL is never rendered as a link.
  test('does not render javascript: URLs as links', async () => {
    api.getCollaborations.mockResolvedValue({
      data: [{ id: 32, title: 'Evil doc', collaboration_type: 'google_docs', url: 'javascript:alert(1)' }],
    });
    renderPage();
    await waitFor(() => expect(screen.getByText('Evil doc')).toBeInTheDocument());
    expect(screen.queryByTitle('Open document')).not.toBeInTheDocument();
  });

  test('safeExternalUrl allows only http(s)', () => {
    expect(safeExternalUrl('https://docs.example.com/d/1')).toBe('https://docs.example.com/d/1');
    expect(safeExternalUrl('javascript:alert(1)')).toBeNull();
    expect(safeExternalUrl(' JavaScript:alert(1)')).toBeNull();
    expect(safeExternalUrl('data:text/html,hi')).toBeNull();
    expect(safeExternalUrl('')).toBeNull();
  });
});
