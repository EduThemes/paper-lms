import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useParams, useSearchParams, Link } from 'react-router-dom';
import {
  CheckCircle,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  FileText,
  RotateCcw,
  Focus,
  Keyboard,
  List,
  LayoutGrid,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';
import RichContentViewer, { sanitizeHTML } from '../components/RichContentViewer';
import { useLiveRegion } from '../components/LiveRegion';
import QuizTimer from '../components/quiz/QuizTimer';
import QuestionPalette from '../components/quiz/QuestionPalette';
import AutoSaveIndicator from '../components/quiz/AutoSaveIndicator';
import RestoreAnswersDialog from '../components/quiz/RestoreAnswersDialog';
import ShortcutsDialog from '../components/quiz/ShortcutsDialog';
import ItemPlayer from '../components/quiz/itemTypes/ItemPlayer';

// Question types that are essentially answer-set-driven and use the legacy
// inline render path. Newer types delegate to ItemPlayer.
const LEGACY_INLINE_TYPES = new Set([
  'multiple_choice',
  'true_false',
  'short_answer',
  'essay',
  'numerical_question',
]);

const lsKey = (submissionId) => `paper.quiz.${submissionId}`;

const QuizTakePage = () => {
  const { t } = useTranslation();
  const { courseId, quizId } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const focusMode = searchParams.get('focus') === '1';
  const { announce } = useLiveRegion();

  const [quiz, setQuiz] = useState(null);
  const [submission, setSubmission] = useState(null);
  const [questions, setQuestions] = useState([]);
  const [answers, setAnswers] = useState({});
  const [currentIdx, setCurrentIdx] = useState(0);
  const [loading, setLoading] = useState(true);
  const [starting, setStarting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const [timeLeft, setTimeLeft] = useState(null);
  const [completed, setCompleted] = useState(false);
  const [started, setStarted] = useState(false);
  const [previousAttempts, setPreviousAttempts] = useState(0);
  const [lastSubmission, setLastSubmission] = useState(null);

  const [autoSaveStatus, setAutoSaveStatus] = useState('idle');
  const [lastSavedAt, setLastSavedAt] = useState(null);
  const [retryPayload, setRetryPayload] = useState(null);

  const [allOnePage, setAllOnePage] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [restorePrompt, setRestorePrompt] = useState(null);

  const lsDebounceRef = useRef(null);
  const savedTimeoutRef = useRef(null);
  const submissionRef = useRef(null);
  useEffect(() => { submissionRef.current = submission; }, [submission]);

  useEffect(() => {
    const init = async () => {
      try {
        const quizData = await api.getQuiz(courseId, quizId);
        setQuiz(quizData);
        try {
          const subResult = await api.getQuizSubmissions(courseId, quizId);
          const subs = subResult.data || [];
          if (subs.length > 0) {
            subs.sort((a, b) => (b.attempt || 0) - (a.attempt || 0));
            const latest = subs[0];
            setPreviousAttempts(subs.filter(s => s.workflow_state === 'complete').length);
            setLastSubmission(latest);
            if (latest.workflow_state === 'untaken' || latest.workflow_state === 'pending') {
              setSubmission(latest);
              if (latest.end_at) {
                const endAt = new Date(latest.end_at);
                setTimeLeft(Math.max(0, Math.floor((endAt - new Date()) / 1000)));
              }
              const { data: qs } = await api.getQuizQuestions(courseId, quizId, 1, 100);
              setQuestions(qs);
              setStarted(true);
              checkLocalBackup(latest);
            } else if (latest.workflow_state === 'complete') {
              const allowed = quizData.allowed_attempts || -1;
              if (allowed !== -1 && subs.filter(s => s.workflow_state === 'complete').length >= allowed) {
                setCompleted(true);
                setSubmission(latest);
              }
            }
          }
        } catch {
          // No existing submissions
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    init();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId, quizId]);

  const checkLocalBackup = (sub) => {
    try {
      const raw = localStorage.getItem(lsKey(sub.id));
      if (!raw) return;
      const parsed = JSON.parse(raw);
      const localTs = parsed?.savedAt ? new Date(parsed.savedAt) : null;
      const serverTs = sub.updated_at ? new Date(sub.updated_at) : null;
      const isNewer = !serverTs || (localTs && localTs > serverTs);
      const count = Object.keys(parsed?.answers || {}).length;
      if (isNewer && count > 0) {
        setRestorePrompt({ answers: parsed.answers, savedAt: localTs, count });
      }
    } catch {
      // corrupted entry — ignore
    }
  };

  const handleBeginQuiz = async () => {
    setStarting(true);
    setError(null);
    try {
      const sub = await api.startQuizSubmission(courseId, quizId);
      setSubmission(sub);
      if (sub.workflow_state === 'complete') {
        setCompleted(true);
        setStarting(false);
        return;
      }
      if (sub.end_at) {
        const endAt = new Date(sub.end_at);
        setTimeLeft(Math.max(0, Math.floor((endAt - new Date()) / 1000)));
      }
      const { data: qs } = await api.getQuizQuestions(courseId, quizId, 1, 100);
      setQuestions(qs);
      setStarted(true);
      checkLocalBackup(sub);
    } catch (err) {
      setError(err.message);
    } finally {
      setStarting(false);
    }
  };

  const persistLocal = useCallback((nextAnswers) => {
    const sub = submissionRef.current;
    if (!sub) return;
    if (lsDebounceRef.current) clearTimeout(lsDebounceRef.current);
    lsDebounceRef.current = setTimeout(() => {
      try {
        localStorage.setItem(
          lsKey(sub.id),
          JSON.stringify({ answers: nextAnswers, savedAt: new Date().toISOString() })
        );
      } catch {
        // localStorage may be full or unavailable
      }
    }, 500);
  }, []);

  const saveAnswerToServer = useCallback(async (questionId, answer) => {
    const sub = submissionRef.current;
    if (!sub) return;
    setAutoSaveStatus('saving');
    try {
      await api.answerQuizQuestion(courseId, quizId, sub.id, questionId, answer);
      const now = new Date();
      setLastSavedAt(now);
      setAutoSaveStatus('saved');
      announce('Answer saved');
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
      savedTimeoutRef.current = setTimeout(() => setAutoSaveStatus('idle'), 2000);
      setRetryPayload(null);
    } catch (err) {
      console.error('Failed to save answer:', err);
      setAutoSaveStatus('error');
      setRetryPayload({ questionId, answer });
      announce('Save failed', 'assertive');
    }
  }, [courseId, quizId, announce]);

  const handleAnswer = useCallback((questionId, answer) => {
    setAnswers(prev => {
      const next = { ...prev, [questionId]: answer };
      persistLocal(next);
      return next;
    });
    saveAnswerToServer(questionId, answer);
  }, [persistLocal, saveAnswerToServer]);

  const handleRetrySave = useCallback(() => {
    if (!retryPayload) return;
    saveAnswerToServer(retryPayload.questionId, retryPayload.answer);
  }, [retryPayload, saveAnswerToServer]);

  const handleSubmit = useCallback(async () => {
    if (submitting || !submissionRef.current) return;
    setSubmitting(true);
    try {
      const result = await api.completeQuizSubmission(courseId, quizId, submissionRef.current.id);
      setSubmission(result);
      setCompleted(true);
      try { localStorage.removeItem(lsKey(submissionRef.current.id)); } catch { /* noop */ }
      announce('Quiz submitted', 'assertive');
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }, [courseId, quizId, submitting, announce]);

  useEffect(() => {
    if (timeLeft === null || timeLeft <= 0 || completed) return;
    const interval = setInterval(() => {
      setTimeLeft(prev => {
        if (prev <= 1) {
          clearInterval(interval);
          handleSubmit();
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [timeLeft, completed, handleSubmit]);

  const goNext = useCallback(() => {
    setCurrentIdx(i => Math.min(questions.length - 1, i + 1));
  }, [questions.length]);
  const goPrev = useCallback(() => {
    setCurrentIdx(i => Math.max(0, i - 1));
  }, []);

  // Keyboard shortcuts: only when quiz is active and not typing.
  useEffect(() => {
    if (!started || completed) return;
    const isTypingTarget = (el) => {
      if (!el) return false;
      const tag = el.tagName;
      return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
    };
    const handler = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (isTypingTarget(e.target)) return;
      if (e.key === 'j') { e.preventDefault(); goNext(); }
      else if (e.key === 'k') { e.preventDefault(); goPrev(); }
      else if (e.key === '?') { e.preventDefault(); setShortcutsOpen(true); }
      else if (/^[1-9]$/.test(e.key)) {
        const target = parseInt(e.key, 10) - 1;
        if (target < questions.length) {
          e.preventDefault();
          setCurrentIdx(target);
        }
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [started, completed, goNext, goPrev, questions.length]);

  useEffect(() => () => {
    if (lsDebounceRef.current) clearTimeout(lsDebounceRef.current);
    if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
  }, []);

  const toggleFocusMode = () => {
    const next = new URLSearchParams(searchParams);
    if (focusMode) next.delete('focus'); else next.set('focus', '1');
    setSearchParams(next, { replace: true });
  };

  const handleRestoreAnswers = async () => {
    if (!restorePrompt) return;
    const restored = restorePrompt.answers;
    setAnswers(restored);
    setRestorePrompt(null);
    announce(`Restored ${Object.keys(restored).length} answers`);
    for (const [qid, val] of Object.entries(restored)) {
      // Replay answers to server (sequential to avoid hammering the API)
      // eslint-disable-next-line no-await-in-loop
      await saveAnswerToServer(qid, val);
    }
  };
  const handleDiscardRestore = () => {
    if (restorePrompt && submissionRef.current) {
      try { localStorage.removeItem(lsKey(submissionRef.current.id)); } catch { /* noop */ }
    }
    setRestorePrompt(null);
  };

  const renderQuestion = (q, idx) => {
    const stimulus = q.stimulus || null;
    // multiple_dropdown renders its own inline question text with embedded
    // <select>s, so suppress the default stem rendering for it.
    const suppressStem = q.question_type === 'multiple_dropdown';
    return (
      <div
        key={q.id}
        className="bg-surface-0 rounded-lg shadow-sm border border-border-default p-6"
        aria-labelledby={`q-${q.id}-label`}
      >
        <div className="flex items-center justify-between mb-3">
          <span id={`q-${q.id}-label`} className="text-sm text-text-tertiary font-medium">
            Question {idx + 1} of {questions.length}
          </span>
          {q.points_possible != null && (
            <span className="text-xs text-text-tertiary">{q.points_possible} pts</span>
          )}
        </div>

        {stimulus && (
          <details
            open
            className="mb-4 border-l-4 border-brand-500 bg-surface-1 rounded-r"
          >
            <summary className="cursor-pointer px-3 py-2 text-xs font-medium text-text-secondary">
              Passage: {stimulus.title || 'Reading'}
            </summary>
            <div className="px-3 pb-3 prose prose-sm max-w-prose text-text-primary"
                 dangerouslySetInnerHTML={{ __html: sanitizeHTML(stimulus.content || '') }} />
          </details>
        )}

        {!suppressStem && (
          <div
            className="prose prose-sm max-w-prose mb-5 text-text-primary"
            dangerouslySetInnerHTML={{ __html: sanitizeHTML(q.question_text) }}
          />
        )}

        <ItemPlayer
          question={q}
          value={answers[q.id]}
          onChange={(v) => handleAnswer(q.id, v)}
        />
      </div>
    );
  };

  const answeredCount = useMemo(
    () => questions.filter(q => answers[q.id] !== undefined && answers[q.id] !== '').length,
    [questions, answers]
  );

  const wrap = (content) => (focusMode ? (
    <div className="min-h-screen bg-surface-1">{content}</div>
  ) : (
    <Layout>{content}</Layout>
  ));

  if (loading) {
    return wrap(
      <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
        <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
        {t('quizTakePage.loadingQuiz')}
      </div>
    );
  }
  if (error && !quiz) {
    return wrap(
      <div className="text-center py-12">
        <p className="text-accent-danger mb-3">{error}</p>
        <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">{t('common.tryAgain')}</button>
      </div>
    );
  }

  if (completed) {
    return wrap(
      <div className="max-w-2xl mx-auto">
        <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
          <CheckCircle className="w-16 h-16 text-accent-success mx-auto mb-4" />
          <h2 className="text-2xl font-bold mb-2">{t('quizTakePage.quizComplete')}</h2>
          {submission?.score !== null && submission?.score !== undefined && (
            <p className="text-lg text-text-secondary mb-4">
              Score: <span className="font-semibold">{submission.score}</span>
              {quiz?.points_possible && ` / ${quiz.points_possible}`}
            </p>
          )}
          {submission?.workflow_state === 'pending_review' && (
            <p className="text-text-tertiary mb-4">Some questions require manual grading.</p>
          )}
          <div className="flex items-center justify-center gap-4">
            {submission?.id && (
              <Link
                to={`/courses/${courseId}/quizzes/${quizId}/submissions/${submission.id}/review`}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
              >
                Review Answers
              </Link>
            )}
            <Link
              to={`/courses/${courseId}`}
              className="text-brand-600 hover:underline text-sm"
            >
              Back to Course
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (!started) {
    const allowedAttempts = quiz?.allowed_attempts || -1;
    const attemptsRemaining = allowedAttempts === -1 ? 'Unlimited' : Math.max(0, allowedAttempts - previousAttempts);
    const canStart = allowedAttempts === -1 || previousAttempts < allowedAttempts;
    return wrap(
      <div className="max-w-2xl mx-auto">
        <div className="mb-4">
          <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
            &larr; Back to Course
          </Link>
        </div>
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <div className="bg-brand-600 px-6 py-5">
            <h2 className="text-2xl font-bold text-white">{quiz?.title}</h2>
            {quiz?.quiz_type && (
              <span className="text-brand-100 text-sm capitalize">{quiz.quiz_type.replace('_', ' ')}</span>
            )}
          </div>
          <div className="p-6">
            {quiz?.description && (
              <RichContentViewer content={quiz.description} className="mb-6" />
            )}
            <div className="grid grid-cols-2 gap-4 mb-6">
              <div className="bg-surface-1 rounded-lg p-4">
                <div className="text-xs text-text-tertiary uppercase tracking-wide font-medium">Points</div>
                <div className="text-lg font-semibold text-text-primary mt-1">{quiz?.points_possible ?? 0}</div>
              </div>
              <div className="bg-surface-1 rounded-lg p-4">
                <div className="text-xs text-text-tertiary uppercase tracking-wide font-medium">Time Limit</div>
                <div className="text-lg font-semibold text-text-primary mt-1">
                  {quiz?.time_limit ? `${quiz.time_limit} minutes` : 'None'}
                </div>
              </div>
              <div className="bg-surface-1 rounded-lg p-4">
                <div className="text-xs text-text-tertiary uppercase tracking-wide font-medium">Questions</div>
                <div className="text-lg font-semibold text-text-primary mt-1">{quiz?.question_count || '—'}</div>
              </div>
              <div className="bg-surface-1 rounded-lg p-4">
                <div className="text-xs text-text-tertiary uppercase tracking-wide font-medium">Attempts</div>
                <div className="text-lg font-semibold text-text-primary mt-1">
                  {previousAttempts > 0 && (
                    <span className="text-sm text-text-tertiary font-normal mr-1">{previousAttempts} used /</span>
                  )}
                  {allowedAttempts === -1 ? 'Unlimited' : `${attemptsRemaining} remaining`}
                </div>
              </div>
            </div>
            {lastSubmission && lastSubmission.workflow_state === 'complete' && (
              <div className="bg-accent-success/10 border border-accent-success/30 rounded-lg p-4 mb-6 flex items-center justify-between">
                <div>
                  <div className="text-sm font-medium text-accent-success">Previous Attempt</div>
                  <div className="text-xs text-accent-success mt-0.5">
                    Score: {lastSubmission.score !== null && lastSubmission.score !== undefined
                      ? `${lastSubmission.score}/${quiz?.points_possible ?? 0}`
                      : 'Pending review'}
                  </div>
                </div>
                {lastSubmission.id && (
                  <Link
                    to={`/courses/${courseId}/quizzes/${quizId}/submissions/${lastSubmission.id}/review`}
                    className="text-sm text-accent-success hover:underline font-medium"
                  >
                    Review Answers
                  </Link>
                )}
              </div>
            )}
            {error && (
              <div className="bg-accent-danger/10 border border-accent-danger/30 rounded-lg p-3 mb-4 text-sm text-accent-danger flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                {error}
              </div>
            )}
            {canStart ? (
              <div className="text-center">
                {quiz?.time_limit && (
                  <p className="text-sm text-accent-warning mb-3 flex items-center justify-center gap-1">
                    <AlertCircle className="w-4 h-4" />
                    Once you begin, the {quiz.time_limit}-minute timer will start and cannot be paused.
                  </p>
                )}
                <div className="flex items-center justify-center gap-3 flex-wrap">
                  <button
                    onClick={handleBeginQuiz}
                    disabled={starting}
                    className="inline-flex items-center gap-2 bg-brand-600 text-white px-8 py-3 rounded-lg hover:bg-brand-700 disabled:opacity-50 text-lg font-semibold transition-colors"
                  >
                    {starting ? <>Starting...</> : previousAttempts > 0 ? <><RotateCcw className="w-5 h-5" /> Retake Quiz</> : <><FileText className="w-5 h-5" /> Begin Quiz</>}
                  </button>
                  <button
                    onClick={toggleFocusMode}
                    className="inline-flex items-center gap-2 px-4 py-3 rounded-lg border border-border-strong bg-surface-0 hover:bg-surface-1 text-sm font-medium text-text-secondary"
                    aria-pressed={focusMode}
                  >
                    <Focus className="w-4 h-4" />
                    {focusMode ? 'Focus on' : 'Focus mode'}
                  </button>
                </div>
              </div>
            ) : (
              <div className="text-center text-text-tertiary">
                <p className="font-medium">No attempts remaining.</p>
                <p className="text-sm mt-1">You have used all {allowedAttempts} allowed attempt{allowedAttempts !== 1 ? 's' : ''}.</p>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  const currentQuestion = questions[currentIdx];

  return wrap(
    <>
      {/* Sticky header: timer + autosave + progress */}
      <div className="sticky top-0 z-20 bg-surface-0/95 backdrop-blur border-b border-border-default -mx-4 sm:-mx-6 px-4 sm:px-6 mb-4">
        <div className="max-w-5xl mx-auto py-2 flex items-center justify-between gap-3 flex-wrap">
          <div className="flex items-center gap-3 min-w-0">
            <h1 className="text-sm font-semibold text-text-primary truncate">{quiz?.title}</h1>
            <span className="text-xs text-text-tertiary whitespace-nowrap">
              {answeredCount}/{questions.length} answered
            </span>
          </div>
          <div className="flex items-center gap-3">
            <AutoSaveIndicator
              status={autoSaveStatus}
              lastSavedAt={lastSavedAt}
              onRetry={handleRetrySave}
            />
            <QuizTimer timeLeft={timeLeft} />
            <button
              type="button"
              onClick={() => setAllOnePage(v => !v)}
              className="hidden sm:inline-flex items-center gap-1 text-xs px-2 py-1 rounded border border-border-strong hover:bg-surface-1"
              aria-pressed={allOnePage}
              title={allOnePage ? 'Single question view' : 'All questions on one page'}
            >
              {allOnePage ? <List className="w-3.5 h-3.5" /> : <LayoutGrid className="w-3.5 h-3.5" />}
              {allOnePage ? 'Single' : 'All'}
            </button>
            <button
              type="button"
              onClick={toggleFocusMode}
              aria-pressed={focusMode}
              className="inline-flex items-center gap-1 text-xs px-2 py-1 rounded border border-border-strong hover:bg-surface-1"
              title="Toggle focus mode"
            >
              <Focus className="w-3.5 h-3.5" />
              Focus
            </button>
            <button
              type="button"
              onClick={() => setShortcutsOpen(true)}
              className="inline-flex items-center gap-1 text-xs px-2 py-1 rounded border border-border-strong hover:bg-surface-1"
              aria-label="Show keyboard shortcuts"
            >
              <Keyboard className="w-3.5 h-3.5" />
              <kbd className="font-mono">?</kbd>
            </button>
          </div>
        </div>
      </div>

      <div className="max-w-5xl mx-auto md:flex md:items-start md:gap-6">
        <div className="flex-1 min-w-0">
          {allOnePage ? (
            <div className="space-y-4">
              {questions.map((q, idx) => renderQuestion(q, idx))}
            </div>
          ) : (
            currentQuestion && renderQuestion(currentQuestion, currentIdx)
          )}

          {!allOnePage && (
            <div className="flex items-center justify-between mt-4">
              <button
                onClick={goPrev}
                disabled={currentIdx === 0}
                className="flex items-center gap-1 px-4 py-2 bg-surface-2 rounded hover:bg-border-default disabled:opacity-50 text-sm font-medium"
              >
                <ChevronLeft className="w-4 h-4" />
                {t('quizTakePage.previous')}
              </button>
              {currentIdx < questions.length - 1 ? (
                <button
                  onClick={goNext}
                  className="flex items-center gap-1 px-4 py-2 bg-surface-2 rounded hover:bg-border-default text-sm font-medium"
                >
                  {t('quizTakePage.next')}
                  <ChevronRight className="w-4 h-4" />
                </button>
              ) : (
                <button
                  onClick={handleSubmit}
                  disabled={submitting}
                  className="px-6 py-2 bg-brand-600 text-white rounded hover:bg-brand-700 disabled:opacity-50 text-sm font-semibold"
                >
                  {submitting ? t('quizTakePage.submitting') : t('quizTakePage.submitQuiz')}
                </button>
              )}
            </div>
          )}

          {allOnePage && (
            <div className="flex justify-end mt-6">
              <button
                onClick={handleSubmit}
                disabled={submitting}
                className="px-6 py-2 bg-brand-600 text-white rounded hover:bg-brand-700 disabled:opacity-50 text-sm font-semibold"
              >
                {submitting ? 'Submitting...' : 'Submit Quiz'}
              </button>
            </div>
          )}
        </div>

        {!allOnePage && (
          <QuestionPalette
            questions={questions}
            currentIdx={currentIdx}
            answers={answers}
            onJump={setCurrentIdx}
          />
        )}
      </div>

      <RestoreAnswersDialog
        open={Boolean(restorePrompt)}
        count={restorePrompt?.count || 0}
        savedAt={restorePrompt?.savedAt || null}
        onRestore={handleRestoreAnswers}
        onDiscard={handleDiscardRestore}
      />
      <ShortcutsDialog open={shortcutsOpen} onOpenChange={setShortcutsOpen} />
    </>
  );
};

export default QuizTakePage;
