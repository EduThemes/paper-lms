import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/quiz/QuizzesSubNav', () => ({ default: () => null }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));
// Stub the RCE so we can drive its value through a plain textarea.
vi.mock('../../components/rce/RichContentEditorV2', () => ({
  default: ({ value, onChange, placeholder }) => (
    <textarea
      data-testid="rce"
      value={value || ''}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    listStimuli: vi.fn(),
    getStimulus: vi.fn(),
    createStimulus: vi.fn(),
    updateStimulus: vi.fn(),
    deleteStimulus: vi.fn(),
    getStimulusQuestions: vi.fn(),
  },
}));
vi.mock('../../services/api', () => ({ api: mockApi }));

import StimulusEditorPage from '../StimulusEditorPage';

function renderPage(path) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/courses/:courseId/stimuli" element={<StimulusEditorPage />} />
        <Route path="/courses/:courseId/stimuli/:stimulusId" element={<StimulusEditorPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('StimulusEditorPage', () => {
  beforeEach(() => {
    Object.values(mockApi).forEach(fn => fn.mockReset());
    mockApi.listStimuli.mockResolvedValue([
      { id: 1, title: 'Passage 1', question_count: 3, updated_at: '2026-01-01' },
    ]);
    mockApi.getStimulus.mockResolvedValue({ id: 1, title: 'Passage 1', content: '<p>hello</p>' });
    mockApi.getStimulusQuestions.mockResolvedValue([]);
  });

  it('renders the stimulus list', async () => {
    renderPage('/courses/1/stimuli');
    expect(await screen.findByText('Passage 1')).toBeInTheDocument();
  });

  it('creates a new stimulus', async () => {
    mockApi.createStimulus.mockResolvedValue({ id: 99, title: 'New', content: '' });
    renderPage('/courses/1/stimuli/new');
    const titleInput = await screen.findByPlaceholderText(/The Lorax/);
    await userEvent.type(titleInput, 'New');
    fireEvent.click(screen.getByRole('button', { name: /Create/ }));
    await waitFor(() => expect(mockApi.createStimulus).toHaveBeenCalledWith('1', {
      title: 'New', content: '',
    }));
  });

  it('loads an existing stimulus into the form and saves edits', async () => {
    mockApi.updateStimulus.mockResolvedValue({});
    renderPage('/courses/1/stimuli/1');
    await waitFor(() => expect(mockApi.getStimulus).toHaveBeenCalledWith('1', '1'));
    const titleInput = await screen.findByDisplayValue('Passage 1');
    fireEvent.change(titleInput, { target: { value: 'Passage 1 (edited)' } });
    fireEvent.click(screen.getByRole('button', { name: /Save/ }));
    await waitFor(() => expect(mockApi.updateStimulus).toHaveBeenCalledWith('1', '1', expect.objectContaining({
      title: 'Passage 1 (edited)',
    })));
  });
});
