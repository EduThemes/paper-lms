import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { BookOpen, Plus, Trash2, Edit2, ChevronDown, ChevronRight, HelpCircle, Download } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const QUESTION_TYPES = [
  { value: 'multiple_choice', label: 'Multiple Choice' },
  { value: 'true_false', label: 'True/False' },
  { value: 'short_answer', label: 'Short Answer' },
  { value: 'essay', label: 'Essay' },
  { value: 'numerical_question', label: 'Numerical' },
];

const formatQuestionType = (type) => {
  const found = QUESTION_TYPES.find((t) => t.value === type);
  return found ? found.label : type;
};

const LoadingSpinner = ({ size = 'h-8 w-8', message }) => (
  <div className="flex items-center justify-center py-12 gap-3 text-text-tertiary">
    <svg className={`animate-spin ${size} text-brand-600`} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
    {message && <span>{message}</span>}
  </div>
);

const emptyQuestion = () => ({
  question_name: '',
  question_type: 'multiple_choice',
  question_text: '',
  points_possible: 1,
  answers: [
    { text: '', weight: 100 },
    { text: '', weight: 0 },
  ],
  feedback: '',
});

// ---------------------------------------------------------------------------
// Pull-to-Quiz Modal
// ---------------------------------------------------------------------------
const PullToQuizModal = ({ courseId, bankId, questions, onClose, onSuccess }) => {
  const [quizzes, setQuizzes] = useState([]);
  const [loadingQuizzes, setLoadingQuizzes] = useState(true);
  const [selectedQuizId, setSelectedQuizId] = useState('');
  const [selectedQuestionIds, setSelectedQuestionIds] = useState(() =>
    questions.map((q) => q.id)
  );
  const [pulling, setPulling] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const result = await api.getQuizzes(courseId, 1, 100);
        if (!cancelled) setQuizzes(result.data || []);
      } catch (err) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoadingQuizzes(false);
      }
    })();
    return () => { cancelled = true; };
  }, [courseId]);

  const toggleQuestion = (qId) => {
    setSelectedQuestionIds((prev) =>
      prev.includes(qId) ? prev.filter((id) => id !== qId) : [...prev, qId]
    );
  };

  const toggleAll = () => {
    if (selectedQuestionIds.length === questions.length) {
      setSelectedQuestionIds([]);
    } else {
      setSelectedQuestionIds(questions.map((q) => q.id));
    }
  };

  const handlePull = async () => {
    if (!selectedQuizId || selectedQuestionIds.length === 0) return;
    setPulling(true);
    setError(null);
    try {
      await api.pullBankQuestionsToQuiz(courseId, bankId, selectedQuizId, selectedQuestionIds);
      onSuccess(selectedQuestionIds.length);
    } catch (err) {
      setError(err.message);
    } finally {
      setPulling(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div
        className="bg-surface-0 rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between p-4 border-b">
          <h3 className="text-lg font-semibold">Pull Questions to Quiz</h3>
          <button onClick={onClose} className="text-text-disabled hover:text-text-secondary text-xl leading-none">&times;</button>
        </div>

        <div className="p-4 overflow-y-auto flex-1 space-y-4">
          {error && (
            <div className="bg-accent-danger/10 text-accent-danger p-3 rounded text-sm">{error}</div>
          )}

          {/* Quiz selector */}
          <div>
            <label htmlFor="pull-quiz-select" className="block text-sm font-medium text-text-secondary mb-1">
              Destination Quiz
            </label>
            {loadingQuizzes ? (
              <div className="flex items-center gap-2 text-sm text-text-tertiary">
                <svg className="animate-spin h-4 w-4 text-brand-600" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
                </svg>
                Loading quizzes...
              </div>
            ) : quizzes.length === 0 ? (
              <p className="text-sm text-text-tertiary">No quizzes found. Create a quiz first.</p>
            ) : (
              <select
                id="pull-quiz-select"
                value={selectedQuizId}
                onChange={(e) => setSelectedQuizId(e.target.value)}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              >
                <option value="">Select a quiz...</option>
                {quizzes.map((q) => (
                  <option key={q.id} value={q.id}>{q.title}</option>
                ))}
              </select>
            )}
          </div>

          {/* Question checkboxes */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-text-secondary">
                Questions ({selectedQuestionIds.length}/{questions.length} selected)
              </span>
              <button
                type="button"
                onClick={toggleAll}
                className="text-xs text-brand-600 hover:text-brand-800"
              >
                {selectedQuestionIds.length === questions.length ? 'Deselect All' : 'Select All'}
              </button>
            </div>
            <div className="border border-border-default rounded-md max-h-48 overflow-y-auto divide-y">
              {questions.map((q) => (
                <label key={q.id} className="flex items-center gap-3 px-3 py-2 hover:bg-surface-1 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={selectedQuestionIds.includes(q.id)}
                    onChange={() => toggleQuestion(q.id)}
                    className="rounded border-border-strong text-brand-600 focus:ring-brand-500"
                  />
                  <div className="min-w-0 flex-1">
                    <span className="text-sm text-text-primary truncate block">
                      {q.question_name || 'Untitled Question'}
                    </span>
                    <span className="text-xs text-text-tertiary">
                      {formatQuestionType(q.question_type)} &middot; {q.points_possible ?? 0} pts
                    </span>
                  </div>
                </label>
              ))}
            </div>
          </div>
        </div>

        <div className="flex justify-end gap-3 p-4 border-t">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handlePull}
            disabled={pulling || !selectedQuizId || selectedQuestionIds.length === 0}
            className="inline-flex items-center px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
          >
            {pulling ? (
              <>
                <svg className="animate-spin h-4 w-4 me-2" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
                </svg>
                Pulling...
              </>
            ) : (
              <>
                <Download className="w-4 h-4 me-1" />
                Pull {selectedQuestionIds.length} Question{selectedQuestionIds.length !== 1 ? 's' : ''}
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Question Form
// ---------------------------------------------------------------------------
const QuestionForm = ({ initial, onSave, onCancel, saving }) => {
  const [form, setForm] = useState(() => initial || emptyQuestion());

  const update = (field, value) => setForm((prev) => ({ ...prev, [field]: value }));

  const updateAnswer = (idx, field, value) => {
    setForm((prev) => {
      const answers = [...(prev.answers || [])];
      answers[idx] = { ...answers[idx], [field]: value };
      return { ...prev, answers };
    });
  };

  const addAnswer = () => {
    setForm((prev) => ({
      ...prev,
      answers: [...(prev.answers || []), { text: '', weight: 0 }],
    }));
  };

  const removeAnswer = (idx) => {
    setForm((prev) => ({
      ...prev,
      answers: (prev.answers || []).filter((_, i) => i !== idx),
    }));
  };

  const markCorrect = (idx) => {
    setForm((prev) => {
      const answers = (prev.answers || []).map((a, i) => ({
        ...a,
        weight: i === idx ? 100 : 0,
      }));
      return { ...prev, answers };
    });
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave({
      ...form,
      points_possible: Number(form.points_possible) || 0,
      answers: form.answers || [],
    });
  };

  const showAnswers = form.question_type === 'multiple_choice' || form.question_type === 'true_false';

  return (
    <form onSubmit={handleSubmit} className="space-y-4 bg-surface-1 rounded-md p-4 border border-border-default">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label htmlFor="q-name" className="block text-sm font-medium text-text-secondary mb-1">Question Name</label>
          <input
            id="q-name"
            type="text"
            required
            value={form.question_name}
            onChange={(e) => update('question_name', e.target.value)}
            className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
            placeholder="e.g. Question 1"
          />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label htmlFor="q-type" className="block text-sm font-medium text-text-secondary mb-1">Type</label>
            <select
              id="q-type"
              value={form.question_type}
              onChange={(e) => {
                const newType = e.target.value;
                update('question_type', newType);
                // Reset answers for true_false
                if (newType === 'true_false') {
                  setForm((prev) => ({
                    ...prev,
                    question_type: newType,
                    answers: [
                      { text: 'True', weight: 100 },
                      { text: 'False', weight: 0 },
                    ],
                  }));
                }
              }}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
            >
              {QUESTION_TYPES.map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="q-points" className="block text-sm font-medium text-text-secondary mb-1">Points</label>
            <input
              id="q-points"
              type="number"
              min="0"
              step="0.5"
              value={form.points_possible}
              onChange={(e) => update('points_possible', e.target.value)}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
            />
          </div>
        </div>
      </div>

      <div>
        <label htmlFor="q-text" className="block text-sm font-medium text-text-secondary mb-1">Question Text</label>
        <textarea
          id="q-text"
          required
          rows={3}
          value={form.question_text}
          onChange={(e) => update('question_text', e.target.value)}
          className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
          placeholder="Enter the question text..."
        />
      </div>

      {/* Answers section for MC / TF */}
      {showAnswers && (
        <div>
          <span className="block text-sm font-medium text-text-secondary mb-2">
            Answers {form.question_type === 'true_false' ? '' : '(click radio to mark correct)'}
          </span>
          <div className="space-y-2">
            {(form.answers || []).map((ans, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <input
                  type="radio"
                  name="correct-answer"
                  checked={ans.weight === 100}
                  onChange={() => markCorrect(idx)}
                  className="text-brand-600 focus:ring-brand-500"
                  title="Mark as correct answer"
                  disabled={form.question_type === 'true_false'}
                />
                <input
                  type="text"
                  value={ans.text}
                  onChange={(e) => updateAnswer(idx, 'text', e.target.value)}
                  className="flex-1 border border-border-strong rounded-md px-3 py-1.5 text-sm"
                  placeholder={`Answer ${idx + 1}`}
                  required
                  readOnly={form.question_type === 'true_false'}
                />
                {form.question_type !== 'true_false' && (form.answers || []).length > 2 && (
                  <button
                    type="button"
                    onClick={() => removeAnswer(idx)}
                    className="text-text-disabled hover:text-accent-danger"
                    title="Remove answer"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
          {form.question_type === 'multiple_choice' && (
            <button
              type="button"
              onClick={addAnswer}
              className="mt-2 text-sm text-brand-600 hover:text-brand-800"
            >
              + Add Answer
            </button>
          )}
        </div>
      )}

      <div>
        <label htmlFor="q-feedback" className="block text-sm font-medium text-text-secondary mb-1">Feedback (optional)</label>
        <textarea
          id="q-feedback"
          rows={2}
          value={form.feedback || ''}
          onChange={(e) => update('feedback', e.target.value)}
          className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
          placeholder="Shown to student after grading..."
        />
      </div>

      <div className="flex justify-end gap-3">
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          {saving ? 'Saving...' : 'Save Question'}
        </button>
      </div>
    </form>
  );
};

// ---------------------------------------------------------------------------
// Bank Card (expandable)
// ---------------------------------------------------------------------------
const BankCard = ({ bank, courseId, isTeacher, onDelete, onUpdate }) => {
  const [expanded, setExpanded] = useState(false);
  const [questions, setQuestions] = useState([]);
  const [loadingQuestions, setLoadingQuestions] = useState(false);
  const [questionsError, setQuestionsError] = useState(null);
  const [showAddQuestion, setShowAddQuestion] = useState(false);
  const [editingQuestionId, setEditingQuestionId] = useState(null);
  const [savingQuestion, setSavingQuestion] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(bank.title);
  const [showPullModal, setShowPullModal] = useState(false);
  const [pullSuccess, setPullSuccess] = useState(null);

  const fetchQuestions = useCallback(async () => {
    setLoadingQuestions(true);
    setQuestionsError(null);
    try {
      const data = await api.listBankQuestions(courseId, bank.id);
      setQuestions(data || []);
    } catch (err) {
      setQuestionsError(err.message);
    } finally {
      setLoadingQuestions(false);
    }
  }, [courseId, bank.id]);

  const handleExpand = () => {
    const willExpand = !expanded;
    setExpanded(willExpand);
    if (willExpand && questions.length === 0 && !loadingQuestions) {
      fetchQuestions();
    }
  };

  const handleSaveTitle = async () => {
    const trimmed = titleDraft.trim();
    if (!trimmed || trimmed === bank.title) {
      setEditingTitle(false);
      setTitleDraft(bank.title);
      return;
    }
    try {
      await api.updateQuestionBank(courseId, bank.id, trimmed);
      onUpdate({ ...bank, title: trimmed });
      setEditingTitle(false);
    } catch (err) {
      setQuestionsError(err.message);
    }
  };

  const handleAddQuestion = async (question) => {
    setSavingQuestion(true);
    try {
      const created = await api.addBankQuestion(courseId, bank.id, question);
      setQuestions((prev) => [...prev, created]);
      setShowAddQuestion(false);
    } catch (err) {
      setQuestionsError(err.message);
    } finally {
      setSavingQuestion(false);
    }
  };

  const handleUpdateQuestion = async (question) => {
    setSavingQuestion(true);
    try {
      const updated = await api.updateBankQuestion(courseId, bank.id, editingQuestionId, question);
      setQuestions((prev) => prev.map((q) => (q.id === editingQuestionId ? updated : q)));
      setEditingQuestionId(null);
    } catch (err) {
      setQuestionsError(err.message);
    } finally {
      setSavingQuestion(false);
    }
  };

  const handleDeleteQuestion = async (questionId) => {
    if (!window.confirm('Delete this question?')) return;
    try {
      await api.deleteBankQuestion(courseId, bank.id, questionId);
      setQuestions((prev) => prev.filter((q) => q.id !== questionId));
    } catch (err) {
      setQuestionsError(err.message);
    }
  };

  const handlePullSuccess = (count) => {
    setShowPullModal(false);
    setPullSuccess(count);
    setTimeout(() => setPullSuccess(null), 4000);
  };

  const totalPoints = questions.reduce((sum, q) => sum + (Number(q.points_possible) || 0), 0);

  return (
    <>
      <div className="bg-surface-0 rounded-lg shadow">
        {/* Bank header */}
        <div
          className="flex items-center justify-between p-4 cursor-pointer hover:bg-surface-1"
          onClick={handleExpand}
          role="button"
          aria-expanded={expanded}
          tabIndex={0}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleExpand(); } }}
        >
          <div className="flex items-center gap-3 min-w-0 flex-1">
            {expanded ? (
              <ChevronDown className="w-5 h-5 text-text-disabled flex-shrink-0" />
            ) : (
              <ChevronRight className="w-5 h-5 text-text-disabled flex-shrink-0" />
            )}
            <BookOpen className="w-5 h-5 text-brand-500 flex-shrink-0" />
            {editingTitle && isTeacher ? (
              <input
                type="text"
                value={titleDraft}
                onChange={(e) => setTitleDraft(e.target.value)}
                onBlur={handleSaveTitle}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleSaveTitle();
                  if (e.key === 'Escape') { setEditingTitle(false); setTitleDraft(bank.title); }
                }}
                onClick={(e) => e.stopPropagation()}
                className="border border-brand-300 rounded px-2 py-1 text-sm font-medium focus:outline-none focus:ring-2 focus:ring-brand-500"
                autoFocus
              />
            ) : (
              <div className="min-w-0">
                <span className="font-medium text-text-primary truncate block">{bank.title}</span>
                <span className="text-xs text-text-tertiary">
                  {questions.length > 0
                    ? `${questions.length} question${questions.length !== 1 ? 's' : ''} \u00b7 ${totalPoints} pts`
                    : expanded && !loadingQuestions ? '0 questions' : ''}
                </span>
              </div>
            )}
          </div>

          <div className="flex items-center gap-2 flex-shrink-0 ms-3" onClick={(e) => e.stopPropagation()}>
            {pullSuccess && (
              <span className="text-xs text-accent-success bg-accent-success/10 px-2 py-1 rounded-full whitespace-nowrap">
                Pulled {pullSuccess} question{pullSuccess !== 1 ? 's' : ''}
              </span>
            )}
            {isTeacher && expanded && questions.length > 0 && (
              <button
                onClick={() => setShowPullModal(true)}
                className="inline-flex items-center text-xs text-brand-600 hover:text-brand-800 px-2 py-1 rounded hover:bg-brand-50"
                title="Pull questions to a quiz"
              >
                <Download className="w-3.5 h-3.5 me-1" />
                Pull to Quiz
              </button>
            )}
            {isTeacher && (
              <>
                <button
                  onClick={() => { setEditingTitle(true); setTitleDraft(bank.title); }}
                  className="text-text-disabled hover:text-brand-600 p-1"
                  title="Edit bank title"
                >
                  <Edit2 className="w-4 h-4" />
                </button>
                <button
                  onClick={() => onDelete(bank.id)}
                  className="text-text-disabled hover:text-accent-danger p-1"
                  title="Delete question bank"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </>
            )}
          </div>
        </div>

        {/* Expanded content */}
        {expanded && (
          <div className="border-t px-4 pb-4">
            {questionsError && (
              <div className="bg-accent-danger/10 text-accent-danger p-3 rounded text-sm mt-3 flex items-center justify-between">
                <span>{questionsError}</span>
                <button
                  onClick={fetchQuestions}
                  className="text-accent-danger hover:text-accent-danger text-xs font-medium ms-3 whitespace-nowrap"
                >
                  Try Again
                </button>
              </div>
            )}

            {loadingQuestions ? (
              <div className="flex items-center justify-center py-6 gap-2 text-text-tertiary">
                <svg className="animate-spin h-5 w-5 text-brand-600" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
                </svg>
                Loading questions...
              </div>
            ) : (
              <>
                {questions.length === 0 && !questionsError && (
                  <div className="text-center text-text-tertiary text-sm py-6">
                    <HelpCircle className="w-8 h-8 mx-auto mb-2 text-text-disabled" />
                    No questions in this bank yet.
                  </div>
                )}

                {/* Question list */}
                {questions.length > 0 && (
                  <div className="divide-y mt-3">
                    {questions.map((q) =>
                      editingQuestionId === q.id && isTeacher ? (
                        <div key={q.id} className="py-3">
                          <QuestionForm
                            initial={{
                              question_name: q.question_name || '',
                              question_type: q.question_type || 'multiple_choice',
                              question_text: q.question_text || '',
                              points_possible: q.points_possible ?? 1,
                              answers: q.answers || [],
                              feedback: q.feedback || '',
                            }}
                            onSave={handleUpdateQuestion}
                            onCancel={() => setEditingQuestionId(null)}
                            saving={savingQuestion}
                          />
                        </div>
                      ) : (
                        <div key={q.id} className="py-3 flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="font-medium text-sm text-text-primary">
                                {q.question_name || 'Untitled Question'}
                              </span>
                              <span className="inline-flex items-center text-xs bg-surface-2 text-text-secondary px-2 py-0.5 rounded-full">
                                {formatQuestionType(q.question_type)}
                              </span>
                              <span className="text-xs text-text-tertiary">
                                {q.points_possible ?? 0} pt{(q.points_possible ?? 0) !== 1 ? 's' : ''}
                              </span>
                            </div>
                            <p className="text-sm text-text-secondary mt-1 line-clamp-2">{q.question_text}</p>
                            {/* Show answers for MC / TF in read-only view */}
                            {(q.question_type === 'multiple_choice' || q.question_type === 'true_false') &&
                              Array.isArray(q.answers) && q.answers.length > 0 && (
                                <div className="mt-2 space-y-1">
                                  {q.answers.map((a, aIdx) => (
                                    <div key={aIdx} className="flex items-center gap-1.5 text-xs">
                                      <span className={`w-3 h-3 rounded-full border ${a.weight === 100 ? 'bg-accent-success border-accent-success' : 'border-border-strong'}`} />
                                      <span className={a.weight === 100 ? 'text-accent-success font-medium' : 'text-text-secondary'}>
                                        {a.text}
                                      </span>
                                    </div>
                                  ))}
                                </div>
                              )}
                            {q.feedback && (
                              <p className="text-xs text-text-disabled mt-1 italic">Feedback: {q.feedback}</p>
                            )}
                          </div>
                          {isTeacher && (
                            <div className="flex items-center gap-1 flex-shrink-0 pt-0.5">
                              <button
                                onClick={() => setEditingQuestionId(q.id)}
                                className="text-text-disabled hover:text-brand-600 p-1"
                                title="Edit question"
                              >
                                <Edit2 className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() => handleDeleteQuestion(q.id)}
                                className="text-text-disabled hover:text-accent-danger p-1"
                                title="Delete question"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          )}
                        </div>
                      )
                    )}
                  </div>
                )}

                {/* Add question form / button */}
                {isTeacher && (
                  <div className="mt-4">
                    {showAddQuestion ? (
                      <QuestionForm
                        onSave={handleAddQuestion}
                        onCancel={() => setShowAddQuestion(false)}
                        saving={savingQuestion}
                      />
                    ) : (
                      <button
                        onClick={() => setShowAddQuestion(true)}
                        className="inline-flex items-center text-sm text-brand-600 hover:text-brand-800 font-medium"
                      >
                        <Plus className="w-4 h-4 me-1" />
                        Add Question
                      </button>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </div>

      {/* Pull-to-Quiz modal */}
      {showPullModal && (
        <PullToQuizModal
          courseId={courseId}
          bankId={bank.id}
          questions={questions}
          onClose={() => setShowPullModal(false)}
          onSuccess={handlePullSuccess}
        />
      )}
    </>
  );
};

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------
const QuestionBanksPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [banks, setBanks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [creating, setCreating] = useState(false);

  const fetchBanks = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const data = await api.listQuestionBanks(courseId);
      setBanks(data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    fetchBanks();
  }, [fetchBanks]);

  const handleCreate = async (e) => {
    e.preventDefault();
    const trimmed = newTitle.trim();
    if (!trimmed) return;
    setCreating(true);
    try {
      const created = await api.createQuestionBank(courseId, trimmed);
      setBanks((prev) => [...prev, created]);
      setNewTitle('');
      setShowCreate(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteBank = async (bankId) => {
    if (!window.confirm('Delete this question bank and all its questions?')) return;
    try {
      await api.deleteQuestionBank(courseId, bankId);
      setBanks((prev) => prev.filter((b) => b.id !== bankId));
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdateBank = (updated) => {
    setBanks((prev) => prev.map((b) => (b.id === updated.id ? updated : b)));
  };

  // Wait for role detection
  if (isTeacher === null) {
    return (
      <Layout>
        <LoadingSpinner message="Loading..." />
      </Layout>
    );
  }

  if (loading) {
    return (
      <Layout>
        <LoadingSpinner message="Loading question banks..." />
      </Layout>
    );
  }

  if (error && banks.length === 0) {
    return (
      <Layout>
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button
            onClick={fetchBanks}
            className="text-brand-600 hover:text-brand-800 text-sm font-medium"
          >
            Try Again
          </button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Question Banks</h2>
          {isTeacher && (
            <button
              onClick={() => setShowCreate(!showCreate)}
              className="inline-flex items-center px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4 me-1" />
              New Bank
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4 flex items-center justify-between">
          <span className="text-sm">{error}</span>
          <button onClick={() => setError(null)} className="text-accent-danger hover:text-accent-danger text-xs font-medium ms-3">
            Dismiss
          </button>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-3">Create Question Bank</h3>
          <div className="flex gap-3">
            <input
              type="text"
              required
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm"
              placeholder="Bank title"
              autoFocus
            />
            <button
              type="submit"
              disabled={creating}
              className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>
            <button
              type="button"
              onClick={() => { setShowCreate(false); setNewTitle(''); }}
              className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {banks.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">
          <BookOpen className="w-12 h-12 mx-auto mb-3 text-text-disabled" />
          <p className="text-lg font-medium text-text-secondary mb-1">No question banks yet</p>
          <p className="text-sm">
            {isTeacher
              ? 'Create a question bank to organize and reuse questions across quizzes.'
              : 'No question banks have been created for this course.'}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {banks.map((bank) => (
            <BankCard
              key={bank.id}
              bank={bank}
              courseId={courseId}
              isTeacher={isTeacher}
              onDelete={handleDeleteBank}
              onUpdate={handleUpdateBank}
            />
          ))}
        </div>
      )}
    </Layout>
  );
};

export default QuestionBanksPage;
