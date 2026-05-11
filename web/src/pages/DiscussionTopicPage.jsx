import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ThumbsUp, MessageSquare, Send } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

const EntryItem = ({ entry, depth = 0, onReply, onRate }) => {
  const [showReplyForm, setShowReplyForm] = useState(false);
  const [replyMessage, setReplyMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleReply = async (e) => {
    e.preventDefault();
    if (!replyMessage.trim()) return;
    setSubmitting(true);
    try {
      await onReply(entry.id, replyMessage);
      setReplyMessage('');
      setShowReplyForm(false);
    } finally {
      setSubmitting(false);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  };

  return (
    <div style={{ marginLeft: depth > 0 ? `${depth * 1.5}rem` : 0 }}>
      <div className={`border-l-2 ${depth > 0 ? 'border-gray-200' : 'border-blue-300'} pl-4 py-3`}>
        <div className="flex items-center space-x-2 mb-1">
          <span className="text-sm font-medium text-gray-700">User {entry.user_id}</span>
          <span className="text-xs text-gray-400">{formatDate(entry.created_at)}</span>
        </div>
        <div className="text-sm text-gray-800 mb-2 whitespace-pre-wrap">{entry.message}</div>
        <div className="flex items-center space-x-4">
          <button
            onClick={() => onRate(entry.id)}
            className="flex items-center space-x-1 text-xs text-gray-500 hover:text-blue-600"
          >
            <ThumbsUp className="w-3.5 h-3.5" />
            <span>{entry.rating_sum > 0 ? entry.rating_sum : ''}</span>
          </button>
          <button
            onClick={() => setShowReplyForm(!showReplyForm)}
            className="flex items-center space-x-1 text-xs text-gray-500 hover:text-blue-600"
          >
            <MessageSquare className="w-3.5 h-3.5" />
            <span>Reply</span>
          </button>
        </div>

        {showReplyForm && (
          <form onSubmit={handleReply} className="mt-2 flex space-x-2">
            <input
              type="text"
              value={replyMessage}
              onChange={(e) => setReplyMessage(e.target.value)}
              placeholder="Write a reply..."
              className="flex-1 border border-gray-300 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              type="submit"
              disabled={submitting || !replyMessage.trim()}
              className="bg-blue-600 text-white px-3 py-1.5 rounded-md hover:bg-blue-700 text-sm disabled:opacity-50"
            >
              <Send className="w-3.5 h-3.5" />
            </button>
          </form>
        )}
      </div>

      {entry.replies && entry.replies.length > 0 && (
        <div>
          {entry.replies.map((reply) => (
            <EntryItem
              key={reply.id}
              entry={reply}
              depth={depth + 1}
              onReply={onReply}
              onRate={onRate}
            />
          ))}
        </div>
      )}
    </div>
  );
};

const DiscussionTopicPage = () => {
  const { courseId, topicId } = useParams();
  const { user } = useAuth();
  const [topic, setTopic] = useState(null);
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [newMessage, setNewMessage] = useState('');
  const [posting, setPosting] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const { data } = await api.getDiscussionTopicView(courseId, topicId);
      setTopic(data.topic);
      setEntries(data.entries || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId, topicId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleNewEntry = async (e) => {
    e.preventDefault();
    if (!newMessage.trim()) return;
    setPosting(true);
    try {
      await api.createDiscussionEntry(courseId, topicId, newMessage);
      setNewMessage('');
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setPosting(false);
    }
  };

  const handleReply = async (entryId, message) => {
    await api.createDiscussionReply(courseId, topicId, entryId, message);
    setLoading(true);
    await fetchData();
  };

  const handleRate = async (entryId) => {
    try {
      await api.rateDiscussionEntry(courseId, topicId, entryId, 1);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-gray-500">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading discussion...
</div></Layout>;
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-red-600 mb-3">{error}</p>
  <button onClick={() => window.location.reload()} className="text-blue-600 hover:text-blue-800 text-sm font-medium">Try Again</button>
</div></Layout>;
  }
  if (!topic) {
    return <Layout><div className="text-center py-12">Discussion not found</div></Layout>;
  }

  return (
    <Layout>
      <div className="mb-6">
        <Link to={`/courses/${courseId}/discussions`} className="text-blue-600 hover:underline text-sm">
          &larr; Back to Discussions
        </Link>
      </div>

      {/* Topic header */}
      <div className="bg-white rounded-lg shadow p-6 mb-6">
        <h2 className="text-2xl font-bold text-gray-900 mb-2">{topic.title}</h2>
        <div className="flex items-center space-x-3 text-sm text-gray-500 mb-4">
          <span>By User {topic.user_id}</span>
          <span className="text-gray-300">|</span>
          <span>{topic.discussion_type === 'threaded' ? 'Threaded' : 'Side Comment'}</span>
          {topic.pinned && (
            <>
              <span className="text-gray-300">|</span>
              <span className="text-blue-600 font-medium">Pinned</span>
            </>
          )}
        </div>
        {topic.message && (
          <div className="text-gray-700 prose max-w-none whitespace-pre-wrap">{topic.message}</div>
        )}
      </div>

      {/* Entries */}
      <div className="bg-white rounded-lg shadow mb-6">
        <div className="p-4 border-b">
          <h3 className="font-semibold">Replies</h3>
        </div>
        {entries.length === 0 ? (
          <div className="p-6 text-center text-gray-500">No replies yet. Be the first to respond!</div>
        ) : (
          <div className="p-4 space-y-2">
            {entries.map((entry) => (
              <EntryItem
                key={entry.id}
                entry={entry}
                depth={0}
                onReply={handleReply}
                onRate={handleRate}
              />
            ))}
          </div>
        )}
      </div>

      {/* New entry form */}
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="font-semibold mb-3">Post a Reply</h3>
        <form onSubmit={handleNewEntry} className="space-y-3">
          <textarea
            value={newMessage}
            onChange={(e) => setNewMessage(e.target.value)}
            placeholder="Write your reply..."
            className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            rows={3}
          />
          <div className="flex justify-end">
            <button
              type="submit"
              disabled={posting || !newMessage.trim()}
              className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 text-sm font-medium disabled:opacity-50"
            >
              {posting ? 'Posting...' : 'Post Reply'}
            </button>
          </div>
        </form>
      </div>
    </Layout>
  );
};

export default DiscussionTopicPage;
