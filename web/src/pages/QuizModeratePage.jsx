import React, { useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Users, Clock, Unlock, X } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import Page from '../components/Page';
import useIsTeacher from '../hooks/useIsTeacher';
import { useQuizModeration, useCourseRosterNames, useSetQuizExtensions } from '../services/api_quiz_engine';
import { attemptsLeftLabel, extensionPayload } from '../components/quiz/quizLifecycle';

function ExtensionDialog({ student, name, timed, onClose, onSave, saving }) {
  const [form, setForm] = useState({
    extra_attempts: String(student.extra_attempts ?? 0),
    extra_time: String(student.extra_time ?? 0),
    manually_unlocked: Boolean(student.manually_unlocked),
    extend_from_now: '',
  });
  const set = (f) => (e) => setForm((prev) => ({ ...prev, [f]: e.target.type === 'checkbox' ? e.target.checked : e.target.value }));
  return (
    <div role="dialog" aria-modal="true" aria-labelledby="moderate-title" className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="bg-surface-0 rounded-lg shadow-xl w-full max-w-md p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 id="moderate-title" className="font-semibold text-text-primary">Moderate: {name}</h3>
          <button type="button" onClick={onClose} aria-label="Close" className="p-1 text-text-tertiary hover:text-text-primary">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="space-y-3">
          <div>
            <label htmlFor="ext-attempts" className="block text-sm font-medium text-text-secondary mb-1">Extra attempts</label>
            <input id="ext-attempts" type="number" min="0" max="1000" value={form.extra_attempts} onChange={set('extra_attempts')}
              className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0" />
          </div>
          {timed && (
            <div>
              <label htmlFor="ext-time" className="block text-sm font-medium text-text-secondary mb-1">Extra time on every attempt (minutes)</label>
              <input id="ext-time" type="number" min="0" max="10080" value={form.extra_time} onChange={set('extra_time')}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0" />
            </div>
          )}
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={form.manually_unlocked} onChange={set('manually_unlocked')} />
            Manually unlock the quiz for this student (ignore the availability dates)
          </label>
          {/* Only a timed (or lock-capped) attempt has a deadline to extend. */}
          {student.in_progress_id && student.in_progress_end_at ? (
            <div>
              <label htmlFor="ext-now" className="block text-sm font-medium text-text-secondary mb-1">
                Extend the attempt in progress (minutes from now)
              </label>
              <input id="ext-now" type="number" min="1" max="10080" value={form.extend_from_now} onChange={set('extend_from_now')}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0" placeholder="e.g. 10" />
              {student.in_progress_end_at && (
                <p className="text-xs text-text-tertiary mt-1">Currently ends {new Date(student.in_progress_end_at).toLocaleString()}</p>
              )}
            </div>
          ) : null}
        </div>
        <div className="flex justify-end gap-2 mt-5">
          <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm rounded border border-border-strong">Cancel</button>
          <button
            type="button"
            disabled={saving}
            onClick={() => onSave(extensionPayload(student.user_id, form))}
            className="px-3 py-1.5 text-sm rounded bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * Moderate this quiz (G047): every student's attempts, kept score and
 * extensions — extra attempts, extra time, manual unlock, and extending
 * an attempt in progress. Instructor only.
 */
export default function QuizModeratePage() {
  const { courseId, quizId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const query = useQuizModeration(courseId, quizId);
  const names = useCourseRosterNames(courseId);
  const save = useSetQuizExtensions(courseId, quizId);
  const [editing, setEditing] = useState(null);
  const [message, setMessage] = useState('');

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  const onSave = async (ext) => {
    setMessage('');
    try {
      await save.mutateAsync([ext]);
      setEditing(null);
      setMessage('Saved.');
    } catch (err) {
      setMessage(`Error: ${err.message}`);
    }
  };

  return (
    <Page query={query} title="Moderate quiz">
      {({ quiz, students }) => {
        const nameOf = (uid) => names.data?.[uid] || `User #${uid}`;
        const timed = Boolean(quiz?.time_limit);
        return (
          <Layout>
            <CourseNav />
            <QuizzesSubNav quizId={quizId} />
            <div className="mb-4">
              <Link to={`/courses/${courseId}/quizzes`} className="text-brand-600 hover:underline text-sm">&larr; Back to Quizzes</Link>
              <h2 className="text-2xl font-bold text-text-primary mt-2 flex items-center gap-2">
                <Users className="w-6 h-6" aria-hidden="true" /> Moderate: {quiz?.title}
              </h2>
              <p className="text-sm text-text-tertiary">
                {quiz?.allowed_attempts === -1 ? 'Unlimited attempts' : `${quiz?.allowed_attempts ?? 1} attempt(s) allowed`}
                {timed ? ` · ${quiz.time_limit} minute time limit` : ''}
              </p>
            </div>
            {message && (
              <p role="status" className={`mb-3 text-sm ${message.startsWith('Error') ? 'text-accent-danger' : 'text-accent-success'}`}>{message}</p>
            )}
            <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-text-tertiary">
                    <th scope="col" className="px-4 py-3">Student</th>
                    <th scope="col" className="px-4 py-3">Attempts used</th>
                    <th scope="col" className="px-4 py-3">Attempts left</th>
                    <th scope="col" className="px-4 py-3">Score kept</th>
                    <th scope="col" className="px-4 py-3">Extensions</th>
                    <th scope="col" className="px-4 py-3"><span className="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {students.length === 0 ? (
                    <tr><td colSpan={6} className="px-4 py-8 text-center text-text-tertiary">No students in this course.</td></tr>
                  ) : students.map((s) => (
                    <tr key={s.user_id} className="hover:bg-surface-1">
                      <td className="px-4 py-3">{nameOf(s.user_id)}</td>
                      <td className="px-4 py-3">{s.attempts_used}</td>
                      <td className="px-4 py-3">{attemptsLeftLabel(s)}</td>
                      <td className="px-4 py-3">{s.kept_score ?? '-'}</td>
                      <td className="px-4 py-3 text-text-secondary">
                        <span className="inline-flex flex-wrap gap-2">
                          {s.extra_attempts > 0 && <span>+{s.extra_attempts} attempt{s.extra_attempts === 1 ? '' : 's'}</span>}
                          {s.extra_time > 0 && <span className="inline-flex items-center gap-1"><Clock className="w-3.5 h-3.5" aria-hidden="true" />+{s.extra_time} min</span>}
                          {s.manually_unlocked && <span className="inline-flex items-center gap-1"><Unlock className="w-3.5 h-3.5" aria-hidden="true" />unlocked</span>}
                          {s.in_progress_id ? <span className="text-accent-warning">attempt in progress</span> : null}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button type="button" onClick={() => setEditing(s)} className="text-brand-600 hover:underline">
                          Moderate
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {editing && (
              <ExtensionDialog
                student={editing}
                name={nameOf(editing.user_id)}
                timed={timed}
                saving={save.isPending}
                onClose={() => setEditing(null)}
                onSave={onSave}
              />
            )}
          </Layout>
        );
      }}
    </Page>
  );
}
