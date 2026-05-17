import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Navigate } from 'react-router-dom';
import { Settings, Link2, RefreshCw, Clock, CheckCircle, AlertCircle, Plus, Trash2, X, Loader2 } from 'lucide-react';
import { api, getCSRFToken } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const getHeaders = () => ({
  'Content-Type': 'application/json',
  'X-CSRF-Token': getCSRFToken(),
});

const BlueprintPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);

  // Template state
  const [template, setTemplate] = useState(null);
  const [restrictions, setRestrictions] = useState('{}');
  const [useDefaults, setUseDefaults] = useState(true);
  const [savingTemplate, setSavingTemplate] = useState(false);

  // Associations state
  const [associations, setAssociations] = useState([]);
  const [newCourseId, setNewCourseId] = useState('');
  const [addingCourse, setAddingCourse] = useState(false);

  // Migrations state
  const [migrations, setMigrations] = useState([]);
  const [syncComment, setSyncComment] = useState('');
  const [syncing, setSyncing] = useState(false);

  // General state
  const [activeTab, setActiveTab] = useState('settings');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);

  const fetchTemplate = useCallback(async () => {
    try {
      const response = await fetch(`${API_URL}/courses/${courseId}/blueprint_templates/default`, {
        credentials: 'include', headers: getHeaders(),
      });
      if (!response.ok) throw new Error('Failed to fetch blueprint template');
      const data = await response.json();
      setTemplate(data);
      setRestrictions(data.default_restrictions || '{}');
      setUseDefaults(data.use_default_restrictions !== false);
    } catch (err) {
      setError(err.message);
    }
  }, [courseId]);

  const fetchAssociations = useCallback(async () => {
    try {
      const response = await fetch(
        `${API_URL}/courses/${courseId}/blueprint_templates/default/associated_courses?per_page=100`,
        { credentials: 'include', headers: getHeaders() }
      );
      if (!response.ok) throw new Error('Failed to fetch associated courses');
      const data = await response.json();
      setAssociations(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    }
  }, [courseId]);

  const fetchMigrations = useCallback(async () => {
    try {
      const response = await fetch(
        `${API_URL}/courses/${courseId}/blueprint_templates/default/migrations?per_page=50`,
        { credentials: 'include', headers: getHeaders() }
      );
      if (!response.ok) throw new Error('Failed to fetch sync history');
      const data = await response.json();
      setMigrations(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    }
  }, [courseId]);

  useEffect(() => {
    const loadAll = async () => {
      setLoading(true);
      await Promise.all([fetchTemplate(), fetchAssociations(), fetchMigrations()]);
      setLoading(false);
    };
    loadAll();
  }, [fetchTemplate, fetchAssociations, fetchMigrations]);

  const handleSaveTemplate = async (e) => {
    e.preventDefault();
    setSavingTemplate(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await fetch(`${API_URL}/courses/${courseId}/blueprint_templates/default`, {
        method: 'PUT',
        credentials: 'include', headers: getHeaders(),
        body: JSON.stringify({
          blueprint_template: {
            default_restrictions: restrictions,
            use_default_restrictions: useDefaults,
          },
        }),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to save template settings');
      }

      const data = await response.json();
      setTemplate(data);
      setSuccess('Blueprint settings saved successfully.');
    } catch (err) {
      setError(err.message);
    } finally {
      setSavingTemplate(false);
    }
  };

  const handleAddCourse = async (e) => {
    e.preventDefault();
    if (!newCourseId.trim()) return;

    setAddingCourse(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await fetch(
        `${API_URL}/courses/${courseId}/blueprint_templates/default/associated_courses`,
        {
          method: 'PUT',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify({
            course_ids_to_add: [parseInt(newCourseId, 10)],
            course_ids_to_remove: [],
          }),
        }
      );

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to add associated course');
      }

      setNewCourseId('');
      setSuccess('Course associated successfully.');
      await fetchAssociations();
    } catch (err) {
      setError(err.message);
    } finally {
      setAddingCourse(false);
    }
  };

  const handleRemoveCourse = async (childCourseId) => {
    setError(null);
    setSuccess(null);

    try {
      const response = await fetch(
        `${API_URL}/courses/${courseId}/blueprint_templates/default/associated_courses`,
        {
          method: 'PUT',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify({
            course_ids_to_add: [],
            course_ids_to_remove: [childCourseId],
          }),
        }
      );

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to remove associated course');
      }

      setSuccess('Course removed from associations.');
      await fetchAssociations();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSync = async (e) => {
    e.preventDefault();
    setSyncing(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await fetch(
        `${API_URL}/courses/${courseId}/blueprint_templates/default/migrations`,
        {
          method: 'POST',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify({
            comment: syncComment,
            send_notification: false,
          }),
        }
      );

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to trigger sync');
      }

      setSyncComment('');
      setSuccess('Blueprint sync completed successfully.');
      await fetchMigrations();
    } catch (err) {
      setError(err.message);
    } finally {
      setSyncing(false);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getStateBadge = (state) => {
    switch (state) {
      case 'completed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-success/20 text-accent-success">
            <CheckCircle className="w-3 h-3" />
            Completed
          </span>
        );
      case 'running':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-brand-100 text-brand-800">
            <Loader2 className="w-3 h-3 animate-spin" />
            Running
          </span>
        );
      case 'queued':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface-2 text-text-secondary">
            <Clock className="w-3 h-3" />
            Queued
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-danger/20 text-accent-danger">
            <AlertCircle className="w-3 h-3" />
            Failed
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface-2 text-text-secondary">
            {state}
          </span>
        );
    }
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-20">
          <Loader2 className="w-8 h-8 text-brand-500 animate-spin" />
          <span className="ml-3 text-text-tertiary">Loading blueprint settings...</span>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-text-primary">Blueprint Course</h2>
        <p className="text-text-secondary mt-1">
          Manage blueprint template settings, associated courses, and sync history.
        </p>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertCircle className="w-4 h-4 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {success && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <CheckCircle className="w-4 h-4 flex-shrink-0" />
          {success}
          <button onClick={() => setSuccess(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-border-default mb-6">
        <nav className="flex space-x-8">
          <button
            onClick={() => setActiveTab('settings')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'settings'
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <span className="flex items-center gap-2">
              <Settings className="w-4 h-4" />
              Settings
            </span>
          </button>
          <button
            onClick={() => setActiveTab('associations')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'associations'
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <span className="flex items-center gap-2">
              <Link2 className="w-4 h-4" />
              Associated Courses
              {associations.length > 0 && (
                <span className="bg-surface-2 text-text-secondary text-xs px-2 py-0.5 rounded-full">
                  {associations.length}
                </span>
              )}
            </span>
          </button>
          <button
            onClick={() => setActiveTab('sync')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'sync'
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <span className="flex items-center gap-2">
              <RefreshCw className="w-4 h-4" />
              Sync History
              {migrations.length > 0 && (
                <span className="bg-surface-2 text-text-secondary text-xs px-2 py-0.5 rounded-full">
                  {migrations.length}
                </span>
              )}
            </span>
          </button>
        </nav>
      </div>

      {/* Settings Tab */}
      {activeTab === 'settings' && (
        <div className="bg-surface-0 rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold text-text-primary mb-4">Template Settings</h3>

          {template && (
            <div className="mb-4 text-sm text-text-tertiary">
              Template ID: {template.id} | Status: {template.workflow_state}
            </div>
          )}

          <form onSubmit={handleSaveTemplate} className="space-y-6">
            <div>
              <label className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={useDefaults}
                  onChange={(e) => setUseDefaults(e.target.checked)}
                  className="h-4 w-4 rounded border-border-strong text-brand-600 focus:ring-brand-500"
                />
                <div>
                  <span className="block text-sm font-medium text-text-primary">
                    Use default restrictions
                  </span>
                  <span className="block text-xs text-text-tertiary">
                    Apply the same content lock settings to all associated courses
                  </span>
                </div>
              </label>
            </div>

            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Default Restrictions (JSON)
              </label>
              <textarea
                value={restrictions}
                onChange={(e) => setRestrictions(e.target.value)}
                rows={4}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder='{"content": true, "points": true, "due_dates": false, "availability_dates": false}'
              />
              <p className="mt-1 text-xs text-text-tertiary">
                Define which content attributes are locked in associated courses. Keys: content, points, due_dates, availability_dates.
              </p>
            </div>

            <div className="flex items-center gap-3">
              <button
                type="submit"
                disabled={savingTemplate}
                className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Settings className="w-4 h-4" />
                {savingTemplate ? 'Saving...' : 'Save Settings'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Associations Tab */}
      {activeTab === 'associations' && (
        <div>
          {/* Add Course Form */}
          <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
            <h3 className="text-lg font-semibold text-text-primary mb-4">Add Associated Course</h3>
            <form onSubmit={handleAddCourse} className="flex items-end gap-4">
              <div className="flex-1">
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Course ID
                </label>
                <input
                  type="number"
                  value={newCourseId}
                  onChange={(e) => setNewCourseId(e.target.value)}
                  placeholder="Enter course ID to associate"
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  min="1"
                />
              </div>
              <button
                type="submit"
                disabled={addingCourse || !newCourseId.trim()}
                className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Plus className="w-4 h-4" />
                {addingCourse ? 'Adding...' : 'Add Course'}
              </button>
            </form>
          </div>

          {/* Associations List */}
          <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
            <div className="px-6 py-4 border-b border-border-default flex items-center justify-between">
              <h3 className="text-lg font-semibold text-text-primary">Associated Courses</h3>
              <button
                onClick={fetchAssociations}
                className="text-text-tertiary hover:text-text-secondary p-1"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
            </div>

            {associations.length === 0 ? (
              <div className="text-center py-12">
                <Link2 className="w-12 h-12 text-gray-300 mx-auto mb-4" />
                <h3 className="text-lg font-medium text-text-primary mb-1">No Associated Courses</h3>
                <p className="text-text-tertiary text-sm">
                  Add course IDs above to associate them with this blueprint.
                </p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-border-default">
                <thead className="bg-surface-1">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Subscription ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Child Course ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Added
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-surface-0 divide-y divide-border-default">
                  {associations.map((assoc) => (
                    <tr key={assoc.id} className="hover:bg-surface-1">
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-text-primary">
                        #{assoc.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-secondary">
                        Course #{assoc.child_course_id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-success/20 text-accent-success">
                          {assoc.workflow_state}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                        {formatDate(assoc.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        <button
                          onClick={() => handleRemoveCourse(assoc.child_course_id)}
                          className="text-accent-danger hover:text-accent-danger text-sm font-medium inline-flex items-center gap-1"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                          Remove
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {/* Sync History Tab */}
      {activeTab === 'sync' && (
        <div>
          {/* Sync Now Form */}
          <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
            <h3 className="text-lg font-semibold text-text-primary mb-4">Sync Now</h3>
            <p className="text-text-secondary text-sm mb-4">
              Push changes from this blueprint course to all associated courses.
            </p>
            <form onSubmit={handleSync} className="flex items-end gap-4">
              <div className="flex-1">
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Comment (optional)
                </label>
                <input
                  type="text"
                  value={syncComment}
                  onChange={(e) => setSyncComment(e.target.value)}
                  placeholder="Describe the changes being synced..."
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                />
              </div>
              <button
                type="submit"
                disabled={syncing}
                className="flex items-center gap-2 bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <RefreshCw className={`w-4 h-4 ${syncing ? 'animate-spin' : ''}`} />
                {syncing ? 'Syncing...' : 'Sync Now'}
              </button>
            </form>
          </div>

          {/* Migrations List */}
          <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
            <div className="px-6 py-4 border-b border-border-default flex items-center justify-between">
              <h3 className="text-lg font-semibold text-text-primary">Sync History</h3>
              <button
                onClick={fetchMigrations}
                className="text-text-tertiary hover:text-text-secondary p-1"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
            </div>

            {migrations.length === 0 ? (
              <div className="text-center py-12">
                <RefreshCw className="w-12 h-12 text-gray-300 mx-auto mb-4" />
                <h3 className="text-lg font-medium text-text-primary mb-1">No Sync History</h3>
                <p className="text-text-tertiary text-sm">
                  Trigger a sync above to push changes to associated courses.
                </p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-border-default">
                <thead className="bg-surface-1">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Comment
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Created
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Completed
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-surface-0 divide-y divide-border-default">
                  {migrations.map((migration) => (
                    <tr key={migration.id} className="hover:bg-surface-1">
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-text-primary">
                        #{migration.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {getStateBadge(migration.workflow_state)}
                      </td>
                      <td className="px-6 py-4 text-sm text-text-secondary max-w-xs truncate">
                        {migration.comment || '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                        {formatDate(migration.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                        {formatDate(migration.completed_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </Layout>
  );
};

export default BlueprintPage;
