import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { FileText, ArrowLeft, Pencil, Save, X } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import useUnsavedChanges from '../hooks/useUnsavedChanges';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentViewer, { sanitizeHTML } from '../components/RichContentViewer';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';

const PageDetailPage = () => {
  const { courseId, slug } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [page, setPage] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editForm, setEditForm] = useState({ title: '', body: '' });
  const [isDirty, setIsDirty] = useState(false);

  useUnsavedChanges(isDirty);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const data = await api.getPage(courseId, slug);
        setPage(data);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, slug]);

  const startEditing = () => {
    setEditForm({ title: page.title || '', body: page.body || '' });
    setEditing(true);
  };

  const doSave = async () => {
    setSaving(true);
    try {
      await api.updatePage(courseId, slug, {
        title: editForm.title,
        body: editForm.body,
      });
      setPage((prev) => ({ ...prev, title: editForm.title, body: editForm.body, updated_at: new Date().toISOString() }));
      setIsDirty(false);
      setEditing(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleSave = (e) => {
    e.preventDefault();
    checkAndSave(editForm.body, doSave);
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading page...
</div></Layout>;
  }
  if (error || !page) {
    return (
      <Layout>
        <CourseNav />
        <div className="text-center py-12">
          <FileText className="w-12 h-12 text-text-disabled mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-text-secondary mb-2">Page Not Found</h2>
          <p className="text-text-tertiary mb-4">This page doesn't exist or has been deleted.</p>
          <Link to={`/courses/${courseId}/pages`} className="text-brand-600 hover:underline">
            Back to Pages
          </Link>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-4">
        <Link to={`/courses/${courseId}/pages`} className="text-brand-600 hover:underline text-sm inline-flex items-center space-x-1">
          <ArrowLeft className="w-3 h-3" />
          <span>Back to Pages</span>
        </Link>
      </div>

      {editing ? (
        <form onSubmit={handleSave} className="bg-surface-0 rounded-lg shadow">
          <div className="p-6 border-b space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-text-primary">Edit Page</h3>
              <button type="button" onClick={() => { setEditing(false); setIsDirty(false); }} className="text-text-disabled hover:text-text-secondary">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Title</label>
              <input
                type="text"
                required
                value={editForm.title}
                onChange={(e) => { setEditForm({ ...editForm, title: e.target.value }); setIsDirty(true); }}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Content</label>
              <RichContentEditorV2
                value={editForm.body}
                onChange={(html) => { setEditForm((prev) => ({ ...prev, body: html })); setIsDirty(true); }}
                placeholder="Page content..."
                minHeight="300px"
                courseId={courseId}
                autoSaveKey={`page-${courseId}-${slug || 'new'}-body`}
              />
            </div>
          </div>
          <div className="p-4 flex justify-end space-x-3 bg-surface-1">
            <button type="button" onClick={() => { setEditing(false); setIsDirty(false); }} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="inline-flex items-center gap-1.5 px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              {saving ? 'Saving...' : 'Save Page'}
            </button>
          </div>
        </form>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow">
          <div className="p-6 border-b flex items-start justify-between">
            <div>
              <h1 className="text-2xl font-bold text-text-primary">{page.title}</h1>
              {page.updated_at && (
                <p className="text-sm text-text-disabled mt-1">
                  Last edited {new Date(page.updated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
                </p>
              )}
            </div>
            {isTeacher && (
              <button
                onClick={startEditing}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
              >
                <Pencil className="w-4 h-4" />
                Edit
              </button>
            )}
          </div>
          <div className="p-6">
            {page.body ? (
              <RichContentViewer content={page.body} />
            ) : (
              <p className="text-text-disabled italic">This page has no content yet.</p>
            )}
          </div>
        </div>
      )}
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default PageDetailPage;
