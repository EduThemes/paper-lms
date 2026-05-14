import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import RecipesList from '../RecipesList';
import { _resetVocabularyCache } from '../../../hooks/useGamificationVocabulary';

vi.mock('../../../services/api', () => ({
  api: {
    gamification: {
      listRules: vi.fn(),
      createRule: vi.fn(),
      updateRule: vi.fn(),
      deleteRule: vi.fn(),
      getVocabulary: vi.fn(),
      listCurrencies: vi.fn(),
      listBadges: vi.fn(),
    },
  },
}));

import { api } from '../../../services/api';

const VOCAB = {
  triggers: [
    {
      kind: 'OnEvent',
      params: [
        { name: 'verb', type: 'enum', required: true, enum: ['completed'] },
        { name: 'object_type', type: 'enum', required: true, enum: ['Quiz'] },
      ],
    },
  ],
  predicates: [],
  effects: [],
  set_ops: ['AND', 'OR', 'N_OF_M'],
  audiences: ['higher_ed'],
  scopes: ['site'],
  windows: ['day'],
  mastery_levels: ['novice'],
};

beforeEach(() => {
  _resetVocabularyCache();
  vi.clearAllMocks();
  api.gamification.getVocabulary.mockResolvedValue(VOCAB);
  api.gamification.listCurrencies.mockResolvedValue({ currencies: [] });
  api.gamification.listBadges.mockResolvedValue({ badges: [] });
});

const SAMPLE_RULES = {
  rules: [
    {
      id: 1,
      name: 'Award XP on quiz completion',
      description: 'standard',
      audience_level: 'higher_ed',
      enabled: true,
      tenant_id: 1,
      scope_type: 'site',
      scope_id: 1,
      trigger_event: { kind: 'OnEvent', verb: 'completed', object_type: 'Quiz' },
      condition_set: { kind: 'ConditionSet', op: 'AND', children: [] },
      effects: [{ kind: 'AwardCurrency', code: 'xp', amount: 10 }],
      created_at: '2026-05-14T12:00:00Z',
      updated_at: '2026-05-14T12:00:00Z',
    },
    {
      id: 2,
      name: 'Disabled rule',
      description: '',
      audience_level: 'higher_ed',
      enabled: false,
      tenant_id: 1,
      scope_type: 'site',
      scope_id: 1,
      trigger_event: { kind: 'OnSchedule', cron: '0 3 * * *' },
      condition_set: { kind: 'ConditionSet', op: 'OR', children: [{ kind: 'EarnedBadge', badge_id: 5 }] },
      effects: [],
      created_at: '2026-05-14T12:00:00Z',
      updated_at: '2026-05-14T12:00:00Z',
    },
  ],
  total_count: 2,
  page: 1,
  per_page: 50,
};

describe('RecipesList', () => {
  it('renders rows with trigger / condition / effect summaries', async () => {
    api.gamification.listRules.mockResolvedValue(SAMPLE_RULES);
    render(<RecipesList />);
    await waitFor(() => expect(screen.getByText('Award XP on quiz completion')).toBeInTheDocument());

    expect(screen.getByText(/completed Quiz/)).toBeInTheDocument();
    expect(screen.getByText(/cron: 0 3 \* \* \*/)).toBeInTheDocument();
    // Condition counts: empty children → "0 (AND)"; one child → "1 (OR)".
    expect(screen.getByText(/0 \(AND\)/)).toBeInTheDocument();
    expect(screen.getByText(/1 \(OR\)/)).toBeInTheDocument();
    expect(screen.getByText('Enabled')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('hits listRules with course scope when courseId is provided', async () => {
    api.gamification.listRules.mockResolvedValue({ rules: [], total_count: 0 });
    render(<RecipesList courseId={42} />);
    await waitFor(() => expect(api.gamification.listRules).toHaveBeenCalled());
    expect(api.gamification.listRules).toHaveBeenCalledWith({ courseId: 42 });
    expect(screen.getByText(/course #42/i)).toBeInTheDocument();
  });

  it('toggles enabled via the quick action', async () => {
    api.gamification.listRules.mockResolvedValueOnce(SAMPLE_RULES).mockResolvedValueOnce(SAMPLE_RULES);
    api.gamification.updateRule.mockResolvedValue({});
    render(<RecipesList />);
    await waitFor(() => expect(screen.getByText('Award XP on quiz completion')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Disable recipe' }));

    expect(api.gamification.updateRule).toHaveBeenCalledWith(
      1,
      { enabled: false },
      { courseId: undefined },
    );
  });

  it('surfaces server validation errors back to the editor', async () => {
    api.gamification.listRules.mockResolvedValue({ rules: [], total_count: 0 });
    api.gamification.createRule.mockRejectedValue(
      new Error('audience_level must be one of [k5 m68 h912 higher_ed corp pro]'),
    );

    render(<RecipesList />);
    await waitFor(() => expect(api.gamification.listRules).toHaveBeenCalled());

    fireEvent.click(screen.getByRole('button', { name: /New recipe/i }));
    // Dialog title shows "New recipe" too; wait for the form fields
    // to render rather than disambiguating two matching elements.
    await waitFor(() => expect(screen.getByPlaceholderText(/Award XP when/i)).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText(/Award XP when/i), {
      target: { value: 'Will fail' },
    });
    // Set OnEvent verb so trigger_event passes the client-side gate.
    fireEvent.change(screen.getByLabelText(/^Verb/), { target: { value: 'completed' } });
    fireEvent.change(screen.getByLabelText(/^Object Type/), { target: { value: 'Quiz' } });

    fireEvent.click(screen.getByRole('button', { name: /Create recipe/i }));

    await waitFor(() => expect(screen.getByText(/audience_level must be one of/i)).toBeInTheDocument());
  });
});
