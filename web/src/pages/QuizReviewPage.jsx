import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { CheckCircle, XCircle, MinusCircle, HelpCircle } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { sanitizeHTML } from '../components/RichContentViewer';

const QuizReviewPage = () => {
  const { courseId, quizId, submissionId } = useParams();
  const [quiz, setQuiz] = useState(null);
  const [submission, setSubmission] = useState(null);
  const [answers, setAnswers] = useState([]);
  const [questions, setQuestions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [quizResult, subResult, answersResult, questionsResult] = await Promise.allSettled([
          api.getQuiz(courseId, quizId),
          api.getQuizSubmission(courseId, quizId, submissionId),
          api.getQuizSubmissionAnswers(courseId, quizId, submissionId),
          api.getQuizQuestions(courseId, quizId, 1, 100),
        ]);
        if (quizResult.status === 'rejected') throw new Error(quizResult.reason?.message || 'Failed to load quiz');
        if (subResult.status === 'rejected') throw new Error(subResult.reason?.message || 'Failed to load submission');
        setQuiz(quizResult.value);
        setSubmission(subResult.value);
        setAnswers(answersResult.status === 'fulfilled' ? (answersResult.value || []) : []);
        setQuestions(questionsResult.status === 'fulfilled' ? (questionsResult.value.data || []) : []);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, quizId, submissionId]);

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
      <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
      Loading quiz review...
    </div></Layout>;
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-accent-danger mb-3">{error}</p>
  <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
</div></Layout>;
  }

  // Build a map of question_id -> answer
  const answerMap = {};
  for (const a of answers) {
    answerMap[a.question_id] = a;
  }

  const getAnswerIcon = (answer) => {
    if (!answer) return <MinusCircle className="w-5 h-5 text-text-disabled" />;
    if (answer.correct === true) return <CheckCircle className="w-5 h-5 text-accent-success" />;
    if (answer.correct === false) return <XCircle className="w-5 h-5 text-accent-danger" />;
    return <HelpCircle className="w-5 h-5 text-accent-warning" />;
  };

  const getAnswerLabel = (answer) => {
    if (!answer) return 'Not answered';
    if (answer.correct === true) return 'Correct';
    if (answer.correct === false) return 'Incorrect';
    return 'Pending review';
  };

  const formatStudentAnswer = (question, answer) => {
    if (!answer || !answer.answer) return 'No answer provided';

    if (question.question_type === 'multiple_choice' || question.question_type === 'true_false') {
      // Answer is an option ID — resolve to text
      try {
        const options = JSON.parse(question.answers || '[]');
        const selected = options.find(o => String(o.id) === String(answer.answer));
        return selected ? selected.text : answer.answer;
      } catch {
        return answer.answer;
      }
    }

    return answer.answer;
  };

  const totalPoints = answers.reduce((sum, a) => sum + (a.points || 0), 0);
  const maxPoints = questions.reduce((sum, q) => sum + (q.points_possible || 0), 0);

  return (
    <Layout>
      <CourseNav />
      <div className="max-w-3xl mx-auto">
        <div className="mb-4">
          <Link to={`/courses/${courseId}/quizzes`} className="text-brand-600 hover:underline text-sm">
            &larr; Back to Quizzes
          </Link>
        </div>

        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h2 className="text-2xl font-bold mb-2">{quiz?.title} — Review</h2>
          <div className="flex items-center gap-4 text-sm text-text-secondary">
            {submission?.score !== null && submission?.score !== undefined && (
              <span>
                Score: <span className="font-semibold text-text-primary">{submission.score}</span>
                {quiz?.points_possible ? ` / ${quiz.points_possible}` : ''}
              </span>
            )}
            <span>
              Points earned: <span className="font-semibold text-text-primary">{totalPoints}</span> / {maxPoints}
            </span>
            {submission?.workflow_state === 'pending_review' && (
              <span className="text-accent-warning font-medium">Some questions pending review</span>
            )}
          </div>
        </div>

        <div className="space-y-4">
          {questions.map((question, idx) => {
            const answer = answerMap[question.id];
            return (
              <div key={question.id} className="bg-surface-0 rounded-lg shadow p-6">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    {getAnswerIcon(answer)}
                    <span className="text-sm font-medium text-text-tertiary">
                      Question {idx + 1}
                    </span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${
                      answer?.correct === true ? 'bg-accent-success/20 text-accent-success' :
                      answer?.correct === false ? 'bg-accent-danger/20 text-accent-danger' :
                      'bg-accent-warning/20 text-accent-warning'
                    }`}>
                      {getAnswerLabel(answer)}
                    </span>
                  </div>
                  <span className="text-sm text-text-tertiary">
                    {answer ? `${answer.points || 0}` : '0'} / {question.points_possible || 0} pts
                  </span>
                </div>

                <div
                  className="text-text-primary mb-4 prose max-w-none"
                  dangerouslySetInnerHTML={{ __html: sanitizeHTML(question.question_text) }}
                />

                {/* Show answer options for MC/TF with highlighting */}
                {(question.question_type === 'multiple_choice' || question.question_type === 'true_false') && (() => {
                  let options = [];
                  try { options = JSON.parse(question.answers || '[]'); } catch { /* ignore */ }
                  return (
                    <div className="space-y-2 mb-3">
                      {options.map(opt => {
                        const isSelected = answer && String(answer.answer) === String(opt.id);
                        const isCorrect = opt.weight > 0;
                        let borderClass = 'border-border-default';
                        let bgClass = '';
                        if (isSelected && answer?.correct === true) {
                          borderClass = 'border-accent-success/60';
                          bgClass = 'bg-accent-success/10';
                        } else if (isSelected && answer?.correct === false) {
                          borderClass = 'border-accent-danger/60';
                          bgClass = 'bg-accent-danger/10';
                        } else if (isCorrect) {
                          borderClass = 'border-accent-success/40';
                          bgClass = 'bg-accent-success/10/50';
                        }
                        return (
                          <div
                            key={opt.id}
                            className={`flex items-center gap-3 p-3 rounded border ${borderClass} ${bgClass}`}
                          >
                            <div className={`w-4 h-4 rounded-full border-2 flex-shrink-0 ${
                              isSelected ? 'border-brand-500 bg-brand-500' : 'border-border-strong'
                            }`}>
                              {isSelected && <div className="w-full h-full rounded-full" />}
                            </div>
                            <span className="text-sm">{opt.text}</span>
                            {isCorrect && !isSelected && (
                              <span className="text-xs text-accent-success ml-auto">Correct answer</span>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  );
                })()}

                {/* Show text answers for short answer / essay / numerical */}
                {(question.question_type === 'short_answer' || question.question_type === 'essay' || question.question_type === 'numerical_question') && (
                  <div className="mb-3">
                    <p className="text-sm font-medium text-text-tertiary mb-1">Your answer:</p>
                    <div className={`p-3 rounded border text-sm ${
                      answer?.correct === true ? 'border-accent-success/40 bg-accent-success/10' :
                      answer?.correct === false ? 'border-accent-danger/40 bg-accent-danger/10' :
                      'border-border-default bg-surface-1'
                    }`}>
                      {formatStudentAnswer(question, answer)}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        <div className="mt-6 text-center">
          <Link
            to={`/courses/${courseId}`}
            className="text-brand-600 hover:underline"
          >
            Back to Course
          </Link>
        </div>
      </div>
    </Layout>
  );
};

export default QuizReviewPage;
