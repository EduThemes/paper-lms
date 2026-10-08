import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/quiz/QuizzesSubNav', () => ({ default: () => null }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));

const { mockItemsApi } = vi.hoisted(() => ({
  mockItemsApi: {
    getItemAnalysis: vi.fn(),
    itemAnalysisCsvUrl: (c, q) => `/api/v1/courses/${c}/quizzes/${q}/item_analysis.csv`,
    studentAnalysisCsvUrl: (c, q) => `/api/v1/courses/${c}/quizzes/${q}/student_analysis.csv`,
  },
}));
vi.mock('../../services/api_quiz_items', () => ({
  quizItemsApi: mockItemsApi,
  quizItemsKeys: { itemAnalysis: (c, q) => ['ia', c, q] },
}));

import ItemAnalysisPage, { difficultyLabel, discriminationLabel, groupOptions } from '../ItemAnalysisPage';

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/courses/1/quizzes/9/item-analysis']}>
        <Routes>
          <Route path="/courses/:courseId/quizzes/:quizId/item-analysis" element={<ItemAnalysisPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const report = {
  quiz_id: 9,
  quiz_title: 'Quiz 9',
  anonymous: false,
  summary: {
    respondents: 4, points_possible: 3, mean_score: 2.25, median_score: 2.5, standard_deviation: 0.83,
    high_score: 3, low_score: 1, alpha: 0.71, alpha_item_count: 2, pending_review: 0,
  },
  items: [
    {
      question_id: 1, question_type: 'multiple_choice', question_text: 'Q1?', points_possible: 1,
      respondents: 4, answered: 4, unanswered: 0, correct: 3, partially_correct: 0, incorrect: 1, pending_review: 0,
      average_points: 0.75, difficulty_index: 0.75, discrimination_index: 0.5, point_biserial: 0.62,
      options: [
        { id: 'a', text: 'Alpha', correct: true, count: 3, ratio: 0.75, point_biserial: 0.62 },
        { id: 'b', text: 'Beta', correct: false, count: 1, ratio: 0.25, point_biserial: -0.62 },
      ],
    },
    {
      question_id: 2, question_type: 'fill_in_multiple_blanks', question_text: 'Roses are [c1]', points_possible: 2,
      respondents: 4, answered: 3, unanswered: 1, correct: 2, partially_correct: 1, incorrect: 0, pending_review: 0,
      average_points: 1.5, difficulty_index: 0.75, discrimination_index: -0.1, point_biserial: null,
      responses: [
        { group: 'c1', text: 'red', correct: true, count: 2, ratio: 0.5 },
        { group: 'c1', text: 'blue', correct: false, count: 1, ratio: 0.25 },
      ],
    },
  ],
};

describe('ItemAnalysisPage', () => {
  beforeEach(() => {
    mockItemsApi.getItemAnalysis.mockReset();
  });

  it('renders the server report: summary, indices and distributions', async () => {
    mockItemsApi.getItemAnalysis.mockResolvedValue(report);
    renderPage();
    expect(await screen.findByText('Quiz 9')).toBeInTheDocument();
    await waitFor(() => expect(mockItemsApi.getItemAnalysis).toHaveBeenCalledWith('1', '9'));
    expect(screen.getByText('0.71')).toBeInTheDocument(); // Cronbach alpha
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('3 (75%)')).toBeInTheDocument();
    expect(screen.getByText('c1')).toBeInTheDocument(); // blank group heading
    expect(screen.getByText(/negative — review this item/)).toBeInTheDocument();
  });

  it('links both CSV reports', async () => {
    mockItemsApi.getItemAnalysis.mockResolvedValue(report);
    renderPage();
    const itemCsv = await screen.findByRole('link', { name: /Item analysis CSV/ });
    expect(itemCsv).toHaveAttribute('href', '/api/v1/courses/1/quizzes/9/item_analysis.csv');
    expect(screen.getByRole('link', { name: /Student analysis CSV/ })).toHaveAttribute('href', '/api/v1/courses/1/quizzes/9/student_analysis.csv');
  });

  it('shows the empty state when nobody has finished an attempt', async () => {
    mockItemsApi.getItemAnalysis.mockResolvedValue({ ...report, summary: { ...report.summary, respondents: 0 } });
    renderPage();
    expect(await screen.findByText(/No analysis data yet/)).toBeInTheDocument();
  });

  it('surfaces a load error', async () => {
    mockItemsApi.getItemAnalysis.mockRejectedValue(new Error('boom'));
    renderPage();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });
});

describe('item analysis helpers', () => {
  it('labels difficulty and discrimination', () => {
    expect(difficultyLabel(0.95)).toBe('very easy');
    expect(difficultyLabel(0.5)).toBe('moderate');
    expect(difficultyLabel(0.1)).toBe('hard');
    expect(discriminationLabel(0.45)).toBe('good');
    expect(discriminationLabel(0.25)).toBe('fair');
    expect(discriminationLabel(-0.2)).toMatch(/negative/);
    expect(difficultyLabel(null)).toBe('');
  });

  it('groups options by blank / left item, preserving order', () => {
    const groups = groupOptions([
      { group: 'b', text: '1' }, { group: 'a', text: '2' }, { group: 'b', text: '3' }, { text: '4' },
    ]);
    expect(groups.map(g => g.name)).toEqual(['b', 'a', '']);
    expect(groups[0].options.map(o => o.text)).toEqual(['1', '3']);
  });
});
