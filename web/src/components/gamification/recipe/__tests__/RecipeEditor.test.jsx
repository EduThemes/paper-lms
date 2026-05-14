import React from 'react';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import RecipeEditor from '../RecipeEditor';
import { _resetVocabularyCache } from '../../../../hooks/useGamificationVocabulary';

// Vocabulary fixture: enough kinds + enums to exercise every editor
// path. Mirrors the shape that internal/service/gamification/
// vocabulary.go serialises today.
const VOCAB = {
  triggers: [
    {
      kind: 'OnEvent',
      params: [
        { name: 'verb', type: 'enum', required: true, enum: ['completed', 'submitted', 'earned'] },
        { name: 'object_type', type: 'enum', required: true, enum: ['Quiz', 'Assignment', 'Badge'] },
      ],
    },
    { kind: 'OnSchedule', params: [{ name: 'cron', type: 'string', required: true }] },
    { kind: 'OnManualTrigger', params: [{ name: 'handle', type: 'string', required: true }] },
  ],
  predicates: [], // not consumed in this test; component reads PREDICATE_KINDS locally
  effects: [],
  set_ops: ['AND', 'OR', 'N_OF_M'],
  audiences: ['k5', 'm68', 'h912', 'higher_ed', 'corp', 'pro'],
  scopes: ['site', 'course'],
  windows: ['day', 'week', 'lifetime'],
  mastery_levels: ['novice', 'familiar', 'proficient', 'mastered'],
};

vi.mock('../../../../services/api', () => ({
  api: {
    gamification: {
      getVocabulary: vi.fn(),
    },
  },
}));

import { api } from '../../../../services/api';

beforeEach(() => {
  _resetVocabularyCache();
  vi.clearAllMocks();
  api.gamification.getVocabulary.mockResolvedValue(VOCAB);
});

describe('RecipeEditor', () => {
  it('opens with the WHEN/IF/THEN scaffold and fetches vocabulary', async () => {
    render(<RecipeEditor open onOpenChange={() => {}} onSave={() => {}} />);

    await waitFor(() => expect(api.gamification.getVocabulary).toHaveBeenCalledTimes(1));

    expect(screen.getByText('New recipe')).toBeInTheDocument();
    expect(screen.getByText('When')).toBeInTheDocument();
    expect(screen.getByText('If')).toBeInTheDocument();
    expect(screen.getByText('Then')).toBeInTheDocument();
    // THEN placeholder explicitly calls out W2-E.3 ownership.
    expect(screen.getByText(/Effects palette lands in W2-E\.3/i)).toBeInTheDocument();
  });

  it('emits a well-formed save payload with a default AND ConditionSet root', async () => {
    const onSave = vi.fn();
    render(<RecipeEditor open onOpenChange={() => {}} onSave={onSave} />);

    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText(/Award XP when a quiz is completed/i), {
      target: { value: 'Test recipe' },
    });

    // OnEvent is the default; pick verb + object_type from the vocab-
    // populated selects.
    const verbSelect = screen.getByLabelText(/^Verb/);
    fireEvent.change(verbSelect, { target: { value: 'completed' } });
    const objSelect = screen.getByLabelText(/^Object Type/);
    fireEvent.change(objSelect, { target: { value: 'Quiz' } });

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));

    expect(onSave).toHaveBeenCalledTimes(1);
    const body = onSave.mock.calls[0][0];
    expect(body.name).toBe('Test recipe');
    expect(body.enabled).toBe(true);
    expect(body.audience_level).toBe('higher_ed');
    expect(body.trigger_event).toEqual({
      kind: 'OnEvent',
      verb: 'completed',
      object_type: 'Quiz',
    });
    expect(body.condition_set).toEqual({
      kind: 'ConditionSet',
      op: 'AND',
      children: [],
    });
    expect(body.effects).toEqual([]);
  });

  it('switches trigger kind and resets per-kind fields', async () => {
    const onSave = vi.fn();
    render(<RecipeEditor open onOpenChange={() => {}} onSave={onSave} />);
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText(/Award XP when/i), {
      target: { value: 'Cron-fired recipe' },
    });

    // Pick OnSchedule, fill cron, save — the OnEvent verb/object_type
    // fields should disappear and not leak into the payload.
    fireEvent.click(screen.getByRole('radio', { name: /On a schedule/i }));
    fireEvent.change(screen.getByLabelText(/^Cron/), { target: { value: '0 3 * * *' } });

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));

    const body = onSave.mock.calls[0][0];
    expect(body.trigger_event).toEqual({ kind: 'OnSchedule', cron: '0 3 * * *' });
    expect(body.trigger_event).not.toHaveProperty('verb');
    expect(body.trigger_event).not.toHaveProperty('object_type');
  });

  it('disables submit when name is blank', async () => {
    render(<RecipeEditor open onOpenChange={() => {}} onSave={() => {}} />);
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());

    const submit = screen.getByRole('button', { name: /Create recipe/i });
    expect(submit).toBeDisabled();
  });

  it('honors saving + saveError props from the parent', async () => {
    render(
      <RecipeEditor
        open
        onOpenChange={() => {}}
        onSave={() => {}}
        saving
        saveError="Validation: trigger_event OnEvent.verb must be one of …"
      />,
    );
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());

    expect(screen.getByRole('button', { name: /Saving…/i })).toBeDisabled();
    expect(screen.getByText(/Validation: trigger_event/i)).toBeInTheDocument();
  });
});

describe('ConditionNode (via RecipeEditor)', () => {
  function setup() {
    const onSave = vi.fn();
    render(<RecipeEditor open onOpenChange={() => {}} onSave={onSave} />);
    return onSave;
  }

  async function fillNameAndTrigger() {
    fireEvent.change(screen.getByPlaceholderText(/Award XP when/i), {
      target: { value: 'Has Conditions' },
    });
    fireEvent.change(screen.getByLabelText(/^Verb/), { target: { value: 'completed' } });
    fireEvent.change(screen.getByLabelText(/^Object Type/), { target: { value: 'Quiz' } });
  }

  it('adds an atomic predicate via the Add Condition menu', async () => {
    const onSave = setup();
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());
    await fillNameAndTrigger();

    const addMenu = screen.getByLabelText('Add condition');
    fireEvent.change(addMenu, { target: { value: 'SubmittedQuiz' } });

    // Fill quiz_id, save, assert the predicate landed.
    fireEvent.change(screen.getByLabelText(/Quiz ID/i), { target: { value: '42' } });
    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));

    const body = onSave.mock.calls[0][0];
    expect(body.condition_set.children).toHaveLength(1);
    expect(body.condition_set.children[0]).toEqual({
      kind: 'SubmittedQuiz',
      quiz_id: 42,
    });
  });

  it('supports N_OF_M with a threshold gated on the operator switch', async () => {
    const onSave = setup();
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());
    await fillNameAndTrigger();

    // AND is default → no threshold input.
    expect(screen.queryByLabelText(/N of M threshold/i)).not.toBeInTheDocument();

    // Switch to N_OF_M, threshold input appears.
    fireEvent.click(screen.getByRole('radio', { name: 'N OF M' }));
    const threshold = screen.getByLabelText(/N of M threshold/i);
    expect(threshold).toBeInTheDocument();

    fireEvent.change(threshold, { target: { value: '2' } });

    // Add two atomic children so threshold is meaningful.
    const addMenu = screen.getByLabelText('Add condition');
    fireEvent.change(addMenu, { target: { value: 'EarnedBadge' } });
    fireEvent.change(screen.getByLabelText(/Badge ID/i), { target: { value: '5' } });

    const addMenu2 = screen.getByLabelText('Add condition');
    fireEvent.change(addMenu2, { target: { value: 'ReputationThreshold' } });
    const minAmounts = screen.getAllByLabelText(/Min amount/i);
    fireEvent.change(minAmounts[minAmounts.length - 1], { target: { value: '100' } });

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));

    const body = onSave.mock.calls[0][0];
    expect(body.condition_set.op).toBe('N_OF_M');
    expect(body.condition_set.threshold).toBe(2);
    expect(body.condition_set.children).toHaveLength(2);
    expect(body.condition_set.children[0]).toEqual({ kind: 'EarnedBadge', badge_id: 5 });
    expect(body.condition_set.children[1]).toEqual({ kind: 'ReputationThreshold', min_amount: 100 });
  });

  it('strips threshold when switching back to AND', async () => {
    const onSave = setup();
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());
    await fillNameAndTrigger();

    fireEvent.click(screen.getByRole('radio', { name: 'N OF M' }));
    fireEvent.click(screen.getByRole('radio', { name: 'AND' }));

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));
    const body = onSave.mock.calls[0][0];
    expect(body.condition_set.op).toBe('AND');
    expect(body.condition_set).not.toHaveProperty('threshold');
  });

  it('removes a predicate when its trash button is clicked', async () => {
    const onSave = setup();
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());
    await fillNameAndTrigger();

    const addMenu = screen.getByLabelText('Add condition');
    fireEvent.change(addMenu, { target: { value: 'EarnedBadge' } });

    fireEvent.click(screen.getByRole('button', { name: /Remove condition 1/i }));

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));
    const body = onSave.mock.calls[0][0];
    expect(body.condition_set.children).toHaveLength(0);
  });

  it('nests groups recursively', async () => {
    const onSave = setup();
    await waitFor(() => expect(screen.getByText('When')).toBeInTheDocument());
    await fillNameAndTrigger();

    fireEvent.click(screen.getByRole('button', { name: /Group/i }));

    // Two "Add condition" menus now: nested (rendered inside the
    // parent's <ul>, so earlier in the DOM) and root (in the parent's
    // footer). Target the nested one — index 0 by DOM order.
    const menus = screen.getAllByLabelText('Add condition');
    expect(menus.length).toBe(2);
    fireEvent.change(menus[0], { target: { value: 'EarnedBadge' } });
    fireEvent.change(screen.getByLabelText(/Badge ID/i), { target: { value: '7' } });

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));
    const body = onSave.mock.calls[0][0];
    expect(body.condition_set.children).toHaveLength(1);
    expect(body.condition_set.children[0].kind).toBe('ConditionSet');
    expect(body.condition_set.children[0].children).toHaveLength(1);
    expect(body.condition_set.children[0].children[0]).toEqual({ kind: 'EarnedBadge', badge_id: 7 });
  });
});

describe('Edit mode pre-populates from an existing recipe', () => {
  it('renders an existing rule and emits a patch payload on save', async () => {
    const onSave = vi.fn();
    const existing = {
      id: 99,
      name: 'Already there',
      description: 'pre-existing',
      audience_level: 'k5',
      enabled: false,
      trigger_event: { kind: 'OnEvent', verb: 'submitted', object_type: 'Assignment' },
      condition_set: {
        kind: 'ConditionSet',
        op: 'OR',
        children: [{ kind: 'CurrencyThreshold', code: 'xp', min_amount: 50 }],
      },
      effects: [{ kind: 'AwardCurrency', code: 'xp', amount: 10 }],
    };

    render(<RecipeEditor open onOpenChange={() => {}} onSave={onSave} recipe={existing} />);
    await waitFor(() => expect(screen.getByText('Edit recipe')).toBeInTheDocument());

    expect(screen.getByDisplayValue('Already there')).toBeInTheDocument();
    expect(screen.getByLabelText(/^Verb/)).toHaveValue('submitted');
    expect(screen.getByLabelText(/^Object Type/)).toHaveValue('Assignment');
    // Op radio reflects existing OR.
    const orRadio = screen.getByRole('radio', { name: 'OR' });
    expect(orRadio).toHaveAttribute('aria-checked', 'true');
    // Effect count is reflected in the placeholder block.
    expect(screen.getByText(/Effects authored in the current draft/i).parentElement.textContent).toMatch(/\b1\b/);

    fireEvent.click(screen.getByRole('button', { name: /Save changes/i }));
    const body = onSave.mock.calls[0][0];
    expect(body.enabled).toBe(false);
    expect(body.effects).toHaveLength(1);
    expect(body.condition_set.children[0]).toEqual({
      kind: 'CurrencyThreshold',
      code: 'xp',
      min_amount: 50,
    });
  });
});
