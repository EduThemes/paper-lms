import { describe, it, expect, vi } from 'vitest';
import {
  quizSettingsFromQuiz,
  quizSettingsPayload,
  validateQuizSettings,
  isGradedQuizType,
  answerKeyReleaseSummary,
  needsManualGrading,
  buildScoresPayload,
  attemptTotal,
  describeQuizEvent,
  suspiciousEventCount,
  attemptsLeftLabel,
  extensionPayload,
  effectiveAllowedAttempts,
  studentDateLines,
} from '../quizLifecycle';
import { createQuizEventQueue } from '../useQuizEventLog';
import { isQuizAssignment } from '../AssignmentQuizBanner';

describe('quiz settings form (G041 / G043 / G044 / G046)', () => {
  it('defaults a new quiz to Canvas defaults', () => {
    const form = quizSettingsFromQuiz({});
    expect(form.scoring_policy).toBe('keep_highest');
    expect(form.show_correct_answers).toBe(true);
    expect(form.one_time_results).toBe(false);
    expect(form.due_at).toBe('');
    expect(form.assignment_group_id).toBe('');
  });

  it('round-trips dates and settings through the payload', () => {
    const quiz = {
      scoring_policy: 'keep_average',
      show_correct_answers: false,
      hide_results: 'until_after_last_attempt',
      one_time_results: true,
      due_at: '2026-05-01T15:30:00.000Z',
      lock_at: null,
      assignment_group_id: 12,
    };
    const payload = quizSettingsPayload(quizSettingsFromQuiz(quiz));
    expect(payload.scoring_policy).toBe('keep_average');
    expect(payload.show_correct_answers).toBe(false);
    expect(payload.hide_results).toBe('until_after_last_attempt');
    expect(payload.one_time_results).toBe(true);
    expect(payload.due_at).toBe('2026-05-01T15:30:00.000Z');
    // Cleared dates are sent as null so the server clears them.
    expect(payload.lock_at).toBeNull();
    expect(payload.show_correct_answers_at).toBeNull();
    expect(payload.assignment_group_id).toBe(12);
  });

  it('leaves the assignment group out when none is chosen', () => {
    expect(quizSettingsPayload({}).assignment_group_id).toBeUndefined();
  });

  it('flags inverted windows', () => {
    expect(validateQuizSettings({ unlock_at: '2026-05-02T00:00', lock_at: '2026-05-01T00:00' })).toHaveLength(1);
    expect(validateQuizSettings({ due_at: '2026-05-03T00:00', lock_at: '2026-05-01T00:00' })).toHaveLength(1);
    expect(validateQuizSettings({
      show_correct_answers_at: '2026-05-02T00:00', hide_correct_answers_at: '2026-05-02T00:00',
    })).toHaveLength(1);
    expect(validateQuizSettings({ unlock_at: '2026-05-01T00:00', due_at: '2026-05-02T00:00', lock_at: '2026-05-03T00:00' })).toEqual([]);
  });

  it('treats graded quizzes and graded surveys as gradebook items', () => {
    expect(isGradedQuizType('assignment')).toBe(true);
    expect(isGradedQuizType('graded_survey')).toBe(true);
    expect(isGradedQuizType('')).toBe(true);
    expect(isGradedQuizType('practice_quiz')).toBe(false);
    expect(isGradedQuizType('survey')).toBe(false);
  });

  it('summarizes when answers are released', () => {
    expect(answerKeyReleaseSummary({ hide_results: 'always' })).toMatch(/never see/);
    expect(answerKeyReleaseSummary({ show_correct_answers: false })).toMatch(/never shown/);
    expect(answerKeyReleaseSummary({ show_correct_answers: true, show_correct_answers_last_attempt: true })).toMatch(/last attempt/);
    expect(answerKeyReleaseSummary({ show_correct_answers: true })).toMatch(/as soon as/);
    expect(answerKeyReleaseSummary({ show_correct_answers: true, show_correct_answers_at: '2026-05-01T00:00' })).toMatch(/from /);
  });
});

describe('manual grading (G042)', () => {
  it('knows which question types need a grader', () => {
    expect(needsManualGrading('essay')).toBe(true);
    expect(needsManualGrading('file_upload')).toBe(true);
    expect(needsManualGrading('multiple_choice')).toBe(false);
  });

  it('builds the scores payload from edits', () => {
    const payload = buildScoresPayload({ 3: { score: '2.5', comment: 'Good' }, 4: { score: '' }, 5: { comment: '' } }, '1', '');
    expect(payload).toEqual({ questions: { 3: { score: 2.5, comment: 'Good' }, 5: { comment: '' } }, fudge_points: 1 });
  });

  it('clears fudge points with null and skips unchanged fudge points', () => {
    expect(buildScoresPayload({}, '', 2).fudge_points).toBeNull();
    expect('fudge_points' in buildScoresPayload({}, '2', 2)).toBe(false);
    expect('fudge_points' in buildScoresPayload({}, undefined, 2)).toBe(false);
  });

  it('rejects non-numeric scores', () => {
    expect(() => buildScoresPayload({ 1: { score: 'abc' } })).toThrow(/number/);
    expect(() => buildScoresPayload({}, 'x', '')).toThrow(/Fudge/);
  });

  it('previews the attempt total with edits and fudge points', () => {
    const questions = [{ id: 1 }, { id: 2 }, { id: 3 }];
    const answers = [{ question_id: 1, points: 1 }, { question_id: 2, points: 0 }];
    expect(attemptTotal(questions, answers, {}, 0)).toBe(1);
    expect(attemptTotal(questions, answers, { 2: { score: '3' } }, '-0.5')).toBe(3.5);
  });
});

describe('event log (G057)', () => {
  it('labels events', () => {
    expect(describeQuizEvent({ event_type: 'page_blurred' })).toBe('Left the quiz page');
    expect(describeQuizEvent({ event_type: 'question_answered', event_data: { question_id: 7 } })).toBe('Answered a question (question 7)');
    expect(describeQuizEvent({ event_type: 'custom' })).toBe('custom');
    expect(suspiciousEventCount([{ event_type: 'page_blurred' }, { event_type: 'page_focused' }, { event_type: 'page_blurred' }])).toBe(2);
  });

  it('batches events and keeps them when a send fails', async () => {
    const send = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue({});
    const now = () => new Date('2026-05-01T10:00:00Z');
    const q = createQuizEventQueue(send, { now });
    q.push('page_blurred');
    q.push('question_viewed', { question_id: 4 });
    expect(await q.flush()).toBe(0);
    expect(q.size()).toBe(2);
    expect(await q.flush()).toBe(2);
    expect(q.size()).toBe(0);
    expect(send).toHaveBeenLastCalledWith([
      { event_type: 'page_blurred', client_timestamp: '2026-05-01T10:00:00.000Z' },
      { event_type: 'question_viewed', client_timestamp: '2026-05-01T10:00:00.000Z', event_data: { question_id: 4 } },
    ]);
  });

  it('bounds the queue, dropping the oldest events', () => {
    const q = createQuizEventQueue(vi.fn(), { maxQueued: 3 });
    for (let i = 0; i < 5; i++) q.push('page_focused');
    expect(q.size()).toBe(3);
  });

  it('does nothing when the queue is empty', async () => {
    const send = vi.fn();
    expect(await createQuizEventQueue(send).flush()).toBe(0);
    expect(send).not.toHaveBeenCalled();
  });
});

describe('moderation and take page (G046 / G047)', () => {
  it('labels attempts left', () => {
    expect(attemptsLeftLabel({ attempts_left: -1 })).toBe('Unlimited');
    expect(attemptsLeftLabel({ attempts_left: 2 })).toBe('2');
  });

  it('normalizes the moderate dialog', () => {
    expect(extensionPayload('9', { extra_attempts: '2', extra_time: '-5', manually_unlocked: true, extend_from_now: '' }))
      .toEqual({ user_id: 9, extra_attempts: 2, extra_time: 0, manually_unlocked: true });
    expect(extensionPayload(9, { extend_from_now: '15' })).toEqual({ user_id: 9, extend_from_now: 15 });
  });

  it('prefers the server attempt status (extra attempts) over the quiz setting', () => {
    expect(effectiveAllowedAttempts({ allowed_attempts: 1 }, { allowed_attempts: 3 })).toBe(3);
    expect(effectiveAllowedAttempts({ allowed_attempts: 2 }, null)).toBe(2);
    expect(effectiveAllowedAttempts({ allowed_attempts: 0 }, null)).toBe(-1);
    expect(effectiveAllowedAttempts({}, { allowed_attempts: -1 })).toBe(-1);
  });

  it('lists the student own dates', () => {
    expect(studentDateLines(null)).toEqual([]);
    const lines = studentDateLines({
      due_at: '2026-05-01T10:00:00Z',
      unlock_at: '2000-01-01T00:00:00Z',
      lock_at: '2026-05-02T10:00:00Z',
      manually_unlocked: true,
    });
    expect(lines).toHaveLength(3);
    expect(lines[0]).toMatch(/^Due /);
    expect(lines[1]).toMatch(/^Available until /);
    expect(lines[2]).toMatch(/unlocked/);
  });
});

describe('quiz assignments (G041)', () => {
  it('recognizes online_quiz assignments', () => {
    expect(isQuizAssignment({ submission_types: 'online_quiz' })).toBe(true);
    expect(isQuizAssignment({ submission_types: ['online_quiz'] })).toBe(true);
    expect(isQuizAssignment({ submission_types: 'online_text_entry,online_upload' })).toBe(false);
    expect(isQuizAssignment(null)).toBe(false);
  });
});
