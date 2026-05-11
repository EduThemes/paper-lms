import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ClipboardList, Eye, EyeOff, Plus, CheckCircle, Clock, AlertCircle, BarChart3 } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { Skeleton } from '@/components/ui/skeleton';

const QuizzesPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [quizzes, setQuizzes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newQuiz, setNewQuiz] = useState({ title: '', description: '', quiz_type: 'assignment', points_possible: 0, time_limit: '' });
  const [mySubmissions, setMySubmissions] = useState({}); // quizId -> submission
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  const fetchQuizzes = async () => {
    setError(null);
    setLoading(true);
    try {
      const result = await api.getQuizzes(courseId);
      setQuizzes(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchQuizzes();
  }, [courseId]);

  // For students, fetch quiz submissions to show status
  useEffect(() => {
    if (isTeacher !== false || quizzes.length === 0) return;
    const fetchSubmissions = async () => {
      const subMap = {};
      const subPromises = quizzes.map(async (quiz) => {
        try {
          const subResult = await api.getQuizSubmissions(courseId, quiz.id, 1, 5);
          const subs = subResult.data || [];
          const mySub = subs
            .filter(s => s.user_id === user?.id)
            .sort((a, b) => (b.attempt || 0) - (a.attempt || 0))[0];
          if (mySub) subMap[quiz.id] = mySub;
        } catch { /* ignore */ }
      });
      await Promise.allSettled(subPromises);
      setMySubmissions(subMap);
    };
    fetchSubmissions();
  }, [isTeacher, quizzes, courseId, user?.id]);

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const doCreate = async () => {
    setCreating(true);
    try {
      const created = await api.createQuiz(courseId, {
        title: newQuiz.title,
        description: newQuiz.description,
        quiz_type: newQuiz.quiz_type,
        points_possible: Number(newQuiz.points_possible),
        time_limit: newQuiz.time_limit ? Number(newQuiz.time_limit) : null,
        published: false,
      });
      setQuizzes((prev) => [...prev, created]);
      setNewQuiz({ title: '', description: '', quiz_type: 'assignment', points_possible: 0, time_limit: '' });
      setShowCreate(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleCreate = (e) => {
    e.preventDefault();
    checkAndSave(newQuiz.description, doCreate);
  };

  const formatQuizType = (quizType) => {
    if (!quizType) return 'Quiz';
    const types = {
      practice_quiz: 'Practice Quiz',
      assignment: 'Graded Quiz',
      graded_survey: 'Graded Survey',
      survey: 'Ungraded Survey',
    };
    return types[quizType] || quizType;
  };

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }
  if (error && quizzes.length === 0) {
    return <Layout><div className="text-center py-12"><p className="text-accent-danger mb-3">{error}</p><button onClick={fetchQuizzes} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button></div></Layout>;
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Quizzes</h2>
          {isTeacher && (
            <button
              onClick={() => setShowCreate(!showCreate)}
              className="inline-flex items-center px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4 mr-1" />
              Quiz
            </button>
          )}
        </div>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="bg-surface-0 rounded-lg shadow p-6 mb-6 space-y-4">
          <div>
            <label htmlFor="quiz-title" className="block text-sm font-medium text-text-secondary mb-1">Title</label>
            <input
              id="quiz-title"
              type="text"
              required
              value={newQuiz.title}
              onChange={(e) => setNewQuiz({ ...newQuiz, title: e.target.value })}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              placeholder="Quiz title"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
            <RichContentEditorV2
              value={newQuiz.description}
              onChange={(html) => setNewQuiz((prev) => ({ ...prev, description: html }))}
              placeholder="Quiz instructions..."
              minHeight="120px"
              courseId={courseId}
              autoSaveKey={`quiz-${courseId}-new-description`}
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div>
              <label htmlFor="quiz-type" className="block text-sm font-medium text-text-secondary mb-1">Type</label>
              <select
                id="quiz-type"
                value={newQuiz.quiz_type}
                onChange={(e) => setNewQuiz({ ...newQuiz, quiz_type: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              >
                <option value="assignment">Graded Quiz</option>
                <option value="practice_quiz">Practice Quiz</option>
                <option value="graded_survey">Graded Survey</option>
                <option value="survey">Ungraded Survey</option>
              </select>
            </div>
            <div>
              <label htmlFor="quiz-points" className="block text-sm font-medium text-text-secondary mb-1">Points</label>
              <input
                id="quiz-points"
                type="number"
                min="0"
                value={newQuiz.points_possible}
                onChange={(e) => setNewQuiz({ ...newQuiz, points_possible: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label htmlFor="quiz-time-limit" className="block text-sm font-medium text-text-secondary mb-1">Time Limit (min)</label>
              <input
                id="quiz-time-limit"
                type="number"
                min="0"
                value={newQuiz.time_limit}
                onChange={(e) => setNewQuiz({ ...newQuiz, time_limit: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
                placeholder="No limit"
              />
            </div>
          </div>
          <div className="flex justify-end space-x-3">
            <button type="button" onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
              Cancel
            </button>
            <button type="submit" disabled={creating} className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
              {creating ? 'Creating...' : 'Create Quiz'}
            </button>
          </div>
        </form>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">All Quizzes</h3>
        </div>
        {quizzes.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No quizzes yet.</div>
        ) : (
          <div className="divide-y">
            {quizzes.map((quiz) => {
              const mySub = mySubmissions[quiz.id];
              const isCompleted = mySub && (mySub.workflow_state === 'complete' || mySub.workflow_state === 'pending_review');
              const studentLink = isCompleted && mySub.id
                ? `/courses/${courseId}/quizzes/${quiz.id}/submissions/${mySub.id}/review`
                : `/courses/${courseId}/quizzes/${quiz.id}/take`;

              return (
                <div key={quiz.id} className="flex items-center justify-between p-4 hover:bg-surface-1">
                  <Link
                    to={isTeacher ? `/courses/${courseId}/quizzes/${quiz.id}/edit` : studentLink}
                    className="flex items-center space-x-3 min-w-0 flex-1"
                  >
                    <ClipboardList className="w-5 h-5 text-text-disabled flex-shrink-0" />
                    <div className="min-w-0">
                      <span className="font-medium text-text-primary truncate block">{quiz.title}</span>
                      <div className="flex items-center space-x-3 text-xs text-text-disabled mt-0.5">
                        <span>{formatQuizType(quiz.quiz_type)}</span>
                        {quiz.points_possible != null && (
                          <span>{quiz.points_possible} pts</span>
                        )}
                        {quiz.due_at && (
                          <span>Due {formatDate(quiz.due_at)}</span>
                        )}
                      </div>
                    </div>
                  </Link>
                  <div className="flex items-center space-x-3 flex-shrink-0 ml-4">
                    {/* Student submission status */}
                    {!isTeacher && mySub && (
                      <>
                        {mySub.workflow_state === 'complete' && (
                          <span className="inline-flex items-center gap-1 text-xs text-accent-success bg-accent-success/10 px-2 py-0.5 rounded-full">
                            <CheckCircle className="w-3 h-3" />
                            <span>
                              {mySub.score !== null && mySub.score !== undefined
                                ? `${mySub.score}${quiz.points_possible ? ` / ${quiz.points_possible}` : ''}`
                                : 'Completed'}
                            </span>
                          </span>
                        )}
                        {mySub.workflow_state === 'pending_review' && (
                          <span className="inline-flex items-center gap-1 text-xs text-accent-warning bg-accent-warning/10 px-2 py-0.5 rounded-full">
                            <Clock className="w-3 h-3" />
                            <span>Pending Review</span>
                          </span>
                        )}
                        {mySub.workflow_state === 'untaken' && (
                          <span className="inline-flex items-center gap-1 text-xs text-brand-700 bg-brand-50 px-2 py-0.5 rounded-full">
                            <AlertCircle className="w-3 h-3" />
                            <span>In Progress</span>
                          </span>
                        )}
                        {mySub.attempt > 1 && (
                          <span className="text-xs text-text-disabled">Attempt {mySub.attempt}</span>
                        )}
                        {isCompleted && mySub.id && (
                          <Link
                            to={`/courses/${courseId}/quizzes/${quiz.id}/submissions/${mySub.id}/review`}
                            className="text-xs text-brand-600 hover:underline"
                            onClick={(e) => e.stopPropagation()}
                          >
                            Review
                          </Link>
                        )}
                      </>
                    )}
                    {!isTeacher && !mySub && (
                      <span className="text-xs text-text-disabled">Not attempted</span>
                    )}
                    {/* Teacher controls */}
                    {isTeacher && (
                      <>
                        <Link
                          to={`/courses/${courseId}/quizzes/${quiz.id}/submissions`}
                          className="text-xs text-brand-600 hover:underline"
                        >
                          Submissions
                        </Link>
                        <Link
                          to={`/courses/${courseId}/quizzes/${quiz.id}/statistics`}
                          className="text-xs text-purple-600 hover:underline inline-flex items-center gap-0.5"
                        >
                          <BarChart3 className="w-3 h-3" />
                          Statistics
                        </Link>
                        {quiz.published ? (
                          <span className="inline-flex items-center space-x-1 text-xs text-accent-success bg-accent-success/10 px-2 py-0.5 rounded-full">
                            <Eye className="w-3 h-3" />
                            <span>Published</span>
                          </span>
                        ) : (
                          <span className="inline-flex items-center space-x-1 text-xs text-text-tertiary bg-surface-2 px-2 py-0.5 rounded-full">
                            <EyeOff className="w-3 h-3" />
                            <span>Unpublished</span>
                          </span>
                        )}
                      </>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default QuizzesPage;
