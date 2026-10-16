import React, { useMemo, useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Save, History, AlertTriangle, PenLine } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import Page from '../components/Page';
import SafeHTML from '../components/SafeHTML';
import useIsTeacher from '../hooks/useIsTeacher';
import { api } from '../services/api';
import {
  useQuizAttemptForGrading,
  useQuizSubmissionEvents,
  useCourseRosterNames,
  useUpdateQuizScores,
} from '../services/api_quiz_engine';
import {
  buildScoresPayload,
  attemptTotal,
  needsManualGrading,
  describeQuizEvent,
  suspiciousEventCount,
} from '../components/quiz/quizLifecycle';

// formatAnswer renders a stored answer for the grader: choice ids resolve
// to their text, structured answers are shown as compact JSON.
function formatAnswer(question, answer) {
  if (!answer || answer.answer === '' || answer.answer == null) return null;
  const raw = answer.answer;
  if (question.question_type === 'multiple_choice' || question.question_type === 'true_false') {
    try {
      const options = JSON.parse(question.answers || '[]');
      const hit = options.find((o) => String(o.id) === String(raw));
      if (hit) return hit.text;
    } catch { /* fall through */ }
  }
  return raw;
}

function QuestionGrader({ index, question, answer, edit, onEdit, fileUrl }) {
  const manual = needsManualGrading(question.question_type);
  const shown = formatAnswer(question, answer);
  const scoreValue = edit?.score ?? (answer?.points ?? '');
  const commentValue = edit?.comment ?? (answer?.comment || '');
  const ungraded = manual && answer && answer.graded_via !== 'manual' && edit?.score === undefined;
  let file = null;
  if (question.question_type === 'file_upload' && answer?.answer) {
    try { file = JSON.parse(answer.answer); } catch { file = null; }
  }

  return (
    <section className="bg-surface-0 rounded-lg shadow p-5" aria-labelledby={`q-${question.id}-title`}>
      <div className="flex items-start justify-between gap-4 mb-3">
        <h3 id={`q-${question.id}-title`} className="text-sm font-semibold text-text-secondary">
          Question {index + 1}
          <span className="ml-2 text-xs font-normal text-text-tertiary">{question.question_type?.replace(/_/g, ' ')}</span>
          {ungraded && (
            <span className="ml-2 text-xs font-medium px-2 py-0.5 rounded-full bg-accent-warning/20 text-accent-warning">Needs grading</span>
          )}
        </h3>
        <span className="text-xs text-text-tertiary">{question.points_possible ?? 1} pts possible</span>
      </div>
      <SafeHTML className="prose max-w-none text-sm text-text-primary mb-3" html={question.question_text} />

      <div className="mb-3">
        <p className="text-xs font-medium text-text-tertiary mb-1">Student answer</p>
        {file?.attachment_id ? (
          <a href={fileUrl} className="text-sm text-brand-600 hover:underline">{file.filename || 'Download file'}</a>
        ) : shown ? (
          <div className="p-3 rounded border border-border-default bg-surface-1 text-sm whitespace-pre-wrap break-words">{shown}</div>
        ) : (
          <p className="text-sm text-text-tertiary italic">No answer</p>
        )}
        {answer?.graded_via === 'auto' && (
          <p className="text-xs text-text-tertiary mt-1">Auto-graded: {answer.correct ? 'correct' : 'incorrect'}</p>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
        <div>
          <label htmlFor={`score-${question.id}`} className="block text-xs font-medium text-text-secondary mb-1">Score</label>
          <input
            id={`score-${question.id}`}
            type="number"
            step="0.25"
            value={scoreValue}
            onChange={(e) => onEdit({ score: e.target.value })}
            className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0"
          />
        </div>
        <div className="md:col-span-3">
          <label htmlFor={`comment-${question.id}`} className="block text-xs font-medium text-text-secondary mb-1">Comment to the student</label>
          <textarea
            id={`comment-${question.id}`}
            rows={2}
            value={commentValue}
            onChange={(e) => onEdit({ comment: e.target.value })}
            className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0"
          />
        </div>
      </div>
    </section>
  );
}

function EventLog({ courseId, quizId, submissionId }) {
  const query = useQuizSubmissionEvents(courseId, quizId, submissionId);
  if (query.isLoading) return <p className="text-sm text-text-tertiary">Loading log…</p>;
  if (query.isError) return <p className="text-sm text-accent-danger">{query.error?.message || 'Could not load the log.'}</p>;
  const events = query.data || [];
  const leaves = suspiciousEventCount(events);
  return (
    <div>
      {leaves > 0 && (
        <p className="flex items-center gap-1 text-sm text-accent-warning mb-2">
          <AlertTriangle className="w-4 h-4" aria-hidden="true" /> The student left the quiz page {leaves} time{leaves === 1 ? '' : 's'}.
        </p>
      )}
      {events.length === 0 ? (
        <p className="text-sm text-text-tertiary">No events recorded for this attempt.</p>
      ) : (
        <ol className="divide-y divide-border-default text-sm">
          {events.map((e) => (
            <li key={e.id} className="py-1.5 flex items-start justify-between gap-3">
              <span>
                {describeQuizEvent(e)}
                {e.source === 'client' && <span className="ml-2 text-xs text-text-tertiary">(reported by browser)</span>}
              </span>
              <time className="text-xs text-text-tertiary whitespace-nowrap" dateTime={e.created_at}>
                {new Date(e.client_timestamp || e.created_at).toLocaleTimeString()}
              </time>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

/**
 * Grade one quiz attempt (G042): per-question scores and comments, fudge
 * points, and the attempt's event log (G057). Instructor only.
 */
export default function QuizGradePage() {
  const { courseId, quizId, submissionId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const query = useQuizAttemptForGrading(courseId, quizId, submissionId);
  const names = useCourseRosterNames(courseId);
  const save = useUpdateQuizScores(courseId, quizId, submissionId);
  const [edits, setEdits] = useState({});
  const [fudge, setFudge] = useState(undefined);
  const [message, setMessage] = useState('');

  const originalFudge = query.data?.submission?.fudge_points ?? '';
  const fudgeValue = fudge ?? originalFudge;
  const preview = useMemo(() => {
    if (!query.data) return 0;
    return attemptTotal(query.data.questions, query.data.answers, edits, fudgeValue === '' ? 0 : fudgeValue);
  }, [query.data, edits, fudgeValue]);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  const onSave = async () => {
    setMessage('');
    let payload;
    try {
      payload = buildScoresPayload(edits, fudge, originalFudge);
    } catch (err) {
      setMessage(`Error: ${err.message}`);
      return;
    }
    try {
      await save.mutateAsync(payload);
      setEdits({});
      setFudge(undefined);
      setMessage('Scores saved.');
    } catch (err) {
      setMessage(`Error: ${err.message}`);
    }
  };

  return (
    <Page query={query} title="Grade quiz attempt">
      {({ quiz, submission, questions, answers }) => {
        const byQ = {};
        for (const a of answers) byQ[a.question_id] = a;
        const student = names.data?.[submission.user_id] || `User #${submission.user_id}`;
        const dirty = Object.keys(edits).length > 0 || fudge !== undefined;
        return (
          <Layout>
            <CourseNav />
            <QuizzesSubNav quizId={quizId} />
            <div className="max-w-4xl mx-auto">
              <Link to={`/courses/${courseId}/quizzes/${quizId}/submissions`} className="text-brand-600 hover:underline text-sm">
                &larr; All attempts
              </Link>
              <div className="bg-surface-0 rounded-lg shadow p-5 mt-2 mb-4">
                <h2 className="text-xl font-bold text-text-primary flex items-center gap-2">
                  <PenLine className="w-5 h-5" aria-hidden="true" /> {quiz.title}
                </h2>
                <p className="text-sm text-text-secondary mt-1">
                  {student} · attempt {submission.attempt} ·{' '}
                  <span className={submission.workflow_state === 'pending_review' ? 'text-accent-warning font-medium' : ''}>
                    {submission.workflow_state === 'pending_review' ? 'needs grading' : submission.workflow_state}
                  </span>
                  {submission.score_before_regrade != null && (
                    <> · score before regrade {submission.score_before_regrade}</>
                  )}
                </p>
                <div className="flex flex-wrap items-end gap-4 mt-4">
                  <div>
                    <label htmlFor="fudge-points" className="block text-xs font-medium text-text-secondary mb-1">Fudge points</label>
                    <input
                      id="fudge-points"
                      type="number"
                      step="0.25"
                      value={fudgeValue}
                      onChange={(e) => setFudge(e.target.value)}
                      className="w-32 border border-border-strong rounded px-3 py-2 text-sm bg-surface-0"
                    />
                  </div>
                  <p className="text-sm text-text-secondary">
                    Score: <span className="font-semibold text-text-primary">{preview}</span>
                    {quiz.points_possible ? ` / ${quiz.points_possible}` : ''}
                  </p>
                  <button
                    type="button"
                    onClick={onSave}
                    disabled={!dirty || save.isPending}
                    className="ml-auto inline-flex items-center gap-2 px-4 py-2 bg-brand-600 text-on-brand rounded hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
                  >
                    <Save className="w-4 h-4" aria-hidden="true" /> {save.isPending ? 'Saving…' : 'Save scores'}
                  </button>
                </div>
                {message && (
                  <p role="status" className={`mt-3 text-sm ${message.startsWith('Error') ? 'text-accent-danger' : 'text-accent-success'}`}>{message}</p>
                )}
              </div>

              <div className="space-y-4">
                {questions.map((q, i) => (
                  <QuestionGrader
                    key={q.id}
                    index={i}
                    question={q}
                    answer={byQ[q.id]}
                    edit={edits[q.id]}
                    onEdit={(patch) => setEdits((prev) => ({ ...prev, [q.id]: { ...prev[q.id], ...patch } }))}
                    fileUrl={api.getQuizAnswerFileUrl ? api.getQuizAnswerFileUrl(courseId, quizId, submissionId, q.id) : undefined}
                  />
                ))}
              </div>

              <section className="bg-surface-0 rounded-lg shadow p-5 mt-6" aria-labelledby="quiz-log-title">
                <h3 id="quiz-log-title" className="flex items-center gap-2 font-semibold text-text-primary mb-3">
                  <History className="w-4 h-4" aria-hidden="true" /> Attempt log
                </h3>
                <EventLog courseId={courseId} quizId={quizId} submissionId={submissionId} />
              </section>
            </div>
          </Layout>
        );
      }}
    </Page>
  );
}
