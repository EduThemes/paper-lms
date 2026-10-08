// Catalog of all quiz item types (legacy + Wave A additions).
// Used by editor + take components to drive the type-switch render.

export const LEGACY_TYPES = [
  { value: 'multiple_choice', label: 'Multiple Choice' },
  { value: 'true_false', label: 'True/False' },
  { value: 'short_answer', label: 'Short Answer' },
  { value: 'essay', label: 'Essay' },
  { value: 'numerical_question', label: 'Numerical' },
  { value: 'matching', label: 'Matching' },
  { value: 'fill_in_multiple_blanks', label: 'Fill in Multiple Blanks' },
];

export const NEW_TYPES = [
  { value: 'multiple_answer', label: 'Multiple Answer' },
  { value: 'multiple_dropdown', label: 'Multiple Dropdown' },
  { value: 'fill_in_the_blank', label: 'Fill in the Blank' },
  { value: 'formula', label: 'Formula' },
  { value: 'file_upload', label: 'File Upload' },
  { value: 'ordering', label: 'Ordering' },
  { value: 'categorization', label: 'Categorization' },
  { value: 'hot_spot', label: 'Hot Spot' },
  { value: 'text_only', label: 'Text / Instructions Only' },
];

export const ALL_TYPES = [...LEGACY_TYPES, ...NEW_TYPES];

// Types that are auto-graded (no manual review required).
export const AUTO_GRADED_TYPES = new Set([
  'multiple_choice',
  'true_false',
  'short_answer',
  'numerical_question',
  'matching',
  'fill_in_multiple_blanks',
  'multiple_answer',
  'multiple_dropdown',
  'fill_in_the_blank',
  'formula',
  'ordering',
  'categorization',
  'hot_spot',
]);

export const TYPE_LABELS = Object.fromEntries(ALL_TYPES.map(t => [t.value, t.label]));

const makeId = (prefix = 'x') =>
  `${prefix}${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`;

// Parse `question.answers` (JSON string or array) into a JS value. Returns
// a fallback when parsing fails so editors render gracefully.
export function parseAnswers(answers, fallback = []) {
  if (Array.isArray(answers) || (answers && typeof answers === 'object')) return answers;
  if (typeof answers !== 'string' || !answers) return fallback;
  try {
    return JSON.parse(answers);
  } catch {
    return fallback;
  }
}

export function stringifyAnswers(answers) {
  if (typeof answers === 'string') return answers;
  return JSON.stringify(answers ?? []);
}

// Returns the default `answers` JS object for a given question type.
// Always returns an array (sometimes single-element) for backend compatibility.
export function defaultAnswersForType(type) {
  switch (type) {
    case 'true_false':
      return [
        { id: 'true', text: 'True', weight: 100, comments: '' },
        { id: 'false', text: 'False', weight: 0, comments: '' },
      ];
    case 'multiple_choice':
      return [
        { id: makeId('a'), text: '', weight: 100 },
        { id: makeId('a'), text: '', weight: 0 },
        { id: makeId('a'), text: '', weight: 0 },
        { id: makeId('a'), text: '', weight: 0 },
      ];
    case 'short_answer':
      return [{ id: makeId('a'), text: '', weight: 100 }];
    case 'numerical_question':
      // Canvas numerical shape: exact (with margin), range or precision.
      return [{ id: makeId('a'), numerical_answer_type: 'exact_answer', exact: '', margin: '', weight: 100 }];
    case 'matching':
      // Left → right pairs; right_id names the correct right-hand option.
      return [
        { id: makeId('m'), left: '', right: '', right_id: makeId('r'), weight: 100 },
        { id: makeId('m'), left: '', right: '', right_id: makeId('r'), weight: 100 },
      ];
    case 'fill_in_multiple_blanks':
      // One entry per accepted answer per blank; blanks come from the
      // [tokens] the author types in the question text.
      return [];
    case 'multiple_answer':
      return [
        { id: makeId('a'), text: '', weight: 100 },
        { id: makeId('a'), text: '', weight: 100 },
        { id: makeId('a'), text: '', weight: 0 },
        { id: makeId('a'), text: '', weight: 0 },
      ];
    case 'multiple_dropdown':
      // One blank, two options
      return [
        { id: makeId('a'), blank_id: 'blank1', text: 'Option A', weight: 100 },
        { id: makeId('a'), blank_id: 'blank1', text: 'Option B', weight: 0 },
      ];
    case 'fill_in_the_blank':
      return [{ id: makeId('a'), text: '', weight: 100 }];
    case 'formula':
      return [{
        id: makeId('a'),
        text: '',
        weight: 100,
        formula: '',
        variables: [],
        tolerance: 0,
        answer_value: '',
      }];
    case 'ordering':
      return [
        { id: makeId('o'), text: 'First item', position: 0 },
        { id: makeId('o'), text: 'Second item', position: 1 },
        { id: makeId('o'), text: 'Third item', position: 2 },
      ];
    case 'categorization':
      return [{
        id: makeId('a'),
        text: '',
        weight: 100,
        items: [
          { id: makeId('i'), text: '' },
          { id: makeId('i'), text: '' },
        ],
        buckets: [
          { id: makeId('b'), label: 'Bucket A', item_ids: [] },
          { id: makeId('b'), label: 'Bucket B', item_ids: [] },
        ],
      }];
    case 'hot_spot':
      return [{
        id: makeId('a'),
        text: '',
        weight: 100,
        image_url: '',
        regions: [],
      }];
    case 'file_upload':
    case 'text_only':
    default:
      return [];
  }
}

export { makeId };

// Legacy / Canvas spellings older imports persisted → Paper's unified types.
const TYPE_ALIASES = {
  multiple_answers: 'multiple_answer',
  multiple_answers_question: 'multiple_answer',
  multiple_dropdowns: 'multiple_dropdown',
  multiple_dropdowns_question: 'multiple_dropdown',
  multiple_choice_question: 'multiple_choice',
  true_false_question: 'true_false',
  short_answer_question: 'short_answer',
  essay_question: 'essay',
  matching_question: 'matching',
  fill_in_multiple_blanks_question: 'fill_in_multiple_blanks',
  file_upload_question: 'file_upload',
  text_only_question: 'text_only',
  calculated_question: 'formula',
};

export function normalizeQuestionType(type) {
  return TYPE_ALIASES[type] || type;
}

// Matching answers hold left→right pairs plus (for Canvas imports) the
// right-hand option pool flagged with blank_id "_match_options_".
export const MATCH_POOL = '_match_options_';

export function matchingParts(answers) {
  const list = Array.isArray(answers) ? answers : [];
  const pairs = [];
  const pool = [];
  const seen = new Set();
  for (const a of list) {
    if (!a) continue;
    if (a.blank_id === MATCH_POOL) {
      if (a.id != null && !seen.has(a.id)) {
        seen.add(a.id);
        pool.push({ id: a.id, text: a.text });
      }
      continue;
    }
    pairs.push({ key: a.left || a.id, label: a.text || a.left || a.id });
    const right = a.right_id || a.match_id;
    if (right && a.right && !seen.has(right)) {
      seen.add(right);
      pool.push({ id: right, text: a.right });
    }
  }
  return { pairs, pool };
}

// Total points a student can earn (Canvas semantics): every ungrouped
// question counts once; a question group contributes pick_count × the
// per-question points (the group's points_per_item, else the average of its
// questions' points). Per-attempt bank-draw copies (question_bank_entry_id)
// are not authored questions and never count directly.
export function computeQuizPointsPossible(questions = [], groups = []) {
  const pts = (q) => (q.points_possible ?? 1);
  const authored = questions.filter(q => q.question_bank_entry_id == null);
  const groupIds = new Set(groups.map(g => g.id));
  let total = 0;
  for (const q of authored) {
    const gid = q.quiz_question_group_id;
    if (!gid || !groupIds.has(gid)) total += pts(q);
  }
  for (const g of groups) {
    const members = authored.filter(q => q.quiz_question_group_id === g.id);
    const fromBank = Boolean(g.question_bank_id);
    const available = fromBank ? Infinity : members.length;
    const pick = Math.min(Number(g.pick_count) || 1, available);
    if (pick <= 0) continue;
    let per;
    if (g.points_per_item != null) per = Number(g.points_per_item);
    else if (members.length > 0) per = members.reduce((s, q) => s + pts(q), 0) / members.length;
    else per = 1;
    total += pick * per;
  }
  return Math.round(total * 100) / 100;
}
