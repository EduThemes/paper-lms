import { normalizeQuestionType } from './itemTypes/types';

// Item types whose stored answer is JSON (arrays / objects) rather than a
// plain string; the review page parses them back for the read-only player.
const STRUCTURED_TYPES = new Set([
  'multiple_answer',
  'multiple_dropdown',
  'matching',
  'fill_in_multiple_blanks',
  'ordering',
  'categorization',
  'hot_spot',
  'file_upload',
]);

// Types the review page renders with its own inline markup; everything
// else is shown through the read-only ItemPlayer.
export const REVIEW_INLINE_TYPES = new Set([
  'multiple_choice',
  'true_false',
  'short_answer',
  'essay',
  'numerical_question',
  'file_upload',
]);

export function parseStoredAnswer(questionType, raw) {
  if (raw == null || raw === '') return undefined;
  if (!STRUCTURED_TYPES.has(normalizeQuestionType(questionType))) return raw;
  if (typeof raw !== 'string') return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}
