import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Plus, Trash2, GripVertical, Save, ChevronDown, ChevronUp, Check, Pencil, Layers, X, BookOpen } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import useUnsavedChanges from '../hooks/useUnsavedChanges';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { ALL_TYPES as QUESTION_TYPES, defaultAnswersForType, parseAnswers, stringifyAnswers } from '../components/quiz/itemTypes/types';
import MultipleAnswerEditor from '../components/quiz/itemTypes/MultipleAnswerEditor';
import MultipleDropdownEditor from '../components/quiz/itemTypes/MultipleDropdownEditor';
import FillInBlankEditor from '../components/quiz/itemTypes/FillInBlankEditor';
import FormulaEditor from '../components/quiz/itemTypes/FormulaEditor';
import FileUploadEditor from '../components/quiz/itemTypes/FileUploadEditor';
import OrderingEditor from '../components/quiz/itemTypes/OrderingEditor';
import CategorizationEditor from '../components/quiz/itemTypes/CategorizationEditor';
import HotSpotEditor from '../components/quiz/itemTypes/HotSpotEditor';
import TextOnlyEditor from '../components/quiz/itemTypes/TextOnlyEditor';

const emptyAnswer = () => ({
  id: `a${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
  text: '',
  weight: 0,
  comments: '',
});

const QuestionEditor = ({ question, index, onUpdate, onDelete, groups, courseId, stimuli = [] }) => {
  const [expanded, setExpanded] = useState(true);
  const [answers, setAnswers] = useState([]);

  useEffect(() => {
    try {
      const parsed = typeof question.answers === 'string'
        ? JSON.parse(question.answers || '[]')
        : (question.answers || []);
      setAnswers(parsed);
    } catch {
      setAnswers([]);
    }
  }, [question.id, question.question_type]);

  const updateField = (field, value) => {
    onUpdate({ ...question, [field]: value });
  };

  const updateAnswers = (newAnswers) => {
    setAnswers(newAnswers);
    onUpdate({ ...question, answers: JSON.stringify(newAnswers) });
  };

  const setCorrectAnswer = (answerIndex) => {
    const updated = answers.map((a, i) => ({
      ...a,
      weight: i === answerIndex ? 100 : 0,
    }));
    updateAnswers(updated);
  };

  const updateAnswerText = (answerIndex, text) => {
    const updated = answers.map((a, i) => i === answerIndex ? { ...a, text } : a);
    updateAnswers(updated);
  };

  const addAnswer = () => {
    updateAnswers([...answers, emptyAnswer()]);
  };

  const removeAnswer = (answerIndex) => {
    if (answers.length <= 2) return;
    updateAnswers(answers.filter((_, i) => i !== answerIndex));
  };

  const qType = question.question_type || 'multiple_choice';

  return (
    <div className="bg-surface-0 rounded-lg shadow border border-border-default">
      {/* Header */}
      <div
        className="flex items-center justify-between px-4 py-3 bg-surface-1 rounded-t-lg cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          <GripVertical className="w-4 h-4 text-text-disabled" />
          <span className="text-sm font-semibold text-text-secondary">
            Question {index + 1}
          </span>
          <span className="text-xs text-text-disabled">
            {QUESTION_TYPES.find(t => t.value === qType)?.label || qType}
          </span>
          <span className="text-xs text-text-disabled">
            ({question.points_possible ?? 1} pts)
          </span>
          {question.quiz_question_group_id && (() => {
            const grp = (groups || []).find(g => g.id === question.quiz_question_group_id);
            return grp ? (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-700">
                <Layers className="w-3 h-3" />
                {grp.name}
              </span>
            ) : null;
          })()}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            className="p-1 text-accent-danger hover:text-accent-danger"
            title="Delete question"
          >
            <Trash2 className="w-4 h-4" />
          </button>
          {expanded ? <ChevronUp className="w-4 h-4 text-text-disabled" /> : <ChevronDown className="w-4 h-4 text-text-disabled" />}
        </div>
      </div>

      {/* Body */}
      {expanded && (
        <div className="p-4 space-y-4">
          {/* Question Type, Points & Group */}
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Question Type</label>
              <select
                value={qType}
                onChange={(e) => {
                  updateField('question_type', e.target.value);
                  const newAnswers = defaultAnswersForType(e.target.value);
                  setAnswers(newAnswers);
                  onUpdate({
                    ...question,
                    question_type: e.target.value,
                    answers: JSON.stringify(newAnswers),
                  });
                }}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
              >
                {QUESTION_TYPES.map(t => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Points</label>
              <input
                type="number"
                min="0"
                step="0.5"
                value={question.points_possible ?? 1}
                onChange={(e) => updateField('points_possible', parseFloat(e.target.value) || 0)}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Group</label>
              <select
                value={question.quiz_question_group_id || ''}
                onChange={(e) => updateField('quiz_question_group_id', e.target.value ? Number(e.target.value) : null)}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
              >
                <option value="">None (ungrouped)</option>
                {(groups || []).map(g => (
                  <option key={g.id} value={g.id}>{g.name}</option>
                ))}
              </select>
            </div>
          </div>

          {/* Question Text */}
          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Question Text</label>
            <RichContentEditorV2
              value={question.question_text || ''}
              onChange={(html) => updateField('question_text', html)}
              placeholder="Enter the question..."
              minHeight="100px"
              courseId={courseId}
              autoSaveKey={`quiz-question-${courseId}-${question.id || `new-${index}`}-text`}
            />
          </div>

          {/* Linked stimulus (passage) selector */}
          {stimuli.length > 0 && (
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1 flex items-center gap-1">
                <BookOpen className="w-3.5 h-3.5" />
                Link to stimulus passage <span className="text-text-disabled font-normal">(optional)</span>
              </label>
              <select
                value={question.stimulus_id || ''}
                onChange={(e) => updateField('stimulus_id', e.target.value ? Number(e.target.value) : null)}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0 text-text-primary"
              >
                <option value="">None</option>
                {stimuli.map(s => (
                  <option key={s.id} value={s.id}>{s.title}</option>
                ))}
              </select>
            </div>
          )}

          {/* Answer Options (MC, TF) */}
          {(qType === 'multiple_choice' || qType === 'true_false') && (
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-2">
                Answers {qType === 'multiple_choice' && <span className="text-text-disabled">(click circle to mark correct)</span>}
              </label>
              <div className="space-y-2">
                {answers.map((answer, i) => (
                  <div key={answer.id || i} className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setCorrectAnswer(i)}
                      className={`w-6 h-6 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${
                        answer.weight > 0
                          ? 'border-accent-success bg-accent-success text-white'
                          : 'border-border-strong hover:border-accent-success/60'
                      }`}
                      title={answer.weight > 0 ? 'Correct answer' : 'Mark as correct'}
                    >
                      {answer.weight > 0 && <Check className="w-3 h-3" />}
                    </button>
                    <input
                      type="text"
                      value={answer.text}
                      onChange={(e) => updateAnswerText(i, e.target.value)}
                      className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm"
                      placeholder={`Answer ${i + 1}`}
                      disabled={qType === 'true_false'}
                    />
                    {qType === 'multiple_choice' && answers.length > 2 && (
                      <button
                        onClick={() => removeAnswer(i)}
                        className="p-1 text-text-disabled hover:text-accent-danger"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              {qType === 'multiple_choice' && (
                <button
                  onClick={addAnswer}
                  className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
                >
                  <Plus className="w-3 h-3" /> Add Answer
                </button>
              )}
            </div>
          )}

          {/* Short Answer */}
          {qType === 'short_answer' && (
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-2">
                Acceptable Answers <span className="text-text-disabled">(case-insensitive match)</span>
              </label>
              <div className="space-y-2">
                {answers.map((answer, i) => (
                  <div key={answer.id || i} className="flex items-center gap-2">
                    <input
                      type="text"
                      value={answer.text}
                      onChange={(e) => updateAnswerText(i, e.target.value)}
                      className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm"
                      placeholder="Acceptable answer..."
                    />
                    {answers.length > 1 && (
                      <button
                        onClick={() => removeAnswer(i)}
                        className="p-1 text-text-disabled hover:text-accent-danger"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <button
                onClick={addAnswer}
                className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
              >
                <Plus className="w-3 h-3" /> Add Acceptable Answer
              </button>
            </div>
          )}

          {/* Numerical */}
          {qType === 'numerical_question' && (
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">Expected Answer</label>
              <input
                type="number"
                step="any"
                value={answers[0]?.text || ''}
                onChange={(e) => updateAnswers([{ ...emptyAnswer(), text: e.target.value, weight: 100 }])}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                placeholder="Enter the expected numeric answer..."
              />
            </div>
          )}

          {/* Essay - no answers needed */}
          {qType === 'essay' && (
            <p className="text-xs text-text-tertiary italic">Essay questions require manual grading.</p>
          )}

          {/* Wave A new item types */}
          {qType === 'multiple_answer' && (
            <MultipleAnswerEditor answers={answers} onChange={updateAnswers} />
          )}
          {qType === 'multiple_dropdown' && (
            <MultipleDropdownEditor
              answers={answers}
              onChange={updateAnswers}
              questionText={question.question_text}
            />
          )}
          {qType === 'fill_in_the_blank' && (
            <FillInBlankEditor answers={answers} onChange={updateAnswers} />
          )}
          {qType === 'formula' && (
            <FormulaEditor answers={answers} onChange={updateAnswers} />
          )}
          {qType === 'file_upload' && <FileUploadEditor />}
          {qType === 'ordering' && (
            <OrderingEditor answers={answers} onChange={updateAnswers} />
          )}
          {qType === 'categorization' && (
            <CategorizationEditor answers={answers} onChange={updateAnswers} />
          )}
          {qType === 'hot_spot' && (
            <HotSpotEditor answers={answers} onChange={updateAnswers} courseId={courseId} />
          )}
          {qType === 'text_only' && <TextOnlyEditor />}

          {/* Feedback */}
          <details className="text-sm">
            <summary className="cursor-pointer text-text-tertiary hover:text-text-secondary text-xs">Feedback (optional)</summary>
            <div className="mt-2 grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="block text-xs text-text-tertiary mb-1">Correct feedback</label>
                <input
                  type="text"
                  value={question.correct_comments || ''}
                  onChange={(e) => updateField('correct_comments', e.target.value)}
                  className="w-full border border-border-strong rounded px-2 py-1 text-xs"
                  placeholder="Shown when correct..."
                />
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">Incorrect feedback</label>
                <input
                  type="text"
                  value={question.incorrect_comments || ''}
                  onChange={(e) => updateField('incorrect_comments', e.target.value)}
                  className="w-full border border-border-strong rounded px-2 py-1 text-xs"
                  placeholder="Shown when incorrect..."
                />
              </div>
              <div>
                <label className="block text-xs text-text-tertiary mb-1">Neutral feedback</label>
                <input
                  type="text"
                  value={question.neutral_comments || ''}
                  onChange={(e) => updateField('neutral_comments', e.target.value)}
                  className="w-full border border-border-strong rounded px-2 py-1 text-xs"
                  placeholder="Shown to everyone..."
                />
              </div>
            </div>
          </details>
        </div>
      )}
    </div>
  );
};

const QuizEditorPage = () => {
  const { courseId, quizId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [quiz, setQuiz] = useState(null);
  const [questions, setQuestions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(null);
  const [isDirty, setIsDirty] = useState(false);

  // Question groups
  const [groups, setGroups] = useState([]);
  const [questionBanks, setQuestionBanks] = useState([]);
  const [stimuli, setStimuli] = useState([]);
  const [showGroupForm, setShowGroupForm] = useState(false);
  const [editingGroup, setEditingGroup] = useState(null);
  const [groupForm, setGroupForm] = useState({ name: '', pick_count: 1, question_bank_id: '' });

  useUnsavedChanges(isDirty);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  // Quiz settings form
  const [quizForm, setQuizForm] = useState({
    title: '', description: '', quiz_type: 'assignment',
    time_limit: '', allowed_attempts: 1, points_possible: 0, published: false,
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [quizData, questionResult, groupsData, banksData, stimuliData] = await Promise.allSettled([
          api.getQuiz(courseId, quizId),
          api.getQuizQuestions(courseId, quizId, 1, 200),
          api.listQuizQuestionGroups(courseId, quizId),
          api.listQuestionBanks(courseId),
          api.listStimuli ? api.listStimuli(courseId) : Promise.resolve([]),
        ]);
        if (quizData.status === 'rejected') throw new Error(quizData.reason?.message || 'Failed to load quiz');
        if (questionResult.status === 'rejected') throw new Error(questionResult.reason?.message || 'Failed to load questions');
        const qd = quizData.value;
        setQuiz(qd);
        setQuizForm({
          title: qd.title || '',
          description: qd.description || '',
          quiz_type: qd.quiz_type || 'assignment',
          time_limit: qd.time_limit || '',
          allowed_attempts: qd.allowed_attempts || 1,
          points_possible: qd.points_possible ?? 0,
          published: qd.published || false,
        });
        const qr = questionResult.value;
        setQuestions(((qr.data || qr || []) instanceof Array ? (qr.data || qr) : []).sort((a, b) => (a.position || 0) - (b.position || 0)));
        if (groupsData.status === 'fulfilled') setGroups(groupsData.value || []);
        if (banksData.status === 'fulfilled') setQuestionBanks(banksData.value || []);
        if (stimuliData.status === 'fulfilled') setStimuli(stimuliData.value || []);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, quizId]);

  const doSaveQuiz = async () => {
    setSaving(true);
    setMessage('');
    try {
      const payload = {
        title: quizForm.title,
        description: quizForm.description,
        quiz_type: quizForm.quiz_type,
        time_limit: quizForm.time_limit ? Number(quizForm.time_limit) : null,
        allowed_attempts: Number(quizForm.allowed_attempts) || 1,
        points_possible: totalPoints,
        published: quizForm.published,
      };
      const updated = await api.updateQuiz(courseId, quizId, payload);
      setQuiz(updated);
      setIsDirty(false);
      setMessage('Quiz settings saved.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleSaveQuiz = () => {
    const allHtml = [quizForm.description, ...questions.map(q => q.question_text || '')].join('\n');
    checkAndSave(allHtml, doSaveQuiz);
  };

  const handleAddQuestion = async (type = 'multiple_choice') => {
    try {
      const answers = defaultAnswersForType(type);
      const label = QUESTION_TYPES.find(t => t.value === type)?.label || 'Question';
      const created = await api.createQuizQuestion(courseId, quizId, {
        position: questions.length + 1,
        question_type: type,
        question_text: `New ${label} Question`,
        points_possible: 1,
        answers: JSON.stringify(answers),
      });
      setQuestions(prev => [...prev, created]);
    } catch (err) {
      setMessage('Error adding question: ' + err.message);
    }
  };

  const updateTimerRef = useRef({});

  // Cleanup debounce timers on unmount
  useEffect(() => {
    return () => {
      Object.values(updateTimerRef.current).forEach(clearTimeout);
    };
  }, []);

  const handleUpdateQuestion = useCallback((updated) => {
    // Optimistic local state update immediately
    setQuestions(prev => prev.map(q => q.id === updated.id ? { ...q, ...updated } : q));

    // Debounce the API call (800ms)
    if (updateTimerRef.current[updated.id]) {
      clearTimeout(updateTimerRef.current[updated.id]);
    }
    updateTimerRef.current[updated.id] = setTimeout(async () => {
      try {
        const payload = {
          question_type: updated.question_type,
          question_text: updated.question_text,
          points_possible: updated.points_possible,
          answers: typeof updated.answers === 'string' ? updated.answers : JSON.stringify(updated.answers || []),
          correct_comments: updated.correct_comments || '',
          incorrect_comments: updated.incorrect_comments || '',
          neutral_comments: updated.neutral_comments || '',
          quiz_question_group_id: updated.quiz_question_group_id || null,
          stimulus_id: updated.stimulus_id || null,
        };
        await api.updateQuizQuestion(courseId, quizId, updated.id, payload);
      } catch (err) {
        setMessage('Error saving question: ' + err.message);
      }
    }, 800);
  }, [courseId, quizId]);

  const handleDeleteQuestion = async (questionId) => {
    if (!window.confirm('Delete this question? This cannot be undone.')) return;
    try {
      await api.deleteQuizQuestion(courseId, quizId, questionId);
      setQuestions(prev => prev.filter(q => q.id !== questionId));
    } catch (err) {
      setMessage('Error deleting question: ' + err.message);
    }
  };

  // --- Question Group Management ---
  const resetGroupForm = () => {
    setGroupForm({ name: '', pick_count: 1, question_bank_id: '' });
    setEditingGroup(null);
    setShowGroupForm(false);
  };

  const handleSaveGroup = async () => {
    if (!groupForm.name.trim()) {
      setMessage('Error: Group name is required.');
      return;
    }
    try {
      const payload = {
        name: groupForm.name.trim(),
        pick_count: Number(groupForm.pick_count) || 1,
      };
      if (groupForm.question_bank_id) {
        payload.question_bank_id = Number(groupForm.question_bank_id);
      }

      if (editingGroup) {
        const updated = await api.updateQuizQuestionGroup(courseId, quizId, editingGroup.id, payload);
        setGroups(prev => prev.map(g => g.id === editingGroup.id ? (updated || { ...editingGroup, ...payload }) : g));
        setMessage('Group updated.');
      } else {
        const created = await api.createQuizQuestionGroup(courseId, quizId, payload);
        setGroups(prev => [...prev, created]);
        setMessage('Group created.');
      }
      resetGroupForm();
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
  };

  const handleEditGroup = (group) => {
    setGroupForm({
      name: group.name || '',
      pick_count: group.pick_count ?? 1,
      question_bank_id: group.question_bank_id || '',
    });
    setEditingGroup(group);
    setShowGroupForm(true);
  };

  const handleDeleteGroup = async (groupId) => {
    if (!window.confirm('Delete this question group? Questions in this group will become ungrouped.')) return;
    try {
      await api.deleteQuizQuestionGroup(courseId, quizId, groupId);
      setGroups(prev => prev.filter(g => g.id !== groupId));
      // Ungroup questions that belonged to this group
      setQuestions(prev => prev.map(q =>
        q.quiz_question_group_id === groupId ? { ...q, quiz_question_group_id: null } : q
      ));
      setMessage('Group deleted.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
  };

  // Auto-calculate total points from questions
  const totalPoints = questions.reduce((sum, q) => sum + (q.points_possible ?? 1), 0);

  // Auto-sync quiz points_possible when questions change (skip initial load)
  const initialTotalRef = useRef(null);
  useEffect(() => {
    if (!quiz || questions.length === 0) return;
    if (initialTotalRef.current === null) {
      initialTotalRef.current = totalPoints;
      return;
    }
    if (quiz.points_possible !== totalPoints) {
      api.updateQuiz(courseId, quizId, { points_possible: totalPoints }).catch(() => {});
    }
  }, [totalPoints]);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;
  if (isTeacher === null || loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-accent-danger mb-3">{error}</p>
  <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
</div></Layout>;
  }

  return (
    <Layout>
      <CourseNav />
      <QuizzesSubNav quizId={quizId} />
      <div className="mb-6">
        <Link to={`/courses/${courseId}/quizzes`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Quizzes
        </Link>
        <h2 className="text-2xl font-bold text-text-primary mt-2">Edit Quiz</h2>
      </div>

      {message && (
        <div className={`mb-4 px-4 py-2 rounded text-sm ${message.startsWith('Error') ? 'bg-accent-danger/10 text-accent-danger' : 'bg-accent-success/10 text-accent-success'}`}>
          {message}
        </div>
      )}

      {/* Quiz Settings */}
      <section className="bg-surface-0 rounded-lg shadow p-6 mb-6">
        <h3 className="font-semibold text-lg mb-4">Quiz Settings</h3>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Title</label>
            <input
              type="text"
              value={quizForm.title}
              onChange={(e) => { setQuizForm(f => ({ ...f, title: e.target.value })); setIsDirty(true); }}
              className="w-full border border-border-strong rounded px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Description / Instructions</label>
            <RichContentEditorV2
              value={quizForm.description}
              onChange={(html) => { setQuizForm(f => ({ ...f, description: html })); setIsDirty(true); }}
              placeholder="Quiz instructions..."
              minHeight="120px"
              courseId={courseId}
              autoSaveKey={`quiz-${courseId}-${quizId || 'new'}-description`}
            />
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Type</label>
              <select
                value={quizForm.quiz_type}
                onChange={(e) => { setQuizForm(f => ({ ...f, quiz_type: e.target.value })); setIsDirty(true); }}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
              >
                <option value="assignment">Graded Quiz</option>
                <option value="practice_quiz">Practice Quiz</option>
                <option value="graded_survey">Graded Survey</option>
                <option value="survey">Ungraded Survey</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Time Limit (min)</label>
              <input
                type="number"
                min="0"
                value={quizForm.time_limit}
                onChange={(e) => { setQuizForm(f => ({ ...f, time_limit: e.target.value })); setIsDirty(true); }}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                placeholder="No limit"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Attempts</label>
              <input
                type="number"
                min="-1"
                value={quizForm.allowed_attempts}
                onChange={(e) => { setQuizForm(f => ({ ...f, allowed_attempts: e.target.value })); setIsDirty(true); }}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                title="-1 for unlimited"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Total Points</label>
              <div className="border border-border-default rounded px-3 py-2 text-sm bg-surface-1 text-text-secondary">
                {totalPoints} pts <span className="text-xs text-text-disabled">(from questions)</span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={quizForm.published}
                onChange={(e) => { setQuizForm(f => ({ ...f, published: e.target.checked })); setIsDirty(true); }}
              />
              Published
            </label>
            <button
              onClick={handleSaveQuiz}
              disabled={saving}
              className="inline-flex items-center gap-2 px-4 py-2 bg-brand-600 text-white rounded hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              {saving ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </div>
      </section>

      {/* Question Groups */}
      <section className="bg-surface-0 rounded-lg shadow p-6 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-lg flex items-center gap-2">
            <Layers className="w-5 h-5 text-purple-600" />
            Question Groups
            <span className="text-sm text-text-tertiary font-normal">({groups.length})</span>
          </h3>
          {!showGroupForm && (
            <button
              onClick={() => { resetGroupForm(); setShowGroupForm(true); }}
              className="inline-flex items-center gap-1 px-3 py-1.5 bg-purple-600 text-white rounded hover:bg-purple-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4" /> Add Question Group
            </button>
          )}
        </div>

        {groups.length === 0 && !showGroupForm && (
          <p className="text-sm text-text-tertiary italic">
            No question groups yet. Groups allow you to pick a random subset of questions for each student.
          </p>
        )}

        {/* Group Cards */}
        {groups.length > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 mb-4">
            {groups.map(group => {
              const groupQuestions = questions.filter(q => q.quiz_question_group_id === group.id);
              const bank = questionBanks.find(b => b.id === group.question_bank_id);
              return (
                <div key={group.id} className="border border-purple-200 rounded-lg p-4 bg-purple-50">
                  <div className="flex items-start justify-between mb-2">
                    <h4 className="font-medium text-text-primary text-sm">{group.name}</h4>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleEditGroup(group)}
                        className="p-1 text-text-disabled hover:text-purple-600"
                        title="Edit group"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDeleteGroup(group.id)}
                        className="p-1 text-text-disabled hover:text-accent-danger"
                        title="Delete group"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                  <div className="space-y-1 text-xs text-text-secondary">
                    <div>
                      Pick <span className="font-semibold text-purple-700">{group.pick_count}</span> question{group.pick_count !== 1 ? 's' : ''} randomly
                    </div>
                    <div>
                      {groupQuestions.length} question{groupQuestions.length !== 1 ? 's' : ''} in group
                    </div>
                    {bank && (
                      <div className="flex items-center gap-1">
                        <span className="text-text-disabled">Bank:</span>
                        <span className="text-purple-600 font-medium">{bank.title}</span>
                      </div>
                    )}
                    {group.question_bank_id && !bank && (
                      <div className="text-text-disabled">
                        Bank ID: {group.question_bank_id}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Group Form */}
        {showGroupForm && (
          <div className="border border-purple-300 rounded-lg p-4 bg-purple-50">
            <h4 className="font-medium text-sm text-text-primary mb-3">
              {editingGroup ? 'Edit Question Group' : 'New Question Group'}
            </h4>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">Group Name *</label>
                <input
                  type="text"
                  value={groupForm.name}
                  onChange={(e) => setGroupForm(f => ({ ...f, name: e.target.value }))}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                  placeholder="e.g., Random Pool A"
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">Pick Count</label>
                <input
                  type="number"
                  min="1"
                  value={groupForm.pick_count}
                  onChange={(e) => setGroupForm(f => ({ ...f, pick_count: e.target.value }))}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                />
                <p className="text-xs text-text-disabled mt-0.5">Questions picked per student</p>
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">Question Bank (optional)</label>
                <select
                  value={groupForm.question_bank_id}
                  onChange={(e) => setGroupForm(f => ({ ...f, question_bank_id: e.target.value }))}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                >
                  <option value="">No bank</option>
                  {questionBanks.map(b => (
                    <option key={b.id} value={b.id}>{b.title}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="flex items-center gap-2 mt-3">
              <button
                onClick={handleSaveGroup}
                className="inline-flex items-center gap-1 px-3 py-1.5 bg-purple-600 text-white rounded hover:bg-purple-700 text-sm font-medium"
              >
                <Check className="w-4 h-4" /> {editingGroup ? 'Update Group' : 'Create Group'}
              </button>
              <button
                onClick={resetGroupForm}
                className="inline-flex items-center gap-1 px-3 py-1.5 bg-border-default text-text-secondary rounded hover:bg-border-strong text-sm font-medium"
              >
                <X className="w-4 h-4" /> Cancel
              </button>
            </div>
          </div>
        )}
      </section>

      {/* Questions */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-lg">
            Questions <span className="text-sm text-text-tertiary font-normal">({questions.length} question{questions.length !== 1 ? 's' : ''}, {totalPoints} pts)</span>
          </h3>
          <div className="flex items-center gap-2">
            <select
              id="add-question-type"
              defaultValue="multiple_choice"
              className="border border-border-strong rounded px-2 py-1.5 text-sm"
            >
              {QUESTION_TYPES.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
            <button
              onClick={() => {
                const typeSelect = document.getElementById('add-question-type');
                handleAddQuestion(typeSelect?.value || 'multiple_choice');
              }}
              className="inline-flex items-center gap-1 px-3 py-1.5 bg-accent-success text-white rounded hover:bg-accent-success/90 text-sm font-medium"
            >
              <Plus className="w-4 h-4" /> Add Question
            </button>
          </div>
        </div>

        {questions.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">
            No questions yet. Click "Add Question" to get started.
          </div>
        ) : (
          <div className="space-y-4">
            {questions.map((q, i) => (
              <QuestionEditor
                key={q.id}
                question={q}
                index={i}
                onUpdate={handleUpdateQuestion}
                onDelete={() => handleDeleteQuestion(q.id)}
                groups={groups}
                courseId={courseId}
                stimuli={stimuli}
              />
            ))}
          </div>
        )}

        {questions.length > 0 && (
          <div className="mt-4 flex justify-center">
            <button
              onClick={() => {
                const typeSelect = document.getElementById('add-question-type');
                handleAddQuestion(typeSelect?.value || 'multiple_choice');
              }}
              className="inline-flex items-center gap-1 px-4 py-2 bg-accent-success text-white rounded hover:bg-accent-success/90 text-sm font-medium"
            >
              <Plus className="w-4 h-4" /> Add Question
            </button>
          </div>
        )}
      </section>
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default QuizEditorPage;
