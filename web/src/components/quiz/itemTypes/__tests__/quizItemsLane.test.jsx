import React, { useState } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const { mockItemsApi } = vi.hoisted(() => ({
  mockItemsApi: { formulaPreview: vi.fn(), updateDeliverySettings: vi.fn() },
}));
vi.mock('../../../../services/api_quiz_items', () => ({ quizItemsApi: mockItemsApi }));

import MatchingEditor, { toMatchingModel, fromMatchingModel, duplicateLefts } from '../MatchingEditor';
import FillInMultipleBlanksEditor, { toBlankEntries } from '../FillInMultipleBlanksEditor';
import NumericalEditor, { normalizeNumericalAnswer, serializeNumericalAnswer, numericalAnswerProblem } from '../NumericalEditor';
import FormulaEditor, { formulaProblems } from '../FormulaEditor';
import ItemPlayer from '../ItemPlayer';
import { ALL_TYPES, AUTO_GRADED_TYPES, defaultAnswersForType, MATCH_POOL } from '../types';
import QuizDeliverySettings, { ipFilterProblems, deliveryPayload, deliveryFormFromQuiz } from '../../QuizDeliverySettings';
import { navigationPolicy, firstUnlockedIndex, isAccessCodeError } from '../../takeNavigation';
import { parseStoredAnswer, REVIEW_INLINE_TYPES } from '../../reviewAnswers';

function Harness({ Editor, initial, extraProps = {} }) {
  const [answers, setAnswers] = useState(initial);
  return (
    <>
      <Editor answers={answers} onChange={setAnswers} {...extraProps} />
      <textarea data-testid="state" readOnly value={JSON.stringify(answers)} />
    </>
  );
}
const stateOf = () => JSON.parse(screen.getByTestId('state').value);

describe('G050: matching and fill-in-multiple-blanks are authorable types', () => {
  it('registers both types and auto-grades them', () => {
    const values = ALL_TYPES.map(t => t.value);
    expect(values).toContain('matching');
    expect(values).toContain('fill_in_multiple_blanks');
    expect(AUTO_GRADED_TYPES.has('matching')).toBe(true);
    expect(AUTO_GRADED_TYPES.has('fill_in_multiple_blanks')).toBe(true);
    const pairs = defaultAnswersForType('matching');
    expect(pairs).toHaveLength(2);
    expect(pairs[0]).toHaveProperty('right_id');
    expect(defaultAnswersForType('fill_in_multiple_blanks')).toEqual([]);
  });
});

describe('MatchingEditor', () => {
  it('writes pairs with left/right/right_id and distractors into the match pool', () => {
    render(<Harness Editor={MatchingEditor} initial={defaultAnswersForType('matching')} />);
    fireEvent.change(screen.getByLabelText('Prompt 1'), { target: { value: 'France' } });
    fireEvent.change(screen.getByLabelText('Match for prompt 1'), { target: { value: 'Paris' } });
    fireEvent.click(screen.getByText(/Add Distractor/));
    fireEvent.change(screen.getByLabelText('Distractor 1'), { target: { value: 'Rome' } });
    const state = stateOf();
    expect(state[0]).toMatchObject({ left: 'France', right: 'Paris', weight: 100 });
    expect(state[0].right_id).toBeTruthy();
    const pool = state.filter(a => a.blank_id === MATCH_POOL);
    expect(pool).toHaveLength(1);
    expect(pool[0].text).toBe('Rome');
  });

  it('reads the Canvas import shape (text + match_id + pool)', () => {
    const model = toMatchingModel([
      { id: '1', text: 'Cat', match_id: 'm1', weight: 100 },
      { id: 'm1', text: 'Meow', blank_id: MATCH_POOL },
      { id: 'm2', text: 'Moo', blank_id: MATCH_POOL },
    ]);
    expect(model.pairs).toEqual([{ id: '1', left: 'Cat', right: 'Meow', right_id: 'm1' }]);
    expect(model.distractors).toEqual([{ id: 'm2', text: 'Moo' }]);
    const back = fromMatchingModel(model);
    expect(back[0]).toMatchObject({ left: 'Cat', right: 'Meow', right_id: 'm1' });
  });

  it('flags duplicate prompts (they would be ambiguous in a student answer)', () => {
    expect(duplicateLefts([{ left: 'A' }, { left: 'B' }, { left: 'A ' }])).toEqual(['A']);
    render(<Harness Editor={MatchingEditor} initial={[
      { id: 'x', left: 'Same', right: '1', right_id: 'r1' }, { id: 'y', left: 'Same', right: '2', right_id: 'r2' },
    ]} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Same');
  });

  it('the authored question plays back in the matching player', () => {
    const answers = fromMatchingModel({ pairs: [{ id: 'p', left: 'France', right: 'Paris', right_id: 'r1' }], distractors: [{ id: 'r2', text: 'Rome' }] });
    const onChange = vi.fn();
    render(<ItemPlayer question={{ id: 1, question_type: 'matching', answers: JSON.stringify(answers) }} value={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText('Match for France'), { target: { value: 'r1' } });
    expect(onChange).toHaveBeenCalledWith([{ left: 'France', right_id: 'r1' }]);
    expect(screen.getByRole('option', { name: 'Rome' })).toBeInTheDocument();
  });
});

describe('FillInMultipleBlanksEditor', () => {
  it('shows one answer list per [blank] token and stores the Canvas array shape', () => {
    render(<Harness Editor={FillInMultipleBlanksEditor} initial={[]} extraProps={{ questionText: '<p>Roses are [c1], violets are [c2].</p>' }} />);
    expect(screen.getByText('[c1]')).toBeInTheDocument();
    expect(screen.getByText('[c2]')).toBeInTheDocument();
    fireEvent.click(screen.getAllByText(/Add accepted answer/)[0]);
    fireEvent.change(screen.getByLabelText('Accepted answer 1 for c1'), { target: { value: 'red' } });
    expect(stateOf()).toEqual([expect.objectContaining({ blank_id: 'c1', text: 'red', weight: 100 })]);
  });

  it('reads the legacy map shape and surfaces orphaned blanks', () => {
    expect(toBlankEntries({ a: ['x', 'y'] }).map(e => [e.blank_id, e.text])).toEqual([['a', 'x'], ['a', 'y']]);
    render(<Harness Editor={FillInMultipleBlanksEditor} initial={[{ id: '1', blank_id: 'gone', text: 'z', weight: 100 }]} extraProps={{ questionText: 'No tokens' }} />);
    fireEvent.click(screen.getByText(/Remove \(not in question text\)/));
    expect(stateOf()).toEqual([]);
  });
});

describe('G052: NumericalEditor', () => {
  it('reads legacy {text} answers as exact answers', () => {
    expect(normalizeNumericalAnswer({ id: 'a', text: '42', weight: 100 })).toMatchObject({ numerical_answer_type: 'exact_answer', exact: '42' });
    expect(normalizeNumericalAnswer({ start: 1, end: 2 })).toMatchObject({ numerical_answer_type: 'range_answer', start: '1', end: '2' });
  });

  it('serializes only the fields for the chosen kind', () => {
    const exact = serializeNumericalAnswer({ id: 'a', numerical_answer_type: 'exact_answer', exact: '3.5', margin: '5%', start: '9', weight: 100 });
    expect(exact).toEqual({ id: 'a', numerical_answer_type: 'exact_answer', exact: '3.5', margin: '5%', text: '3.5', weight: 100 });
    const range = serializeNumericalAnswer({ id: 'b', numerical_answer_type: 'range_answer', start: '1', end: '2', weight: 100 });
    expect(range).toMatchObject({ start: '1', end: '2' });
    expect(range).not.toHaveProperty('exact');
  });

  it('validates each kind', () => {
    expect(numericalAnswerProblem({ numerical_answer_type: 'exact_answer', exact: '', margin: '' })).toMatch(/exact/);
    expect(numericalAnswerProblem({ numerical_answer_type: 'exact_answer', exact: '1', margin: 'abc' })).toMatch(/Margin/);
    expect(numericalAnswerProblem({ numerical_answer_type: 'range_answer', start: '5', end: '1' })).toMatch(/above/);
    expect(numericalAnswerProblem({ numerical_answer_type: 'precision_answer', approximate: '1.5', precision: '0' })).toMatch(/Precision/);
    expect(numericalAnswerProblem({ numerical_answer_type: 'precision_answer', approximate: '1.5', precision: '2' })).toBe('');
  });

  it('switching to a range and adding a second answer updates the stored key', () => {
    render(<Harness Editor={NumericalEditor} initial={defaultAnswersForType('numerical_question')} />);
    fireEvent.change(screen.getByLabelText('Answer 1 type'), { target: { value: 'range_answer' } });
    fireEvent.change(screen.getByLabelText('Answer 1 range start'), { target: { value: '1' } });
    fireEvent.change(screen.getByLabelText('Answer 1 range end'), { target: { value: '2' } });
    fireEvent.click(screen.getByText(/Add another accepted answer/));
    const state = stateOf();
    expect(state).toHaveLength(2);
    expect(state[0]).toMatchObject({ numerical_answer_type: 'range_answer', start: '1', end: '2' });
    expect(state[1].numerical_answer_type).toBe('exact_answer');
  });
});

describe('G051: FormulaEditor', () => {
  beforeEach(() => mockItemsApi.formulaPreview.mockReset());

  it('validates variable names, ranges and tolerance', () => {
    expect(formulaProblems({ formula: 'x', variables: [{ name: '1x', min: 0, max: 1 }] })[0]).toMatch(/valid variable/);
    expect(formulaProblems({ formula: 'x', variables: [{ name: 'x', min: 5, max: 1 }] })[0]).toMatch(/min is above max/);
    expect(formulaProblems({ formula: 'x', variables: [{ name: 'x', min: 0, max: 1 }, { name: 'x', min: 0, max: 1 }] })[0]).toMatch(/twice/);
    expect(formulaProblems({ formula: 'x', tolerance: 'lots', variables: [] })[0]).toMatch(/Tolerance/);
    expect(formulaProblems({ formula: '', answer_value: '' })[0]).toMatch(/formula/);
    expect(formulaProblems({ formula: 'x*2', tolerance: '5%', variables: [{ name: 'x', min: 1, max: 9 }] })).toEqual([]);
  });

  it('previews sample values from the server', async () => {
    mockItemsApi.formulaPreview.mockResolvedValue([{ variables: [{ name: 'x', value: 3 }], expected: 6 }]);
    const initial = [{ id: 'f', weight: 100, formula: 'x*2', variables: [{ name: 'x', min: 1, max: 9, precision: 0 }], tolerance: '' }];
    render(<Harness Editor={FormulaEditor} initial={initial} extraProps={{ courseId: '1', quizId: '2' }} />);
    fireEvent.click(screen.getByText(/Preview sample values/));
    await waitFor(() => expect(mockItemsApi.formulaPreview).toHaveBeenCalledWith('1', '2', [initial[0]], 5));
    expect(await screen.findByText('6')).toBeInTheDocument();
  });

  it('stores tolerance and answer decimals', () => {
    render(<Harness Editor={FormulaEditor} initial={defaultAnswersForType('formula')} />);
    fireEvent.change(screen.getByLabelText('Tolerance'), { target: { value: '5%' } });
    fireEvent.change(screen.getByLabelText('Answer decimals'), { target: { value: '2' } });
    expect(stateOf()[0]).toMatchObject({ tolerance: '5%', answer_precision: 2 });
  });
});

describe('G049/G055: QuizDeliverySettings', () => {
  beforeEach(() => mockItemsApi.updateDeliverySettings.mockReset());

  const renderSettings = (quiz) => {
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    return render(
      <QueryClientProvider client={client}>
        <QuizDeliverySettings courseId="1" quizId="2" quiz={quiz} />
      </QueryClientProvider>
    );
  };

  it('builds the payload: can\'t-go-back needs one-at-a-time; unchecked code clears it', () => {
    const form = { ...deliveryFormFromQuiz({}), one_question_at_a_time: false, cant_go_back: true, require_access_code: false, access_code: 'x', ip_filter: ' 10.0.0.0/8 ' };
    expect(deliveryPayload(form, false)).toEqual({
      shuffle_answers: false, one_question_at_a_time: false, cant_go_back: false, access_code: '', ip_filter: '10.0.0.0/8', anonymous_submissions: false,
    });
    expect(ipFilterProblems('10.0.0.0/8, nope, 2001:db8::/32')).toEqual(['nope']);
  });

  it('saves shuffle / one-at-a-time / lock / access code / IP filter', async () => {
    mockItemsApi.updateDeliverySettings.mockResolvedValue({ shuffle_answers: true });
    renderSettings({ id: 2, quiz_type: 'assignment' });
    const lock = screen.getByLabelText(/Lock questions after answering/);
    expect(lock).toBeDisabled();
    fireEvent.click(screen.getByLabelText(/Shuffle answers/));
    fireEvent.click(screen.getByLabelText(/Show one question at a time/));
    expect(lock).not.toBeDisabled();
    fireEvent.click(lock);
    fireEvent.click(screen.getByLabelText(/Require an access code/));
    fireEvent.change(screen.getByLabelText('Access code'), { target: { value: 'sesame' } });
    fireEvent.change(screen.getByLabelText(/Filter IP addresses/), { target: { value: '192.168.1.0/24' } });
    fireEvent.click(screen.getByText(/Save Delivery Settings/));
    await waitFor(() => expect(mockItemsApi.updateDeliverySettings).toHaveBeenCalledWith('1', '2', {
      shuffle_answers: true, one_question_at_a_time: true, cant_go_back: true, access_code: 'sesame', ip_filter: '192.168.1.0/24', anonymous_submissions: false,
    }));
  });

  it('blocks saving an invalid IP filter and offers anonymity only for surveys', () => {
    renderSettings({ id: 2, quiz_type: 'survey' });
    expect(screen.getByLabelText(/Keep submissions anonymous/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Filter IP addresses/), { target: { value: 'not-an-ip' } });
    expect(screen.getByText(/Save Delivery Settings/).closest('button')).toBeDisabled();
  });
});

describe('take-page navigation rules', () => {
  it('derives the policy', () => {
    expect(navigationPolicy({})).toEqual({ forceSingle: false, canGoBack: true, locksOnNext: false });
    expect(navigationPolicy({ one_question_at_a_time: true })).toEqual({ forceSingle: true, canGoBack: true, locksOnNext: false });
    expect(navigationPolicy({ one_question_at_a_time: true, cant_go_back: true })).toEqual({ forceSingle: true, canGoBack: false, locksOnNext: true });
    expect(navigationPolicy({ cant_go_back: true }).canGoBack).toBe(true);
  });

  it('resumes at the first unlocked question', () => {
    expect(firstUnlockedIndex([{ locked: true }, { locked: true }, {}])).toBe(2);
    expect(firstUnlockedIndex([{ locked: true }, { locked: true }])).toBe(1);
    expect(firstUnlockedIndex([])).toBe(0);
  });

  it('recognises an access-code refusal', () => {
    expect(isAccessCodeError({ status: 403, message: 'invalid access code' })).toBe(true);
    expect(isAccessCodeError({ status: 403, message: 'IP address denied' })).toBe(false);
  });
});

describe('review answers', () => {
  it('parses structured answers back for the read-only player', () => {
    expect(parseStoredAnswer('matching', '[{"left":"A","right_id":"r"}]')).toEqual([{ left: 'A', right_id: 'r' }]);
    expect(parseStoredAnswer('fill_in_multiple_blanks', '{"c1":"red"}')).toEqual({ c1: 'red' });
    expect(parseStoredAnswer('formula', '12')).toBe('12');
    expect(parseStoredAnswer('matching', 'not json')).toBeUndefined();
    expect(REVIEW_INLINE_TYPES.has('matching')).toBe(false);
  });

  it('locked questions render read-only', () => {
    render(<ItemPlayer question={{ id: 3, question_type: 'short_answer', answers: '[]' }} value="x" onChange={() => {}} readOnly />);
    expect(screen.getByRole('textbox')).toBeDisabled();
  });
});
