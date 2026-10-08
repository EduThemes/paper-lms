// Pure helpers for the quiz lifecycle UI (quiz-engine lane): editor
// settings <-> API payload, manual-grading payloads, answer-key release
// summaries and event-log labels. Kept free of React so vitest covers them.

import { isoToDateTimeLocal, dateTimeLocalToISO } from '../../utils/datetimeLocal';

export const SCORING_POLICIES = [
  { value: 'keep_highest', label: 'Keep highest score' },
  { value: 'keep_latest', label: 'Keep latest score' },
  { value: 'keep_average', label: 'Keep average score' },
];

export const HIDE_RESULTS_OPTIONS = [
  { value: '', label: 'Let students see their results' },
  { value: 'until_after_last_attempt', label: 'Only after their last attempt' },
  { value: 'always', label: 'Never (hide results)' },
];

export const REGRADE_OPTIONS = [
  { value: 'current_correct_only', label: 'Only award points for the new correct answer' },
  { value: 'current_and_previous_correct', label: 'Award points for both the old and the new correct answer' },
  { value: 'full_credit', label: 'Give everyone full credit for this question' },
  { value: 'no_regrade', label: 'Update the answer key only (no score changes)' },
];

const DATE_FIELDS = ['due_at', 'unlock_at', 'lock_at', 'show_correct_answers_at', 'hide_correct_answers_at'];

// quizSettingsFromQuiz maps an API quiz onto the editor's form fields
// (datetime-local strings, '' for empty).
export function quizSettingsFromQuiz(quiz = {}) {
  const out = {
    scoring_policy: quiz.scoring_policy || 'keep_highest',
    show_correct_answers: quiz.show_correct_answers !== false,
    show_correct_answers_last_attempt: Boolean(quiz.show_correct_answers_last_attempt),
    hide_results: quiz.hide_results || '',
    one_time_results: Boolean(quiz.one_time_results),
    assignment_group_id: quiz.assignment_group_id ? String(quiz.assignment_group_id) : '',
  };
  for (const f of DATE_FIELDS) out[f] = isoToDateTimeLocal(quiz[f]);
  return out;
}

// quizSettingsPayload turns the form back into the PUT body fields. Empty
// dates are sent as null so clearing a date clears it on the server.
export function quizSettingsPayload(form = {}) {
  const out = {
    scoring_policy: form.scoring_policy || 'keep_highest',
    show_correct_answers: Boolean(form.show_correct_answers),
    show_correct_answers_last_attempt: Boolean(form.show_correct_answers_last_attempt),
    hide_results: form.hide_results || '',
    one_time_results: Boolean(form.one_time_results),
  };
  for (const f of DATE_FIELDS) out[f] = dateTimeLocalToISO(form[f]) || null;
  if (form.assignment_group_id) out.assignment_group_id = Number(form.assignment_group_id);
  return out;
}

// validateQuizSettings returns a list of human-readable problems.
export function validateQuizSettings(form = {}) {
  const errors = [];
  const t = (f) => (form[f] ? new Date(form[f]).getTime() : null);
  if (t('unlock_at') != null && t('lock_at') != null && t('lock_at') < t('unlock_at')) {
    errors.push('"Available until" must be after "Available from".');
  }
  if (t('due_at') != null && t('lock_at') != null && t('lock_at') < t('due_at')) {
    errors.push('The due date must not be after "Available until".');
  }
  if (t('show_correct_answers_at') != null && t('hide_correct_answers_at') != null &&
      t('hide_correct_answers_at') <= t('show_correct_answers_at')) {
    errors.push('"Hide correct answers at" must be after "Show correct answers at".');
  }
  return errors;
}

// withoutQuizAssignments drops the online_quiz assignments that back the
// given quizzes, for views that list quizzes and assignments side by side
// (a graded quiz then appears once, as the quiz).
export function withoutQuizAssignments(assignments = [], quizzes = []) {
  const linked = new Set(
    (quizzes || []).map((q) => q?.assignment_id).filter((id) => id != null).map(String),
  );
  return (assignments || []).filter((a) => !linked.has(String(a?.id)));
}

// isGradedQuizType: graded quizzes and graded surveys live in the gradebook.
export function isGradedQuizType(type) {
  return !type || type === 'assignment' || type === 'graded_survey';
}

// answerKeyReleaseSummary describes, for the editor, when students will
// see their results and the correct answers.
export function answerKeyReleaseSummary(form = {}) {
  if (form.hide_results === 'always') return 'Students never see their results or the correct answers.';
  const parts = [];
  if (form.hide_results === 'until_after_last_attempt') parts.push('Results appear after the last attempt.');
  if (form.one_time_results) parts.push('Results can be viewed once.');
  if (!form.show_correct_answers) {
    parts.push('Correct answers are never shown.');
  } else if (form.show_correct_answers_last_attempt) {
    parts.push('Correct answers are shown after the last attempt.');
  } else if (form.show_correct_answers_at || form.hide_correct_answers_at) {
    const from = form.show_correct_answers_at ? `from ${new Date(form.show_correct_answers_at).toLocaleString()}` : '';
    const until = form.hide_correct_answers_at ? `until ${new Date(form.hide_correct_answers_at).toLocaleString()}` : '';
    parts.push(`Correct answers are shown ${[from, until].filter(Boolean).join(' ')}.`);
  } else {
    parts.push('Correct answers are shown as soon as an attempt is submitted.');
  }
  return parts.join(' ');
}

// --- manual grading ---------------------------------------------------------

// MANUAL_TYPES are scored by a grader (the auto-grader skips them).
const MANUAL_TYPES = new Set(['essay', 'essay_question', 'file_upload', 'file_upload_question']);

export function needsManualGrading(questionType) {
  return MANUAL_TYPES.has(questionType);
}

// buildScoresPayload produces the PUT .../scores body from the grader's
// edits: {questionId: {score?: string, comment?: string}} plus fudge
// points. Unchanged and blank scores are left out; comments are sent as
// typed (the server stores them as plain text).
export function buildScoresPayload(edits = {}, fudgePoints, originalFudge) {
  const questions = {};
  for (const [id, e] of Object.entries(edits)) {
    const q = {};
    if (e.score !== undefined && e.score !== '' && e.score !== null) {
      const n = Number(e.score);
      if (!Number.isFinite(n)) throw new Error(`Score for question ${id} must be a number.`);
      q.score = n;
    }
    if (e.comment !== undefined) q.comment = e.comment;
    if (Object.keys(q).length) questions[id] = q;
  }
  const payload = { questions };
  if (fudgePoints !== undefined && String(fudgePoints) !== String(originalFudge ?? '')) {
    if (fudgePoints === '' || fudgePoints === null) {
      payload.fudge_points = null;
    } else {
      const n = Number(fudgePoints);
      if (!Number.isFinite(n)) throw new Error('Fudge points must be a number.');
      payload.fudge_points = n;
    }
  }
  return payload;
}

// attemptTotal previews the score the grader will save.
export function attemptTotal(questions = [], answers = [], edits = {}, fudgePoints = 0) {
  const byQ = {};
  for (const a of answers) byQ[a.question_id] = a;
  let total = 0;
  for (const q of questions) {
    const e = edits[q.id];
    if (e && e.score !== undefined && e.score !== '' && Number.isFinite(Number(e.score))) {
      total += Number(e.score);
    } else if (byQ[q.id]?.points != null) {
      total += Number(byQ[q.id].points);
    }
  }
  const fp = Number(fudgePoints);
  if (Number.isFinite(fp)) total += fp;
  return Math.round(total * 100) / 100;
}

// --- event log --------------------------------------------------------------

const EVENT_LABELS = {
  session_started: 'Started the attempt',
  session_resumed: 'Resumed the attempt',
  question_answered: 'Answered a question',
  question_unanswered: 'Cleared an answer',
  question_viewed: 'Viewed a question',
  question_flagged: 'Flagged a question',
  page_blurred: 'Left the quiz page',
  page_focused: 'Came back to the quiz page',
  submission_submitted: 'Submitted the attempt',
  submission_auto_submitted: 'Attempt auto-submitted (time ran out)',
  attempt_extended: 'Attempt time extended',
  manually_graded: 'Graded by an instructor',
  regraded: 'Regraded',
};

export function describeQuizEvent(event = {}) {
  const label = EVENT_LABELS[event.event_type] || event.event_type || 'Event';
  const qid = event.event_data?.question_id;
  return qid ? `${label} (question ${qid})` : label;
}

// suspiciousEventCount counts the take-page events instructors usually
// look for (leaving the page).
export function suspiciousEventCount(events = []) {
  return events.filter((e) => e.event_type === 'page_blurred').length;
}

// --- take page ----------------------------------------------------------------

// effectiveAllowedAttempts is the student's attempt limit (-1 unlimited):
// the server's attempt_status (extra attempts included) when loaded, else
// the quiz setting (0 / missing means unlimited).
export function effectiveAllowedAttempts(quiz, status) {
  if (status && typeof status.allowed_attempts === 'number') return status.allowed_attempts;
  const n = Number(quiz?.allowed_attempts);
  return Number.isFinite(n) && n > 0 ? n : -1;
}

// studentDateLines describes the student's own due and availability dates
// (their section / student overrides applied) for the take page.
export function studentDateLines(status, locale) {
  if (!status) return [];
  const fmt = (iso) => new Date(iso).toLocaleString(locale);
  const lines = [];
  if (status.due_at) lines.push(`Due ${fmt(status.due_at)}`);
  if (status.unlock_at && new Date(status.unlock_at) > new Date()) lines.push(`Available from ${fmt(status.unlock_at)}`);
  if (status.lock_at) lines.push(`Available until ${fmt(status.lock_at)}`);
  if (status.manually_unlocked) lines.push('Your instructor unlocked this quiz for you.');
  return lines;
}

// --- moderation -------------------------------------------------------------

export function attemptsLeftLabel(row = {}) {
  if (row.attempts_left == null || row.attempts_left < 0) return 'Unlimited';
  return String(row.attempts_left);
}

// extensionPayload normalizes the moderate dialog's fields.
export function extensionPayload(userId, form = {}) {
  const intOrUndefined = (v) => {
    if (v === '' || v == null) return undefined;
    const n = Number.parseInt(v, 10);
    return Number.isFinite(n) ? n : undefined;
  };
  const out = { user_id: Number(userId) };
  const extraAttempts = intOrUndefined(form.extra_attempts);
  const extraTime = intOrUndefined(form.extra_time);
  const extendFromNow = intOrUndefined(form.extend_from_now);
  if (extraAttempts !== undefined) out.extra_attempts = Math.max(0, extraAttempts);
  if (extraTime !== undefined) out.extra_time = Math.max(0, extraTime);
  if (form.manually_unlocked !== undefined) out.manually_unlocked = Boolean(form.manually_unlocked);
  if (extendFromNow !== undefined && extendFromNow > 0) out.extend_from_now = extendFromNow;
  return out;
}
