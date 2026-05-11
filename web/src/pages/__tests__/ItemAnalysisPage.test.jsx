import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/quiz/QuizzesSubNav', () => ({ default: () => null }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    getQuiz: vi.fn(),
    getQuizQuestions: vi.fn(),
    getQuizItemAnalysis: vi.fn(),
    getQuizSubmissions: vi.fn(),
    getQuizSubmissionAnswers: vi.fn(),
  },
}));
vi.mock('../../services/api', () => ({ api: mockApi }));

import ItemAnalysisPage from '../ItemAnalysisPage';

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/1/quizzes/9/item-analysis']}>
      <Routes>
        <Route path="/courses/:courseId/quizzes/:quizId/item-analysis" element={<ItemAnalysisPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ItemAnalysisPage', () => {
  beforeEach(() => {
    Object.values(mockApi).forEach(fn => fn.mockReset());
    mockApi.getQuiz.mockResolvedValue({ id: 9, title: 'Quiz 9', points_possible: 10 });
    mockApi.getQuizQuestions.mockResolvedValue({
      data: [
        {
          id: 1,
          question_type: 'multiple_choice',
          question_text: 'Q1?',
          points_possible: 2,
          answers: JSON.stringify([
            { id: 'a', text: 'Alpha', weight: 100 },
            { id: 'b', text: 'Beta', weight: 0 },
          ]),
        },
      ],
    });
  });

  it('renders analysis from the backend endpoint when available', async () => {
    mockApi.getQuizItemAnalysis.mockResolvedValue({
      questions: [{
        question_id: 1,
        attempts: 4,
        pct_correct: 0.75,
        avg_points: 1.5,
        option_counts: { a: 3, b: 1 },
        pending_review: 0,
      }],
    });
    renderPage();
    expect(await screen.findByText('Quiz 9')).toBeInTheDocument();
    await waitFor(() => expect(mockApi.getQuizItemAnalysis).toHaveBeenCalledWith('1', '9'));
    expect(await screen.findByText('75%')).toBeInTheDocument();
    expect(screen.getByText(/4 attempts/)).toBeInTheDocument();
  });

  it('falls back to client-side aggregation when the endpoint fails', async () => {
    mockApi.getQuizItemAnalysis.mockRejectedValue(new Error('not implemented'));
    mockApi.getQuizSubmissions.mockResolvedValue({ data: [
      { id: 100, workflow_state: 'complete' },
      { id: 101, workflow_state: 'complete' },
    ]});
    mockApi.getQuizSubmissionAnswers.mockImplementation((c, q, sid) => Promise.resolve([
      { question_id: 1, answer: 'a', points: 2, correct: true },
    ]));
    renderPage();
    await screen.findByText('Quiz 9');
    await waitFor(() => expect(mockApi.getQuizSubmissions).toHaveBeenCalled());
    expect(await screen.findByText('100%')).toBeInTheDocument();
  });

  it('renders the stub placeholder when no questions are present', async () => {
    mockApi.getQuizQuestions.mockResolvedValue({ data: [] });
    mockApi.getQuizItemAnalysis.mockResolvedValue({ questions: [] });
    renderPage();
    expect(await screen.findByText(/No analysis data yet/)).toBeInTheDocument();
  });
});
