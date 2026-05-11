import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { FileText, Clock, CheckCircle, AlertCircle } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const QuizSubmissionsPage = () => {
  const { courseId, quizId } = useParams();
  const [quiz, setQuiz] = useState(null);
  const [submissions, setSubmissions] = useState([]);
  const [userNames, setUserNames] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [quizResult, subsResult, enrollResult] = await Promise.allSettled([
          api.getQuiz(courseId, quizId),
          api.getQuizSubmissions(courseId, quizId, 1, 100),
          api.getEnrollments(courseId, 1, 200),
        ]);
        if (quizResult.status === 'rejected') throw new Error(quizResult.reason?.message || 'Failed to load quiz');
        setQuiz(quizResult.value);
        setSubmissions(subsResult.status === 'fulfilled' ? (subsResult.value.data || []) : []);
        // Build user name lookup from enrollments
        const names = {};
        if (enrollResult.status === 'fulfilled') {
          for (const e of (enrollResult.value.data || [])) {
            const uid = e.user_id || e.user?.id;
            const name = e.user?.name || e.user?.email;
            if (uid && name) names[uid] = name;
          }
        }
        setUserNames(names);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, quizId]);

  const stateIcon = (state) => {
    switch (state) {
      case 'complete': return <CheckCircle className="w-4 h-4 text-accent-success" />;
      case 'pending_review': return <AlertCircle className="w-4 h-4 text-accent-warning" />;
      default: return <Clock className="w-4 h-4 text-text-disabled" />;
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString();
  };

  const formatDuration = (seconds) => {
    if (!seconds) return '-';
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}m ${s}s`;
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading submissions...
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
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">← Back to Course</Link>
        <h2 className="text-2xl font-bold mt-2">{quiz?.title} - Submissions</h2>
        <p className="text-text-tertiary">
          {quiz?.points_possible ? `${quiz.points_possible} points` : 'Ungraded'}
          {quiz?.time_limit ? ` · ${quiz.time_limit} min time limit` : ''}
        </p>
      </div>

      <div className="bg-surface-0 rounded-lg shadow">
        <table className="w-full">
          <thead>
            <tr className="border-b text-left text-sm text-text-tertiary">
              <th className="px-4 py-3">User</th>
              <th className="px-4 py-3">Attempt</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Score</th>
              <th className="px-4 py-3">Started</th>
              <th className="px-4 py-3">Finished</th>
              <th className="px-4 py-3">Time Spent</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {submissions.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-text-tertiary">No submissions yet.</td>
              </tr>
            ) : (
              submissions.map(sub => (
                <tr key={sub.id} className="hover:bg-surface-1">
                  <td className="px-4 py-3 text-sm">{userNames[sub.user_id] || `User #${sub.user_id}`}</td>
                  <td className="px-4 py-3 text-sm">{sub.attempt}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center space-x-2">
                      {stateIcon(sub.workflow_state)}
                      <span className="text-sm capitalize">{sub.workflow_state?.replace('_', ' ')}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-sm font-medium">
                    {sub.score !== null && sub.score !== undefined ? sub.score : '-'}
                    {sub.score !== null && quiz?.points_possible ? ` / ${quiz.points_possible}` : ''}
                  </td>
                  <td className="px-4 py-3 text-sm text-text-tertiary">{formatDate(sub.started_at)}</td>
                  <td className="px-4 py-3 text-sm text-text-tertiary">{formatDate(sub.finished_at)}</td>
                  <td className="px-4 py-3 text-sm text-text-tertiary">{formatDuration(sub.time_spent)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </Layout>
  );
};

export default QuizSubmissionsPage;
