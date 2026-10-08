import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import SyllabusBodyEditor from '../SyllabusBodyEditor';
import { api } from '../../services/api';

vi.mock('../../services/api', () => ({ api: { updateCourse: vi.fn() } }));
vi.mock('../../hooks/useUnsavedChanges', () => ({ default: () => {} }));
vi.mock('../rce/rceDrafts', () => ({ clearRceDraft: vi.fn() }));
vi.mock('../rce/RichContentEditorV2', () => ({
  default: ({ value, onChange }) => (
    <textarea aria-label="Syllabus content" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

function renderEditor(props) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SyllabusBodyEditor courseId="4" {...props} />
    </QueryClientProvider>,
  );
}

describe('SyllabusBodyEditor (G075)', () => {
  beforeEach(() => vi.clearAllMocks());

  test('saves the syllabus body and reports the stored HTML', async () => {
    api.updateCourse.mockResolvedValue({ id: 4, syllabus_body: '<p>Welcome (sanitized)</p>' });
    const onSaved = vi.fn();
    renderEditor({ body: '', onSaved });

    fireEvent.click(screen.getByRole('button', { name: /Add syllabus description/i }));
    fireEvent.change(screen.getByLabelText('Syllabus content'), { target: { value: '<p>Welcome</p>' } });
    fireEvent.click(screen.getByRole('button', { name: /Save syllabus/i }));

    await waitFor(() => expect(api.updateCourse).toHaveBeenCalledWith('4', { syllabus_body: '<p>Welcome</p>' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('<p>Welcome (sanitized)</p>'));
    expect(screen.getByRole('button', { name: /Add syllabus description/i })).toBeInTheDocument();
  });

  test('shows the server error and stays open', async () => {
    api.updateCourse.mockRejectedValue(new Error('instructor access required'));
    renderEditor({ body: '<p>Old</p>' });
    fireEvent.click(screen.getByRole('button', { name: /Edit syllabus/i }));
    expect(screen.getByLabelText('Syllabus content')).toHaveValue('<p>Old</p>');
    fireEvent.click(screen.getByRole('button', { name: /Save syllabus/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('instructor access required');
    expect(screen.getByRole('form', { name: /Edit syllabus/i })).toBeInTheDocument();
  });
});
