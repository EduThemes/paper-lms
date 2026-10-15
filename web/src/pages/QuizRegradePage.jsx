import React, { useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { RefreshCcw } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import Page from '../components/Page';
import SafeHTML from '../components/SafeHTML';
import useIsTeacher from '../hooks/useIsTeacher';
import { useQuizRegradeData, useRegradeQuizQuestion } from '../services/api_quiz_engine';
import { REGRADE_OPTIONS } from '../components/quiz/quizLifecycle';

const optionLabel = (value) => REGRADE_OPTIONS.find((o) => o.value === value)?.label || value;

function RegradeRow({ index, question, onRegrade, busy }) {
  const [option, setOption] = useState('current_correct_only');
  return (
    <li className="bg-surface-0 rounded-lg shadow p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-xs font-medium text-text-tertiary">
            Question {index + 1} · {question.question_type?.replace(/_/g, ' ')} · {question.points_possible ?? 1} pts
          </p>
          <SafeHTML className="prose max-w-none text-sm text-text-primary mt-1" html={question.question_text} />
        </div>
      </div>
      <div className="flex flex-wrap items-end gap-3 mt-3">
        <div className="flex-1 min-w-[16rem]">
          <label htmlFor={`regrade-${question.id}`} className="block text-xs font-medium text-text-secondary mb-1">How should existing attempts be regraded?</label>
          <select
            id={`regrade-${question.id}`}
            value={option}
            onChange={(e) => setOption(e.target.value)}
            className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0"
          >
            {REGRADE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <button
          type="button"
          disabled={busy}
          onClick={() => onRegrade(question, option)}
          className="inline-flex items-center gap-2 px-3 py-2 bg-brand-600 text-on-brand rounded hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          <RefreshCcw className="w-4 h-4" aria-hidden="true" /> Regrade
        </button>
      </div>
    </li>
  );
}

/**
 * Regrade (G053): after changing a question's answer key in the editor,
 * rescore every finished attempt — only the new answer, award both, full
 * credit, or keep scores — and see the history. Instructor only.
 */
export default function QuizRegradePage() {
  const { courseId, quizId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const query = useQuizRegradeData(courseId, quizId);
  const regrade = useRegradeQuizQuestion(courseId, quizId);
  const [message, setMessage] = useState('');

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  const onRegrade = async (question, option) => {
    setMessage('');
    if (!window.confirm(`Regrade question "${(question.question_text || '').replace(/<[^>]+>/g, '').slice(0, 60)}"?\n\n${optionLabel(option)}`)) return;
    try {
      const rec = await regrade.mutateAsync({ questionId: question.id, regradeOption: option });
      setMessage(`Regraded ${rec.submissions_regraded} attempt(s); ${rec.submissions_changed} score(s) changed.`);
    } catch (err) {
      setMessage(`Error: ${err.message}`);
    }
  };

  return (
    <Page query={query} title="Regrade quiz">
      {({ quiz, questions, regrades }) => {
        const indexOf = {};
        questions.forEach((q, i) => { indexOf[q.id] = i + 1; });
        return (
          <Layout>
            <CourseNav />
            <QuizzesSubNav quizId={quizId} />
            <div className="max-w-4xl mx-auto">
              <Link to={`/courses/${courseId}/quizzes/${quizId}/edit`} className="text-brand-600 hover:underline text-sm">&larr; Edit quiz</Link>
              <h2 className="text-2xl font-bold text-text-primary mt-2 mb-1">Regrade: {quiz.title}</h2>
              <p className="text-sm text-text-tertiary mb-4">
                Fix the answer key in the quiz editor first, then regrade the question here. Grades in the gradebook update with it.
              </p>
              {message && (
                <p role="status" className={`mb-3 text-sm ${message.startsWith('Error') ? 'text-accent-danger' : 'text-accent-success'}`}>{message}</p>
              )}
              {questions.length === 0 ? (
                <p className="text-sm text-text-tertiary">This quiz has no questions.</p>
              ) : (
                <ol className="space-y-3">
                  {questions.map((q, i) => (
                    <RegradeRow key={q.id} index={i} question={q} onRegrade={onRegrade} busy={regrade.isPending} />
                  ))}
                </ol>
              )}

              <section className="mt-8" aria-labelledby="regrade-history">
                <h3 id="regrade-history" className="font-semibold text-text-primary mb-2">Regrade history</h3>
                {regrades.length === 0 ? (
                  <p className="text-sm text-text-tertiary">No regrades yet.</p>
                ) : (
                  <div className="bg-surface-0 rounded-lg shadow overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b text-left text-text-tertiary">
                          <th scope="col" className="px-4 py-2">When</th>
                          <th scope="col" className="px-4 py-2">Question</th>
                          <th scope="col" className="px-4 py-2">Option</th>
                          <th scope="col" className="px-4 py-2">Attempts regraded</th>
                          <th scope="col" className="px-4 py-2">Scores changed</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y">
                        {regrades.map((r) => (
                          <tr key={r.id}>
                            <td className="px-4 py-2 whitespace-nowrap">{new Date(r.created_at).toLocaleString()}</td>
                            <td className="px-4 py-2">{indexOf[r.quiz_question_id] ? `Question ${indexOf[r.quiz_question_id]}` : `#${r.quiz_question_id}`}</td>
                            <td className="px-4 py-2">{optionLabel(r.regrade_option)}</td>
                            <td className="px-4 py-2">{r.submissions_regraded}</td>
                            <td className="px-4 py-2">{r.submissions_changed}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </div>
          </Layout>
        );
      }}
    </Page>
  );
}
