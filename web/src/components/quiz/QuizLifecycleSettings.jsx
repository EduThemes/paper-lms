import React from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { CalendarClock, Eye, ListChecks, Users } from 'lucide-react';
import { api } from '../../services/api';
import {
  SCORING_POLICIES,
  HIDE_RESULTS_OPTIONS,
  answerKeyReleaseSummary,
  isGradedQuizType,
} from './quizLifecycle';

const inputCls = 'w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0 text-text-primary';
const labelCls = 'block text-sm font-medium text-text-secondary mb-1';

/**
 * Quiz editor section for the lifecycle settings (quiz-engine lane):
 * dates and per-student / per-section overrides (G046), assignment group
 * (G041), scoring policy (G044) and when students see results and the
 * correct answers (G043). Controlled: `value` is the form slice built by
 * quizSettingsFromQuiz, `onChange(patch)` merges a change.
 */
export default function QuizLifecycleSettings({ courseId, quiz, quizType, value, onChange }) {
  const graded = isGradedQuizType(quizType);
  const groupsQuery = useQuery({
    queryKey: ['quizEngine', String(courseId), 'assignmentGroups'],
    queryFn: async () => {
      const res = await api.getAssignmentGroups(courseId, 1, 100);
      return res?.data || [];
    },
    enabled: Boolean(courseId) && graded,
    staleTime: 60_000,
  });
  const set = (field) => (e) => {
    const v = e.target.type === 'checkbox' ? e.target.checked : e.target.value;
    onChange({ [field]: v });
  };
  const allowed = Number(quiz?.allowed_attempts ?? 1);
  const multipleAttempts = allowed === -1 || allowed > 1;

  return (
    <div className="space-y-5 border-t border-border-default pt-4" data-testid="quiz-lifecycle-settings">
      {/* Dates (G046) */}
      <fieldset>
        <legend className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-2">
          <CalendarClock className="w-4 h-4" aria-hidden="true" /> Dates
        </legend>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label htmlFor="quiz-due-at" className={labelCls}>Due</label>
            <input id="quiz-due-at" type="datetime-local" value={value.due_at} onChange={set('due_at')} className={inputCls} />
          </div>
          <div>
            <label htmlFor="quiz-unlock-at" className={labelCls}>Available from</label>
            <input id="quiz-unlock-at" type="datetime-local" value={value.unlock_at} onChange={set('unlock_at')} className={inputCls} />
          </div>
          <div>
            <label htmlFor="quiz-lock-at" className={labelCls}>Available until</label>
            <input id="quiz-lock-at" type="datetime-local" value={value.lock_at} onChange={set('lock_at')} className={inputCls} />
          </div>
        </div>
        {graded && quiz?.assignment_id ? (
          <p className="text-xs text-text-tertiary mt-2 flex items-center gap-1">
            <Users className="w-3.5 h-3.5" aria-hidden="true" />
            Different dates for a section or for individual students:{' '}
            <Link to={`/courses/${courseId}/assignments/${quiz.assignment_id}/overrides`} className="text-brand-600 hover:underline">
              manage date overrides
            </Link>
            {' '}· per-student extra attempts and time:{' '}
            <Link to={`/courses/${courseId}/quizzes/${quiz.id}/moderate`} className="text-brand-600 hover:underline">
              moderate this quiz
            </Link>
          </p>
        ) : (
          <p className="text-xs text-text-tertiary mt-2">
            {graded
              ? 'Save the quiz to set section and student date overrides.'
              : 'Section and student date overrides are available for graded quizzes.'}
          </p>
        )}
      </fieldset>

      {/* Gradebook (G041 / G044) */}
      <fieldset>
        <legend className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-2">
          <ListChecks className="w-4 h-4" aria-hidden="true" /> Grading
        </legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {graded ? (
            <div>
              <label htmlFor="quiz-assignment-group" className={labelCls}>Assignment group</label>
              <select id="quiz-assignment-group" value={value.assignment_group_id} onChange={set('assignment_group_id')} className={inputCls}>
                <option value="">{groupsQuery.isLoading ? 'Loading…' : 'Course default group'}</option>
                {(groupsQuery.data || []).map((g) => (
                  <option key={g.id} value={String(g.id)}>{g.name}</option>
                ))}
              </select>
              <p className="text-xs text-text-tertiary mt-1">Scores go to the gradebook in this group.</p>
            </div>
          ) : (
            <p className="text-sm text-text-tertiary self-center">
              {quizType === 'practice_quiz' ? 'Practice quizzes' : 'Ungraded surveys'} are not counted in the gradebook.
            </p>
          )}
          <div>
            <label htmlFor="quiz-scoring-policy" className={labelCls}>Score to keep</label>
            <select
              id="quiz-scoring-policy"
              value={value.scoring_policy}
              onChange={set('scoring_policy')}
              className={inputCls}
              disabled={!multipleAttempts}
            >
              {SCORING_POLICIES.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
            </select>
            {!multipleAttempts && <p className="text-xs text-text-tertiary mt-1">Applies when more than one attempt is allowed.</p>}
          </div>
        </div>
      </fieldset>

      {/* Results and answer key (G043) */}
      <fieldset>
        <legend className="flex items-center gap-2 text-sm font-semibold text-text-primary mb-2">
          <Eye className="w-4 h-4" aria-hidden="true" /> Results and correct answers
        </legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label htmlFor="quiz-hide-results" className={labelCls}>Students see their results</label>
            <select id="quiz-hide-results" value={value.hide_results} onChange={set('hide_results')} className={inputCls}>
              {HIDE_RESULTS_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </div>
          <div className="space-y-2 pt-1">
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={value.one_time_results} onChange={set('one_time_results')} disabled={value.hide_results === 'always'} />
              Only once after each attempt
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={value.show_correct_answers} onChange={set('show_correct_answers')} disabled={value.hide_results === 'always'} />
              Let students see the correct answers
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={value.show_correct_answers_last_attempt}
                onChange={set('show_correct_answers_last_attempt')}
                disabled={!value.show_correct_answers || value.hide_results === 'always' || !multipleAttempts}
              />
              Only after their last attempt
            </label>
          </div>
          {value.show_correct_answers && value.hide_results !== 'always' && !value.show_correct_answers_last_attempt && (
            <>
              <div>
                <label htmlFor="quiz-show-answers-at" className={labelCls}>Show correct answers at</label>
                <input id="quiz-show-answers-at" type="datetime-local" value={value.show_correct_answers_at} onChange={set('show_correct_answers_at')} className={inputCls} />
              </div>
              <div>
                <label htmlFor="quiz-hide-answers-at" className={labelCls}>Hide correct answers at</label>
                <input id="quiz-hide-answers-at" type="datetime-local" value={value.hide_correct_answers_at} onChange={set('hide_correct_answers_at')} className={inputCls} />
              </div>
            </>
          )}
        </div>
        <p className="text-xs text-text-tertiary mt-2" aria-live="polite">{answerKeyReleaseSummary(value)}</p>
      </fieldset>
    </div>
  );
}
