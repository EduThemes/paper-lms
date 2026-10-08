import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

vi.mock('../../components/Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../components/CourseNav', () => ({ default: () => null }));
vi.mock('../../components/quiz/QuizzesSubNav', () => ({ default: () => null }));
vi.mock('../../hooks/useIsTeacher', () => ({ default: () => true }));

const { mockApi, mockItemsApi } = vi.hoisted(() => ({
  mockApi: {
    listQuestionBanks: vi.fn(),
    createQuestionBank: vi.fn(),
    updateQuestionBank: vi.fn(),
    deleteQuestionBank: vi.fn(),
    listBankQuestions: vi.fn(),
    addBankQuestion: vi.fn(),
    updateBankQuestion: vi.fn(),
    deleteBankQuestion: vi.fn(),
    getQuizzes: vi.fn(),
    pullBankQuestionsToQuiz: vi.fn(),
  },
  mockItemsApi: {
    setQuestionBankSharing: vi.fn(),
    copyQuestionBank: vi.fn(),
    randomDrawFromQuestionBank: vi.fn(),
    formulaPreview: vi.fn(),
  },
}));
vi.mock('../../services/api', () => ({ api: mockApi }));
vi.mock('../../services/api_quiz_items', () => ({ quizItemsApi: mockItemsApi }));

import QuestionBanksPage from '../QuestionBanksPage';

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/courses/7/question_banks']}>
      <Routes>
        <Route path="/courses/:courseId/question_banks" element={<QuestionBanksPage />} />
      </Routes>
    </MemoryRouter>
  );
}

const ownBank = { id: 1, course_id: 7, title: 'Unit 1', owned: true, shared_with_account: false };
const sharedBank = { id: 2, course_id: 8, title: 'Department pool', owned: false, shared_with_account: true };

describe('QuestionBanksPage (single bank system, G060)', () => {
  beforeEach(() => {
    Object.values(mockApi).forEach(fn => fn.mockReset());
    Object.values(mockItemsApi).forEach(fn => fn.mockReset());
    mockApi.listQuestionBanks.mockResolvedValue([ownBank, sharedBank]);
    mockApi.listBankQuestions.mockResolvedValue([
      { id: 11, question_name: 'Q1', question_type: 'multiple_choice', question_text: '2+2?', points_possible: 1, answers: '[{"id":"a","text":"4","weight":100},{"id":"b","text":"5","weight":0}]' },
    ]);
    mockApi.getQuizzes.mockResolvedValue({ data: [{ id: 99, title: 'Quiz A' }] });
  });

  it('marks shared banks and keeps them read-only', async () => {
    renderPage();
    expect(await screen.findByText('Shared by another course')).toBeInTheDocument();
    const sharedCard = screen.getByText('Department pool').closest('.bg-surface-0');
    expect(within(sharedCard).queryByTitle('Delete question bank')).toBeNull();
    expect(within(sharedCard).getByText('Copy to course')).toBeInTheDocument();
    const ownCard = screen.getByText('Unit 1').closest('.bg-surface-0');
    expect(within(ownCard).getByTitle('Delete question bank')).toBeInTheDocument();
  });

  it('copies a shared bank into the course', async () => {
    mockItemsApi.copyQuestionBank.mockResolvedValue({ question_bank: { id: 3, course_id: 7, title: 'Department pool (copy)', owned: true }, questions_copied: 4 });
    renderPage();
    fireEvent.click(await screen.findByText('Copy to course'));
    await waitFor(() => expect(mockItemsApi.copyQuestionBank).toHaveBeenCalledWith('7', 2));
    expect(await screen.findByText('Department pool (copy)')).toBeInTheDocument();
  });

  it('shares an owned bank with the account', async () => {
    mockItemsApi.setQuestionBankSharing.mockResolvedValue({ ...ownBank, shared_with_account: true });
    renderPage();
    fireEvent.click(await screen.findByText('Share with account'));
    await waitFor(() => expect(mockItemsApi.setQuestionBankSharing).toHaveBeenCalledWith('7', 1, true));
    expect(await screen.findByText('Shared with account')).toBeInTheDocument();
  });

  it('authors any item type and sends the answer key as JSON text', async () => {
    mockApi.addBankQuestion.mockImplementation((c, b, q) => Promise.resolve({ id: 12, ...q }));
    renderPage();
    fireEvent.click(await screen.findByText('Unit 1'));
    fireEvent.click(await screen.findByText('Add Question'));
    fireEvent.change(screen.getByLabelText('Question Name'), { target: { value: 'Capitals' } });
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'matching' } });
    fireEvent.change(screen.getByLabelText('Question Text'), { target: { value: 'Match them' } });
    fireEvent.change(screen.getByLabelText('Prompt 1'), { target: { value: 'France' } });
    fireEvent.change(screen.getByLabelText('Match for prompt 1'), { target: { value: 'Paris' } });
    fireEvent.click(screen.getByText('Save Question'));
    await waitFor(() => expect(mockApi.addBankQuestion).toHaveBeenCalled());
    const sent = mockApi.addBankQuestion.mock.calls[0][2];
    expect(sent.question_type).toBe('matching');
    expect(typeof sent.answers).toBe('string');
    expect(JSON.parse(sent.answers)[0]).toMatchObject({ left: 'France', right: 'Paris' });
  });

  it('draws random questions into a quiz', async () => {
    mockItemsApi.randomDrawFromQuestionBank.mockResolvedValue({ questions_added: 1 });
    renderPage();
    fireEvent.click(await screen.findByText('Unit 1'));
    fireEvent.click(await screen.findByText('Pull to Quiz'));
    fireEvent.change(await screen.findByLabelText('Destination Quiz'), { target: { value: '99' } });
    fireEvent.click(screen.getByLabelText('Random draw'));
    fireEvent.click(screen.getByText(/Draw 1 at random/));
    await waitFor(() => expect(mockItemsApi.randomDrawFromQuestionBank).toHaveBeenCalledWith('7', 1, '99', 1));
  });
});
