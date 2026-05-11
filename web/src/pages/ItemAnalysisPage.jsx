import React, { useState, useEffect, useMemo } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { BarChart3, AlertCircle, FileQuestion } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import { TYPE_LABELS, parseAnswers } from '../components/quiz/itemTypes/types';

/**
 * Item Analysis — per-question metrics across all student attempts.
 *
 * Strategy: prefer the Wave A backend endpoint
 * (GET /quizzes/:id/item-analysis); on failure or when it isn't deployed
 * yet, fall back to aggregating client-side from submissions + answers.
 * Both paths feed the same render code.
 */
const ItemAnalysisPage = () => {
  const { courseId, quizId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [analysis, setAnalysis] = useState(null);
  const [quiz, setQuiz] = useState(null);
  const [questions, setQuestions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [stubMode, setStubMode] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const [quizData, questionResult] = await Promise.all([
          api.getQuiz(courseId, quizId),
          api.getQuizQuestions(courseId, quizId, 1, 200),
        ]);
        if (cancelled) return;
        setQuiz(quizData);
        const qs = questionResult.data || [];
        setQuestions(qs);

        let stats = null;
        try {
          stats = await api.getQuizItemAnalysis(courseId, quizId);
        } catch {
          stats = null;
        }

        if (!stats) {
          // Client-side aggregation fallback.
          stats = await aggregateClientSide(courseId, quizId, qs);
        }
        if (!cancelled) setAnalysis(stats);
      } catch (err) {
        if (!cancelled) {
          setError(err.message);
          setStubMode(true);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => { cancelled = true; };
  }, [courseId, quizId]);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  return (
    <Layout>
      <CourseNav />
      <QuizzesSubNav quizId={quizId} />

      <header className="mb-6">
        <Link to={`/courses/${courseId}/quizzes/${quizId}/edit`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Quiz
        </Link>
        <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2 mt-2">
          <BarChart3 className="w-6 h-6 text-brand-600" />
          Item Analysis
        </h1>
        {quiz && <p className="text-sm text-text-tertiary mt-1">{quiz.title}</p>}
      </header>

      {error && (
        <div className="mb-4 px-4 py-2 rounded text-sm bg-accent-warning/10 text-accent-warning flex items-center gap-2">
          <AlertCircle className="w-4 h-4" /> {error}
        </div>
      )}

      {loading ? (
        <div className="p-6 text-center text-text-tertiary text-sm">Loading…</div>
      ) : stubMode || !analysis || analysis.questions?.length === 0 ? (
        <StubPlaceholder questions={questions} />
      ) : (
        <ItemAnalysisTable analysis={analysis} questions={questions} />
      )}
    </Layout>
  );
};

const ItemAnalysisTable = ({ analysis, questions }) => {
  const byId = useMemo(() => Object.fromEntries(questions.map(q => [String(q.id), q])), [questions]);

  return (
    <div className="space-y-4">
      {analysis.questions.map((qStat) => {
        const q = byId[String(qStat.question_id)];
        if (!q) return null;
        const pctCorrect = Math.round((qStat.pct_correct ?? 0) * 100);
        const totalAttempts = qStat.attempts ?? 0;
        const opts = parseAnswers(q.answers, []);
        return (
          <section key={q.id} className="bg-surface-0 rounded-lg shadow border border-border-default p-5">
            <header className="flex items-start justify-between mb-3 gap-3">
              <div className="min-w-0">
                <div className="text-xs text-text-tertiary uppercase tracking-wide">
                  {TYPE_LABELS[q.question_type] || q.question_type}
                </div>
                <div className="text-sm text-text-primary mt-1 line-clamp-2"
                     dangerouslySetInnerHTML={{ __html: String(q.question_text || '').slice(0, 240) }} />
              </div>
              <div className="text-right text-xs text-text-tertiary whitespace-nowrap">
                <div>{totalAttempts} attempts</div>
                <div className="mt-0.5">avg {(qStat.avg_points ?? 0).toFixed(2)} / {q.points_possible ?? 1} pts</div>
                {qStat.pending_review > 0 && (
                  <div className="text-accent-warning mt-0.5">{qStat.pending_review} pending review</div>
                )}
              </div>
            </header>

            <div className="flex items-center gap-3 mb-3">
              <div className="flex-1 h-3 bg-surface-1 rounded overflow-hidden">
                <div className={`h-full ${pctCorrect >= 70 ? 'bg-accent-success' : pctCorrect >= 40 ? 'bg-accent-warning' : 'bg-accent-danger'}`}
                     style={{ width: `${Math.max(0, Math.min(100, pctCorrect))}%` }} />
              </div>
              <span className="text-sm font-semibold text-text-primary w-12 text-right">{pctCorrect}%</span>
            </div>

            {/* Per-option distribution (for MC/MA/dropdown shapes) */}
            {qStat.option_counts && Object.keys(qStat.option_counts).length > 0 && (
              <div className="mt-3 space-y-1.5">
                {opts.map(opt => {
                  const count = qStat.option_counts[opt.id] || 0;
                  const pct = totalAttempts > 0 ? Math.round((count / totalAttempts) * 100) : 0;
                  const correct = opt.weight > 0;
                  return (
                    <div key={opt.id} className="flex items-center gap-2 text-xs">
                      <span className={`flex-1 truncate ${correct ? 'font-semibold text-accent-success' : 'text-text-secondary'}`}>
                        {opt.text || opt.blank_id || '(option)'}
                      </span>
                      <div className="w-32 h-2 bg-surface-1 rounded overflow-hidden">
                        <div className={`h-full ${correct ? 'bg-accent-success/70' : 'bg-text-tertiary/50'}`}
                             style={{ width: `${pct}%` }} />
                      </div>
                      <span className="w-16 text-right text-text-tertiary tabular-nums">{count} ({pct}%)</span>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        );
      })}
    </div>
  );
};

const StubPlaceholder = ({ questions }) => (
  <div className="bg-surface-0 rounded-lg shadow border border-border-default p-8 text-center">
    <FileQuestion className="w-10 h-10 mx-auto text-text-disabled mb-3" />
    <h2 className="text-lg font-semibold text-text-primary mb-1">No analysis data yet</h2>
    <p className="text-sm text-text-tertiary max-w-md mx-auto">
      Detailed item analysis appears here once students have submitted attempts. Aggregations include
      % correct, per-option distribution (MC/MA/dropdown), average points earned, and items still
      pending manual grading.
    </p>
    {questions.length > 0 && (
      <p className="text-xs text-text-disabled mt-3">
        {questions.length} question{questions.length !== 1 ? 's' : ''} configured on this quiz.
      </p>
    )}
  </div>
);

// Aggregates per-question metrics directly from submissions when the backend
// item-analysis endpoint isn't available.
async function aggregateClientSide(courseId, quizId, questions) {
  let submissions = [];
  try {
    const result = await api.getQuizSubmissions(courseId, quizId, 1, 500);
    submissions = (result.data || []).filter(s => s.workflow_state === 'complete' || s.workflow_state === 'pending_review');
  } catch {
    return { questions: [] };
  }

  const allAnswers = await Promise.all(
    submissions.map(s => api.getQuizSubmissionAnswers(courseId, quizId, s.id).catch(() => []))
  );

  return {
    questions: questions.map(q => {
      const opts = parseAnswers(q.answers, []);
      const correctIds = new Set(opts.filter(o => o.weight > 0).map(o => o.id));
      const optionCounts = {};
      let attempts = 0;
      let correct = 0;
      let pointsSum = 0;
      let pendingReview = 0;

      allAnswers.forEach((subAnswers) => {
        const ans = subAnswers.find(a => String(a.question_id) === String(q.id));
        if (!ans) return;
        attempts += 1;
        pointsSum += Number(ans.points ?? 0);
        if (ans.workflow_state === 'pending_review' || ans.correct === null) {
          pendingReview += 1;
        }
        if (ans.correct === true) correct += 1;

        // Tally option selections for MC / MA / dropdown shapes.
        const v = ans.answer;
        if (typeof v === 'string') {
          optionCounts[v] = (optionCounts[v] || 0) + 1;
        } else if (Array.isArray(v)) {
          v.forEach(id => { optionCounts[id] = (optionCounts[id] || 0) + 1; });
        } else if (v && typeof v === 'object') {
          Object.values(v).forEach(id => {
            if (typeof id === 'string') optionCounts[id] = (optionCounts[id] || 0) + 1;
          });
        }
      });

      return {
        question_id: q.id,
        attempts,
        pct_correct: attempts > 0 ? correct / attempts : 0,
        avg_points: attempts > 0 ? pointsSum / attempts : 0,
        option_counts: optionCounts,
        pending_review: pendingReview,
        _correctIds: Array.from(correctIds),
      };
    }),
  };
}

export default ItemAnalysisPage;
