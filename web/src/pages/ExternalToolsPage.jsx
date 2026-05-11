import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Puzzle, Plus, Trash2, Pencil, AlertTriangle, X, ArrowLeft } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const ExternalToolsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [course, setCourse] = useState(null);
  const [tools, setTools] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingTool, setEditingTool] = useState(null);
  const [saving, setSaving] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const [courseData, toolsResult] = await Promise.all([
        api.getCourse(courseId),
        api.getExternalTools(courseId, 1, 100),
      ]);
      setCourse(courseData);
      setTools(Array.isArray(toolsResult.data) ? toolsResult.data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const resetForm = () => {
    setShowForm(false);
    setEditingTool(null);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    const formData = new FormData(e.target);

    let customFields = undefined;
    const customFieldsRaw = formData.get('custom_fields');
    if (customFieldsRaw && customFieldsRaw.trim()) {
      try {
        customFields = JSON.parse(customFieldsRaw);
      } catch {
        setError('Custom fields must be valid JSON.');
        setSaving(false);
        return;
      }
    }

    const externalTool = {
      name: formData.get('name'),
      developer_key_id: formData.get('developer_key_id') || undefined,
      url: formData.get('url') || undefined,
      description: formData.get('description') || undefined,
      custom_fields: customFields,
    };

    // Remove undefined fields
    Object.keys(externalTool).forEach(
      (k) => externalTool[k] === undefined && delete externalTool[k]
    );

    try {
      if (editingTool) {
        await api.updateExternalTool(courseId, editingTool.id, externalTool);
      } else {
        await api.createExternalTool(courseId, externalTool);
      }
      resetForm();
      fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleEdit = (tool) => {
    setEditingTool(tool);
    setShowForm(true);
  };

  const handleDelete = async (toolId) => {
    setDeleting(true);
    setError(null);
    try {
      await api.deleteExternalTool(courseId, toolId);
      setDeleteConfirm(null);
      fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm flex items-center gap-1 mb-2">
          <ArrowLeft className="w-3 h-3" />
          Back to Course
        </Link>
        <div className="flex justify-between items-center">
          <div>
            <h2 className="text-2xl font-bold text-text-primary">
              External Tools{course ? `: ${course.name}` : ''}
            </h2>
            <p className="text-text-secondary mt-1">
              Manage LTI tools installed in this course.
            </p>
          </div>
          {isTeacher && (
            <button
              onClick={() => {
                setEditingTool(null);
                setShowForm(!showForm);
              }}
              className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
            >
              <Plus className="w-4 h-4" />
              Install New Tool
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Create / Edit Form */}
      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">
            {editingTool ? 'Edit External Tool' : 'Install New External Tool'}
          </h3>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Name <span className="text-accent-danger">*</span>
                </label>
                <input
                  name="name"
                  required
                  defaultValue={editingTool?.name || ''}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="My LTI Tool"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Developer Key ID
                </label>
                <input
                  name="developer_key_id"
                  defaultValue={editingTool?.developer_key_id || ''}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="e.g., 10000000000001"
                />
                <p className="text-xs text-text-tertiary mt-1">The developer key ID for LTI 1.3 tools.</p>
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">URL</label>
              <input
                name="url"
                type="url"
                defaultValue={editingTool?.url || ''}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://tool-provider.example.com/lti/launch"
              />
              <p className="text-xs text-text-tertiary mt-1">The launch URL or configuration URL for the tool.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
              <textarea
                name="description"
                rows={2}
                defaultValue={editingTool?.description || ''}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="A brief description of this tool..."
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Custom Fields (JSON)</label>
              <textarea
                name="custom_fields"
                rows={4}
                defaultValue={
                  editingTool?.custom_fields
                    ? JSON.stringify(editingTool.custom_fields, null, 2)
                    : ''
                }
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder={'{\n  "custom_param": "value"\n}'}
              />
              <p className="text-xs text-text-tertiary mt-1">
                Optional JSON object of custom parameters to send with each launch.
              </p>
            </div>
            <div className="flex gap-3">
              <button
                type="submit"
                disabled={saving}
                className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                {saving
                  ? editingTool
                    ? 'Updating...'
                    : 'Installing...'
                  : editingTool
                  ? 'Update Tool'
                  : 'Install Tool'}
              </button>
              <button
                type="button"
                onClick={resetForm}
                className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Delete Confirmation */}
      {deleteConfirm !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-surface-0 rounded-lg shadow-xl p-6 max-w-sm mx-4">
            <h3 className="font-semibold text-text-primary mb-2">Remove External Tool</h3>
            <p className="text-sm text-text-secondary mb-4">
              Are you sure you want to remove this tool from the course? Students will no longer be able to access it.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setDeleteConfirm(null)}
                className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
                disabled={deleting}
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteConfirm)}
                disabled={deleting}
                className="bg-accent-danger text-white px-4 py-2 rounded-md hover:bg-accent-danger/90 text-sm disabled:opacity-50"
              >
                {deleting ? 'Removing...' : 'Remove Tool'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Tools List */}
      {loading ? (
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading external tools...
</div>
      ) : tools.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Puzzle className="w-12 h-12 text-text-disabled mx-auto mb-4" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No External Tools Installed</h3>
          <p className="text-text-tertiary text-sm">
            No LTI tools have been installed in this course yet.
          </p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-border-default">
            <thead className="bg-surface-1">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Description
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  URL
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  State
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Installed
                </th>
                {isTeacher && (
                  <th className="px-6 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">
                    Actions
                  </th>
                )}
              </tr>
            </thead>
            <tbody className="bg-surface-0 divide-y divide-border-default">
              {tools.map((tool) => (
                <tr key={tool.id} className="hover:bg-surface-1">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center gap-2">
                      <Puzzle className="w-4 h-4 text-purple-400 flex-shrink-0" />
                      <span className="text-sm font-medium text-text-primary">
                        {tool.name || 'Unnamed Tool'}
                      </span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="text-sm text-text-secondary line-clamp-2">
                      {tool.description || '-'}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <span className="text-sm text-text-secondary break-all max-w-xs block truncate" title={tool.url}>
                      {tool.url || '-'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`text-xs px-2 py-1 rounded-full ${
                        tool.workflow_state === 'public'
                          ? 'bg-accent-success/20 text-accent-success'
                          : tool.workflow_state === 'active'
                          ? 'bg-accent-success/20 text-accent-success'
                          : tool.workflow_state === 'disabled'
                          ? 'bg-surface-2 text-text-secondary'
                          : 'bg-accent-warning/20 text-accent-warning'
                      }`}
                    >
                      {tool.workflow_state || 'active'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                    {formatDate(tool.created_at)}
                  </td>
                  {isTeacher && (
                    <td className="px-6 py-4 whitespace-nowrap text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => handleEdit(tool)}
                          className="text-text-tertiary hover:text-brand-600 p-1"
                          title="Edit tool"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => setDeleteConfirm(tool.id)}
                          className="text-accent-danger hover:text-accent-danger p-1"
                          title="Remove tool"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Layout>
  );
};

export default ExternalToolsPage;
