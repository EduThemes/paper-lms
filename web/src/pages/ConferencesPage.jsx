import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Video, Plus, X, Pencil, Trash2, Play, Square, LogIn, Film } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const ConferencesPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [conferences, setConferences] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    conference_type: 'BigBlueButton',
    duration: 60,
  });
  const [submitting, setSubmitting] = useState(false);
  const [selectedConference, setSelectedConference] = useState(null);
  const [recordings, setRecordings] = useState(null);
  const [participants, setParticipants] = useState(null);

  const fetchConferences = async () => {
    try {
      const result = await api.getConferences(courseId);
      setConferences(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchConferences();
  }, [courseId]);

  const resetForm = () => {
    setFormData({
      title: '',
      description: '',
      conference_type: 'BigBlueButton',
      duration: 60,
    });
    setEditingId(null);
    setShowForm(false);
  };

  const handleEdit = (conf) => {
    setFormData({
      title: conf.title,
      description: conf.description || '',
      conference_type: conf.conference_type,
      duration: conf.duration || 60,
    });
    setEditingId(conf.id);
    setShowForm(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      if (editingId) {
        await api.updateConference(courseId, editingId, formData);
      } else {
        await api.createConference(courseId, formData);
      }
      resetForm();
      setLoading(true);
      await fetchConferences();
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id) => {
    if (!window.confirm('Are you sure you want to delete this conference?')) return;
    try {
      await api.deleteConference(courseId, id);
      setLoading(true);
      await fetchConferences();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleJoin = async (id) => {
    try {
      const result = await api.joinConference(courseId, id);
      if (result.join_url) {
        window.open(result.join_url, '_blank');
      }
    } catch (err) {
      setError(err.message);
    }
  };

  const handleEnd = async (id) => {
    if (!window.confirm('Are you sure you want to end this conference?')) return;
    try {
      await api.endConference(courseId, id);
      setLoading(true);
      await fetchConferences();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleViewRecordings = async (conf) => {
    try {
      const result = await api.getConferenceRecordings(courseId, conf.id);
      setRecordings(result.recordings);
      setSelectedConference(conf);
      setParticipants(null);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleViewParticipants = async (conf) => {
    try {
      const result = await api.getConferenceParticipants(courseId, conf.id);
      setParticipants(result);
      setSelectedConference(conf);
      setRecordings(null);
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
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const conferenceStatus = (conf) => {
    if (conf.ended_at) return { label: 'Ended', color: 'bg-surface-2 text-text-secondary' };
    if (conf.started_at) return { label: 'In Progress', color: 'bg-accent-success/20 text-accent-success' };
    return { label: 'Not Started', color: 'bg-accent-warning/20 text-accent-warning' };
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading conferences...
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
          <h2 className="text-2xl font-bold text-text-primary">Conferences</h2>
          <button
            onClick={() => { if (showForm) { resetForm(); } else { setShowForm(true); } }}
            className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
          >
            {showForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
            <span>{showForm ? 'Cancel' : 'New Conference'}</span>
          </button>
        </div>
      </div>

      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{editingId ? 'Edit Conference' : 'Create Conference'}</h3>
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
                <label className="block text-sm font-medium text-text-secondary mb-1">Conference Type</label>
                <select
                  value={formData.conference_type}
                  onChange={(e) => setFormData({ ...formData, conference_type: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="BigBlueButton">BigBlueButton</option>
                  <option value="Zoom">Zoom</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Duration (minutes)</label>
                <input
                  type="number"
                  value={formData.duration}
                  onChange={(e) => setFormData({ ...formData, duration: parseInt(e.target.value) || 0 })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  min={1}
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
                {submitting ? 'Saving...' : (editingId ? 'Update Conference' : 'Create Conference')}
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">All Conferences</h3>
        </div>
        {conferences.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No conferences yet.</div>
        ) : (
          <div className="divide-y">
            {conferences.map((conf) => {
              const status = conferenceStatus(conf);
              return (
                <div
                  key={conf.id}
                  className="p-4 hover:bg-surface-1"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3 min-w-0">
                      <Video className="w-5 h-5 text-text-disabled flex-shrink-0" />
                      <div className="min-w-0">
                        <div className="flex items-center space-x-2">
                          <span className="font-medium text-text-primary truncate">{conf.title}</span>
                          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${status.color}`}>
                            {status.label}
                          </span>
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-brand-50 text-brand-700">
                            {conf.conference_type}
                          </span>
                        </div>
                        {conf.description && (
                          <p className="text-sm text-text-tertiary truncate">{conf.description}</p>
                        )}
                        <div className="flex items-center space-x-4 mt-1">
                          <span className="text-xs text-text-disabled">
                            Created {formatDate(conf.created_at)}
                          </span>
                          {conf.duration > 0 && (
                            <span className="text-xs text-text-disabled">{conf.duration} min</span>
                          )}
                          {conf.started_at && (
                            <span className="text-xs text-text-disabled">Started {formatDate(conf.started_at)}</span>
                          )}
                          {conf.ended_at && (
                            <span className="text-xs text-text-disabled">Ended {formatDate(conf.ended_at)}</span>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center space-x-1 flex-shrink-0 ml-4">
                      {!conf.started_at && (
                        <button
                          onClick={() => handleJoin(conf.id)}
                          className="inline-flex items-center space-x-1 px-3 py-1.5 bg-accent-success text-white rounded-md hover:bg-accent-success/90 text-xs font-medium"
                          title="Start and Join"
                        >
                          <Play className="w-3.5 h-3.5" />
                          <span>Start</span>
                        </button>
                      )}
                      {conf.started_at && !conf.ended_at && (
                        <>
                          <button
                            onClick={() => handleJoin(conf.id)}
                            className="inline-flex items-center space-x-1 px-3 py-1.5 bg-accent-success text-white rounded-md hover:bg-accent-success/90 text-xs font-medium"
                            title="Join Conference"
                          >
                            <LogIn className="w-3.5 h-3.5" />
                            <span>Join</span>
                          </button>
                          <button
                            onClick={() => handleEnd(conf.id)}
                            className="inline-flex items-center space-x-1 px-3 py-1.5 bg-accent-danger text-white rounded-md hover:bg-accent-danger/90 text-xs font-medium"
                            title="End Conference"
                          >
                            <Square className="w-3.5 h-3.5" />
                            <span>End</span>
                          </button>
                        </>
                      )}
                      <button
                        onClick={() => handleViewRecordings(conf)}
                        className="p-1.5 text-text-disabled hover:text-purple-600 hover:bg-purple-50 rounded"
                        title="Recordings"
                      >
                        <Film className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleEdit(conf)}
                        className="p-1.5 text-text-disabled hover:text-brand-600 hover:bg-brand-50 rounded"
                        title="Edit"
                      >
                        <Pencil className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(conf.id)}
                        className="p-1.5 text-text-disabled hover:text-accent-danger hover:bg-accent-danger/10 rounded"
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Recordings panel */}
      {selectedConference && recordings !== null && (
        <div className="mt-6 bg-surface-0 rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold">Recordings - {selectedConference.title}</h3>
            <button
              onClick={() => { setRecordings(null); setSelectedConference(null); }}
              className="text-text-disabled hover:text-text-secondary"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
          {recordings === '[]' || !recordings ? (
            <p className="text-sm text-text-tertiary">No recordings available for this conference.</p>
          ) : (
            <pre className="text-sm bg-surface-1 rounded p-3 overflow-auto">{recordings}</pre>
          )}
        </div>
      )}

      {/* Participants panel */}
      {selectedConference && participants !== null && (
        <div className="mt-6 bg-surface-0 rounded-lg shadow p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold">Participants - {selectedConference.title}</h3>
            <button
              onClick={() => { setParticipants(null); setSelectedConference(null); }}
              className="text-text-disabled hover:text-text-secondary"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
          {participants.length === 0 ? (
            <p className="text-sm text-text-tertiary">No participants yet.</p>
          ) : (
            <div className="divide-y">
              {participants.map((p) => (
                <div key={p.id} className="flex items-center justify-between py-2">
                  <span className="text-sm text-text-primary">{p.user?.name || `User #${p.user_id}`}</span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-surface-2 text-text-secondary">
                    {p.participation_type}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Layout>
  );
};

export default ConferencesPage;
