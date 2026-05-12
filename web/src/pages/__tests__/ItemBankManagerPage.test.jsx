import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

// Mock heavy dependencies before importing the page under test.
vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/quiz/QuizzesSubNav', () => ({ default: () => null }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));

const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    listQuizItemBanks: vi.fn(),
    createQuizItemBank: vi.fn(),
    updateQuizItemBank: vi.fn(),
    deleteQuizItemBank: vi.fn(),
    listQuizItemBankItems: vi.fn(),
    getQuizzes: vi.fn(),
    addBankItemToQuiz: vi.fn(),
    pullBankQuestionsToQuiz: vi.fn(),
    randomDrawFromBank: vi.fn(),
  },
}));
vi.mock('../../services/api', () => ({ api: mockApi }));

import ItemBankManagerPage from '../ItemBankManagerPage';

function renderPage(courseId = '1') {
  return render(
    <MemoryRouter initialEntries={[`/courses/${courseId}/item-banks`]}>
      <Routes>
        <Route path="/courses/:courseId/item-banks" element={<ItemBankManagerPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ItemBankManagerPage', () => {
  beforeEach(() => {
    Object.values(mockApi).forEach(fn => fn.mockReset());
    mockApi.listQuizItemBanks.mockResolvedValue([
      { id: 1, title: 'Alpha bank', item_count: 5, updated_at: '2026-01-01' },
      { id: 2, title: 'Beta bank', item_count: 0, updated_at: null },
    ]);
    mockApi.listQuizItemBankItems.mockResolvedValue([
      { id: 10, question_type: 'multiple_choice', question_text: 'Q1?', points_possible: 1 },
    ]);
    mockApi.getQuizzes.mockResolvedValue({ data: [{ id: 99, title: 'Target Quiz' }] });
  });

  it('lists banks loaded from the API', async () => {
    renderPage();
    expect(await screen.findByText('Alpha bank')).toBeInTheDocument();
    expect(screen.getByText('Beta bank')).toBeInTheDocument();
  });

  it('creates a new bank', async () => {
    mockApi.createQuizItemBank.mockResolvedValue({ id: 3, title: 'Gamma' });
    renderPage();
    await screen.findByText('Alpha bank');
    fireEvent.click(screen.getByText(/New Bank/));
    const input = screen.getByPlaceholderText(/Bank title/);
    await userEvent.type(input, 'Gamma');
    fireEvent.click(screen.getByText(/Create/));
    await waitFor(() => expect(mockApi.createQuizItemBank).toHaveBeenCalledWith('1', 'Gamma'));
  });

  it('shows the bank items when a bank is clicked', async () => {
    renderPage();
    const row = await screen.findByText('Alpha bank');
    fireEvent.click(row);
    await waitFor(() => expect(mockApi.listQuizItemBankItems).toHaveBeenCalledWith(1));
    expect(await screen.findByText(/Q1\?/)).toBeInTheDocument();
  });

  it('add-to-quiz dialog calls addBankItemToQuiz with the right payload', async () => {
    mockApi.addBankItemToQuiz.mockResolvedValue({});
    renderPage();
    fireEvent.click(await screen.findByText('Alpha bank'));
    await screen.findByText(/Q1\?/);
    fireEvent.click(screen.getByText(/Add to quiz/));
    // Pick quiz + item, then submit
    const dialogSelect = await screen.findByRole('combobox');
    fireEvent.change(dialogSelect, { target: { value: '99' } });
    const checkbox = screen.getByRole('checkbox');
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole('button', { name: /Add 1/ }));
    await waitFor(() => expect(mockApi.addBankItemToQuiz).toHaveBeenCalledWith(1, 10, '99'));
  });

  it('random draw calls randomDrawFromBank with the bank + count', async () => {
    mockApi.randomDrawFromBank.mockResolvedValue({});
    renderPage();
    fireEvent.click(await screen.findByText('Alpha bank'));
    await screen.findByText(/Q1\?/);
    fireEvent.click(screen.getByText(/Random draw/));
    const select = await screen.findByRole('combobox');
    fireEvent.change(select, { target: { value: '99' } });
    fireEvent.click(screen.getByRole('button', { name: /Draw 5/ }));
    await waitFor(() => expect(mockApi.randomDrawFromBank).toHaveBeenCalledWith(1, '99', 5));
  });
});
