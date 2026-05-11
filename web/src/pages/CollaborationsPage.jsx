import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { FileText, Plus, X, Pencil, Trash2, ExternalLink } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const CollaborationsPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [collaborations, setCollaborations] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    collaboration_type: 'google_docs',
    url: '',
    document_id: '',
  });
  const [submitting, setSubmitting] = useState(false);

  const fetchCollaborations = async () => {
    try {
      const result = await api.getCollaborations(courseId);
      setCollaborations(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCollaborations();
  }, [courseId]);

  const resetForm = () => {
    setFormData({
      title: '',
      description: '',
      collaboration_type: 'google_docs',
      url: '',
      document_id: '',
    });
    setEditingId(null);
    setShowForm(false);
  };

  const handleEdit = (collab) => {
    setFormData({
      title: collab.title,
      description: collab.description || '',
      collaboration_type: collab.collaboration_type,
      url: collab.url || '',
      document_id: collab.document_id || '',
    });
    setEditingId(collab.id);
    setShowForm(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      if (editingId) {
        await api.updateCollaboration(courseId, editingId, formData);
      } else {
        await api.createCollaboration(courseId, formData);
      }
      resetForm();
      setLoading(true);
      await fetchCollaborations();
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id) => {
    if (!window.confirm('Are you sure you want to delete this collaboration?')) return;
    try {
      await api.deleteCollaboration(courseId, id);
      setLoading(true);
      await fetchCollaborations();
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

  const typeLabel = (type) => {
    switch (type) {
      case 'google_docs': return 'Google Docs';
      case 'etherpad': return 'Etherpad';
      default: return type;
    }
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading collaborations...
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
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Collaborations</h2>
          <button
            onClick={() => { if (showForm) { resetForm(); } else { setShowForm(true); } }}
            className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
          >
            {showForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            <span>{showForm ? 'Cancel' : 'New Collaboration'}</span>
          </button>
        </div>
      </div>

      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{editingId ? 'Edit Collaboration' : 'Create Collaboration'}</h3>
          <form onSubmit={handleSubmit} className="space-y-4">
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
              <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
              <textarea
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                rows={3}
              />
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Collaboration Type</label>
                <select
                  value={formData.collaboration_type}
                  onChange={(e) => setFormData({ ...formData, collaboration_type: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="google_docs">Google Docs</option>
                  <option value="etherpad">Etherpad</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Document URL</label>
                <input
                  type="url"
                  value={formData.url}
                  onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  placeholder="https://..."
                />
              </div>
            </div>
            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={resetForm}
                className="px-4 py-2 border border-border-strong rounded-md text-sm text-text-secondary hover:bg-surface-1"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={submitting}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
              >
                {submitting ? 'Saving...' : (editingId ? 'Update Collaboration' : 'Create Collaboration')}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">All Collaborations</h3>
        </div>
        {collaborations.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No collaborations yet.</div>
        ) : (
          <div className="divide-y">
            {collaborations.map((collab) => (
              <div
                key={collab.id}
                className="flex items-center justify-between p-4 hover:bg-surface-1"
              >
                <div className="flex items-center space-x-3 min-w-0">
                  <FileText className="w-5 h-5 text-text-disabled flex-shrink-0" />
                  <div className="min-w-0">
                    <div className="flex items-center space-x-2">
                      <span className="font-medium text-text-primary truncate">{collab.title}</span>
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-surface-2 text-text-secondary">
                        {typeLabel(collab.collaboration_type)}
                      </span>
                    </div>
                    {collab.description && (
                      <p className="text-sm text-text-tertiary truncate">{collab.description}</p>
                    )}
                    <span className="text-xs text-text-disabled">Created {formatDate(collab.created_at)}</span>
                  </div>
                </div>
                <div className="flex items-center space-x-2 flex-shrink-0 ml-4">
                  {collab.url && (
                    <a
                      href={collab.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="p-1.5 text-brand-600 hover:bg-brand-50 rounded"
                      title="Open document"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </a>
                  )}
                  <button
                    onClick={() => handleEdit(collab)}
                    className="p-1.5 text-text-disabled hover:text-brand-600 hover:bg-brand-50 rounded"
                    title="Edit"
                  >
                    <Pencil className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(collab.id)}
                    className="p-1.5 text-text-disabled hover:text-accent-danger hover:bg-accent-danger/10 rounded"
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </Layout>
  );
};

export default CollaborationsPage;
