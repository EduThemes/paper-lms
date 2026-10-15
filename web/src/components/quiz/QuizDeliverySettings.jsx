import React, { useEffect, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Save, Shield, Shuffle } from 'lucide-react';
import { quizItemsApi } from '../../services/api_quiz_items';

/**
 * Quiz delivery & security settings (Canvas: shuffle answers, show one
 * question at a time, lock questions after answering, access code, IP
 * filter, anonymous survey submissions). Saved independently of the main
 * quiz settings form via PUT /courses/:id/quizzes/:id.
 */

const IP_ENTRY_RE = /^[0-9a-fA-F:.]+(\/[0-9.]+)?$/;

// Splits a filter the way the server does and returns entries that are
// obviously malformed (the server has the final say).
export function ipFilterProblems(filter) {
  return String(filter || '')
    .split(/[\s,;]+/)
    .map(s => s.trim())
    .filter(Boolean)
    .filter(e => !IP_ENTRY_RE.test(e));
}

export function deliveryFormFromQuiz(quiz = {}) {
  return {
    shuffle_answers: Boolean(quiz.shuffle_answers),
    one_question_at_a_time: Boolean(quiz.one_question_at_a_time),
    cant_go_back: Boolean(quiz.cant_go_back),
    require_access_code: Boolean(quiz.requires_access_code || quiz.access_code),
    access_code: quiz.access_code || '',
    ip_filter: quiz.ip_filter || '',
    anonymous_submissions: Boolean(quiz.anonymous_submissions),
  };
}

// The PUT payload: can't-go-back only applies with one-question-at-a-time;
// unchecking "require an access code" clears it.
export function deliveryPayload(form, isSurvey) {
  return {
    shuffle_answers: form.shuffle_answers,
    one_question_at_a_time: form.one_question_at_a_time,
    cant_go_back: form.one_question_at_a_time && form.cant_go_back,
    access_code: form.require_access_code ? form.access_code.trim() : '',
    ip_filter: form.ip_filter.trim(),
    anonymous_submissions: isSurvey ? form.anonymous_submissions : false,
  };
}

const Check = ({ id, checked, onChange, disabled, children, hint }) => (
  <div className="flex items-start gap-2">
    <input id={id} type="checkbox" className="mt-1" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
    <label htmlFor={id} className={`text-sm ${disabled ? 'text-text-disabled' : 'text-text-primary'}`}>
      {children}
      {hint && <span className="block text-xs text-text-tertiary">{hint}</span>}
    </label>
  </div>
);

const QuizDeliverySettings = ({ courseId, quizId, quiz, onSaved }) => {
  const [form, setForm] = useState(() => deliveryFormFromQuiz(quiz));
  const [dirty, setDirty] = useState(false);
  const isSurvey = quiz?.quiz_type === 'survey' || quiz?.quiz_type === 'graded_survey';

  useEffect(() => {
    if (!dirty) setForm(deliveryFormFromQuiz(quiz));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [quiz]);

  const set = (patch) => { setForm(f => ({ ...f, ...patch })); setDirty(true); };
  const ipProblems = ipFilterProblems(form.ip_filter);
  const codeMissing = form.require_access_code && !form.access_code.trim();

  const mutation = useMutation({
    mutationFn: () => quizItemsApi.updateDeliverySettings(courseId, quizId, deliveryPayload(form, isSurvey)),
    onSuccess: (updated) => {
      setDirty(false);
      if (updated) setForm(deliveryFormFromQuiz(updated));
      onSaved?.(updated);
    },
  });

  return (
    <section className="bg-surface-0 rounded-lg shadow p-6 mb-6" aria-labelledby="quiz-delivery-heading">
      <h3 id="quiz-delivery-heading" className="font-semibold text-lg mb-4 flex items-center gap-2">
        <Shield className="w-5 h-5 text-brand-600" /> Delivery &amp; Security
      </h3>
      <div className="grid md:grid-cols-2 gap-6">
        <div className="space-y-3">
          <Check id="qd-shuffle" checked={form.shuffle_answers} onChange={(v) => set({ shuffle_answers: v })}
            hint="Each student sees multiple-choice, multiple-answer and dropdown options in their own order.">
            <span className="inline-flex items-center gap-1"><Shuffle className="w-3.5 h-3.5" /> Shuffle answers</span>
          </Check>
          <Check id="qd-oqaat" checked={form.one_question_at_a_time}
            onChange={(v) => set({ one_question_at_a_time: v, cant_go_back: v ? form.cant_go_back : false })}
            hint="Students see a single question per page.">
            Show one question at a time
          </Check>
          <div className="pl-6">
            <Check id="qd-cgb" checked={form.cant_go_back} disabled={!form.one_question_at_a_time}
              onChange={(v) => set({ cant_go_back: v })}
              hint="Once a student moves on, they cannot return to (or change) a question.">
              Lock questions after answering
            </Check>
          </div>
          {isSurvey && (
            <Check id="qd-anon" checked={form.anonymous_submissions} onChange={(v) => set({ anonymous_submissions: v })}
              hint="Survey results and reports do not identify students.">
              Keep submissions anonymous
            </Check>
          )}
        </div>
        <div className="space-y-3">
          <Check id="qd-code" checked={form.require_access_code} onChange={(v) => set({ require_access_code: v })}>
            Require an access code
          </Check>
          {form.require_access_code && (
            <div className="pl-6">
              <input type="text" value={form.access_code} onChange={(e) => set({ access_code: e.target.value })}
                className="w-full border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
                placeholder="Access code" aria-label="Access code" autoComplete="off" maxLength={255} />
              {codeMissing && <p className="text-xs text-accent-warning mt-1" role="alert">Enter the code students must type.</p>}
            </div>
          )}
          <div>
            <label htmlFor="qd-ip" className="block text-sm text-text-primary">Filter IP addresses</label>
            <span className="block text-xs text-text-tertiary mb-1">One per line or comma separated: 192.168.1.10, 10.0.0.0/8, 172.16.0.0/255.255.0.0. Leave empty to allow any network.</span>
            <textarea id="qd-ip" rows={3} value={form.ip_filter} onChange={(e) => set({ ip_filter: e.target.value })}
              className="w-full border border-border-strong rounded px-3 py-1.5 text-sm font-mono bg-surface-0 text-text-primary" />
            {ipProblems.length > 0 && (
              <p className="text-xs text-accent-warning mt-1" role="alert">Not an IP address or range: {ipProblems.join(', ')}</p>
            )}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-3 mt-4">
        <button type="button" onClick={() => mutation.mutate()}
          disabled={!dirty || mutation.isPending || ipProblems.length > 0 || codeMissing}
          className="inline-flex items-center gap-2 px-4 py-2 bg-brand-600 text-on-brand rounded hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
          <Save className="w-4 h-4" /> {mutation.isPending ? 'Saving…' : 'Save Delivery Settings'}
        </button>
        {mutation.isError && <span className="text-sm text-accent-danger" role="alert">{mutation.error?.message || 'Could not save'}</span>}
        {mutation.isSuccess && !dirty && <span className="text-sm text-accent-success">Saved.</span>}
      </div>
    </section>
  );
};

export default QuizDeliverySettings;
