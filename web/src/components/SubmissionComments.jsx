import React, { useState, useEffect } from 'react';
import { Send, MessageSquare, Eye, EyeOff } from 'lucide-react';
import { api } from '../services/api';

const SubmissionComments = ({ courseId, assignmentId, userId, isTeacher }) => {
  const [comments, setComments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [newComment, setNewComment] = useState('');
  const [isDraft, setIsDraft] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    fetchComments();
  }, [courseId, assignmentId, userId]);

  const fetchComments = async () => {
    try {
      const data = await api.getSubmissionComments(courseId, assignmentId, userId);
      setComments(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!newComment.trim()) return;

    setSubmitting(true);
    try {
      const comment = {
        text_comment: newComment,
        ...(isTeacher ? { draft: isDraft } : {}),
      };
      const created = await api.createSubmissionComment(courseId, assignmentId, userId, comment);
      setComments(prev => [...prev, created]);
      setNewComment('');
      setIsDraft(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleString();
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center space-x-2">
        <MessageSquare className="w-5 h-5 text-text-tertiary" />
        <h4 className="font-semibold text-text-primary">Comments</h4>
      </div>

      {loading ? (
        <div className="text-sm text-text-tertiary py-2">Loading comments...</div>
      ) : error ? (
        <div className="text-sm text-accent-danger py-2">{error}</div>
      ) : comments.length === 0 ? (
        <div className="text-sm text-text-disabled py-2">No comments yet.</div>
      ) : (
        <div className="space-y-3">
          {comments.map((comment) => (
            <div
              key={comment.id}
              className={`rounded-lg p-3 text-sm ${
                comment.draft
                  ? 'bg-accent-warning/10 border border-accent-warning/30'
                  : 'bg-surface-1 border border-border-default'
              }`}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="font-medium text-text-primary">
                  {comment.author_name || comment.author?.display_name || 'Unknown'}
                </span>
                <div className="flex items-center space-x-2">
                  {comment.draft && (
                    <span className="text-xs bg-yellow-200 text-accent-warning px-1.5 py-0.5 rounded">
                      Draft
                    </span>
                  )}
                  <span className="text-xs text-text-disabled">
                    {formatDate(comment.created_at)}
                  </span>
                </div>
              </div>
              <p className="text-text-secondary">{comment.comment || comment.text_comment}</p>
            </div>
          ))}
        </div>
      )}

      <form onSubmit={handleSubmit} className="border-t pt-4">
        <div className="flex space-x-2">
          <input
            type="text"
            value={newComment}
            onChange={(e) => setNewComment(e.target.value)}
            placeholder="Add a comment..."
            className="flex-1 border border-border-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
            disabled={submitting}
          />
          <button
            type="submit"
            disabled={submitting || !newComment.trim()}
            className="inline-flex items-center space-x-1 bg-brand-600 text-white px-3 py-2 rounded-lg hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
        {isTeacher && (
          <div className="flex items-center space-x-2 mt-2">
            <button
              type="button"
              onClick={() => setIsDraft(!isDraft)}
              className={`inline-flex items-center space-x-1 text-xs px-2 py-1 rounded ${
                isDraft
                  ? 'bg-accent-warning/20 text-accent-warning border border-yellow-300'
                  : 'bg-surface-2 text-text-secondary border border-border-default'
              }`}
            >
              {isDraft ? (
                <>
                  <EyeOff className="w-3 h-3" />
                  <span>Draft (not visible to student)</span>
                </>
              ) : (
                <>
                  <Eye className="w-3 h-3" />
                  <span>Published (visible to student)</span>
                </>
              )}
            </button>
          </div>
        )}
      </form>
    </div>
  );
};

export default SubmissionComments;
