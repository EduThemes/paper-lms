// Catalog of all quiz item types (legacy + Wave A additions).
// Used by editor + take components to drive the type-switch render.

export const LEGACY_TYPES = [
  { value: 'multiple_choice', label: 'Multiple Choice' },
  { value: 'true_false', label: 'True/False' },
  { value: 'short_answer', label: 'Short Answer' },
  { value: 'essay', label: 'Essay' },
  { value: 'numerical_question', label: 'Numerical' },
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
    case 'numerical_question':
      return [{ id: makeId('a'), text: '', weight: 100 }];
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
