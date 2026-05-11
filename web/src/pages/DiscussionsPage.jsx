import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { MessageSquare, Pin, Plus, X, Edit2, Trash2 } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { Skeleton } from '@/components/ui/skeleton';

const DiscussionsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [topics, setTopics] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({
    title: '',
    message: '',
    discussion_type: 'side_comment',
    pinned: false,
  });
  const [creating, setCreating] = useState(false);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  const fetchTopics = async () => {
    try {
      const result = await api.getDiscussionTopics(courseId);
      setTopics(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTopics();
  }, [courseId]);

  const resetForm = () => {
    setFormData({ title: '', message: '', discussion_type: 'side_comment', pinned: false });
    setEditingId(null);
  };

  const doCreate = async () => {
    setCreating(true);
    try {
      if (editingId) {
        await api.updateDiscussionTopic(courseId, editingId, formData);
      } else {
        await api.createDiscussionTopic(courseId, formData);
      }
      resetForm();
      setShowForm(false);
      setLoading(true);
      await fetchTopics();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleCreate = (e) => {
    e.preventDefault();
    checkAndSave(formData.message, doCreate);
  };

  const handleEdit = (topic) => {
    setFormData({
      title: topic.title || '',
      message: topic.message || '',
      discussion_type: topic.discussion_type || 'side_comment',
      pinned: topic.pinned || false,
    });
    setEditingId(topic.id);
    setShowForm(true);
  };

  const handleDelete = async (topicId) => {
    if (!window.confirm('Delete this discussion topic?')) return;
    try {
      await api.deleteDiscussionTopic(courseId, topicId);
      setLoading(true);
      await fetchTopics();
    } catch (err) {
      setError(err.message);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
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
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Discussions</h2>
          {isTeacher && (
            <button
              onClick={() => {
                if (showForm) { resetForm(); }
                setShowForm(!showForm);
              }}
              className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              {showForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
              <span>{showForm ? 'Cancel' : 'New Discussion'}</span>
            </button>
          )}
        </div>
      </div>

      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{editingId ? 'Edit Discussion Topic' : 'Create Discussion Topic'}</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Title</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Message</label>
              <RichContentEditorV2
                value={formData.message}
                onChange={(html) => setFormData((prev) => ({ ...prev, message: html }))}
                placeholder="Discussion topic content..."
                minHeight="160px"
                courseId={courseId}
                autoSaveKey={`discussion-${courseId}-${editingId || 'new'}-message`}
              />
            </div>
            <div className="flex items-center space-x-6">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Discussion Type</label>
                <select
                  value={formData.discussion_type}
                  onChange={(e) => setFormData({ ...formData, discussion_type: e.target.value })}
                  className="border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="side_comment">Side Comment</option>
                  <option value="threaded">Threaded</option>
                </select>
              </div>
              <div className="flex items-center space-x-2 pt-5">
                <input
                  type="checkbox"
                  id="pinned"
                  checked={formData.pinned}
                  onChange={(e) => setFormData({ ...formData, pinned: e.target.checked })}
                  className="rounded border-border-strong"
                />
                <label htmlFor="pinned" className="text-sm text-text-secondary">Pinned</label>
              </div>
            </div>
            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={() => { resetForm(); setShowForm(false); }}
                className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={creating}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
              >
                {creating ? 'Saving...' : editingId ? 'Update Topic' : 'Create Topic'}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">Topics</h3>
        </div>
        {topics.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No discussions yet.</div>
        ) : (
          <div className="divide-y">
            {topics.map((topic) => (
              <div key={topic.id} className="flex items-center justify-between p-4 hover:bg-surface-1 group">
                <Link
                  to={`/courses/${courseId}/discussions/${topic.id}`}
                  className="flex items-center space-x-3 min-w-0 flex-1"
                >
                  <MessageSquare className="w-5 h-5 text-text-disabled flex-shrink-0" />
                  <div className="min-w-0">
                    <div className="flex items-center space-x-2">
                      <span className="font-medium text-text-primary truncate">{topic.title}</span>
                      {topic.pinned && (
                        <Pin className="w-3.5 h-3.5 text-brand-500 flex-shrink-0" />
                      )}
                    </div>
                    <span className="text-xs text-text-disabled">
                      {topic.discussion_type === 'side_comment' ? 'Side Comment' : topic.discussion_type === 'threaded' ? 'Threaded' : topic.discussion_type}
                    </span>
                  </div>
                </Link>
                <div className="flex items-center gap-2 flex-shrink-0 ml-4">
                  <span className="text-xs text-text-disabled">
                    {formatDate(topic.created_at)}
                  </span>
                  {isTeacher && (
                    <>
                      <button
                        onClick={(e) => { e.preventDefault(); handleEdit(topic); }}
                        className="p-1 text-text-disabled hover:text-brand-600 opacity-0 group-hover:opacity-100"
                        title="Edit"
                      >
                        <Edit2 className="w-4 h-4" />
                      </button>
                      <button
                        onClick={(e) => { e.preventDefault(); handleDelete(topic.id); }}
                        className="p-1 text-text-disabled hover:text-accent-danger opacity-0 group-hover:opacity-100"
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default DiscussionsPage;
