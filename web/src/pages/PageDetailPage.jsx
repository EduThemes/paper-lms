import React, { useState, useEffect } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { FileText, ArrowLeft, Pencil, Save, X, Home, History, Trash2, Eye, EyeOff, CalendarCheck, Paintbrush, Code2 } from 'lucide-react';
import {
  EDITING_ROLE_OPTIONS, editingRolesLabel, canDeletePage, canUnpublishPage, editFormFromPage, buildPageUpdate,
} from '../utils/pageManagement';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import useUnsavedChanges from '../hooks/useUnsavedChanges';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentViewer, { sanitizeHTML } from '../components/RichContentViewer';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import { clearRceDraft } from '../components/rce/rceDrafts';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { useCourseUI } from '../contexts/CourseUIContext';
import DesignedPageView, { PageContent } from '../components/pages/DesignedPageView';
import PageCssPanel from '../components/pages/PageCssPanel';
import DetachDesignDialog from '../components/pages/DetachDesignDialog';
import { isDesignedPage, designerPath, pageCssSource } from '../utils/pageDesign';

const PageDetailPage = () => {
  const { courseId, slug } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [page, setPage] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  // Errors from save / publish / delete: shown inline, unlike a load
  // error, which replaces the page with "not found".
  const [actionError, setActionError] = useState(null);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editForm, setEditForm] = useState(editFormFromPage(null));
  const [isDirty, setIsDirty] = useState(false);
  // Designed pages (Page Designer) are edited in the designer. "Edit as
  // HTML" opens a plain HTML editor instead, after a confirm, and saving
  // detaches the design (design: null). They never open in the rich text
  // editor, which would rewrite their markup (block links, empty layout
  // boxes).
  const [htmlMode, setHtmlMode] = useState(false);
  const [detachOpen, setDetachOpen] = useState(false);
  const { course } = useCourseUI();
  const navigate = useNavigate();
  // G074: students may edit pages whose editing roles admit them; the
  // server reports it as can_edit.
  const canEdit = isTeacher || Boolean(page?.can_edit);
  const designed = isDesignedPage(page);
  // Page CSS (wiki_page.css): teachers, when the account allows
  // hand-written CSS, on ordinary pages (a designed page's CSS belongs to
  // the designer; once detached it is an ordinary page's CSS).
  const showCssPanel = Boolean(isTeacher && course?.custom_css_enabled && (!designed || htmlMode));

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

  const startEditing = ({ asHtml = false } = {}) => {
    // Never open a designed page in the rich text editor; go through the
    // detach confirm instead.
    if (isDesignedPage(page) && !asHtml) {
      setDetachOpen(true);
      return;
    }
    setEditForm({ ...editFormFromPage(page), css: pageCssSource(page) });
    setHtmlMode(asHtml);
    setEditing(true);
  };

  const confirmDetach = () => {
    setDetachOpen(false);
    startEditing({ asHtml: true });
  };

  const stopEditing = () => {
    setEditing(false);
    setHtmlMode(false);
    setIsDirty(false);
  };

  const doSave = async () => {
    setSaving(true);
    try {
      const update = buildPageUpdate(editForm, { isTeacher });
      if (htmlMode) update.design = null;
      if (showCssPanel && (editForm.css || '') !== pageCssSource(page)) update.css = editForm.css || '';
      const saved = await api.updatePage(courseId, slug, update);
      clearRceDraft(`page-${courseId}-${slug || 'new'}-body`);
      // Show what the server stored (it sanitizes the body), not the local
      // editor HTML — otherwise anything the server drops looks saved until
      // the next reload (B102).
      const detached = htmlMode ? { design: null, has_design: false } : {};
      setPage((prev) => (saved && typeof saved === 'object' && 'body' in saved
        ? { ...prev, ...saved, ...detached }
        : { ...prev, title: editForm.title, body: editForm.body, updated_at: new Date().toISOString(), ...detached }));
      stopEditing();
    } catch (err) {
      setActionError(err.message);
    } finally {
      setSaving(false);
    }
  };

  // B147: the course home "Front Page" view shows the page marked here.
  // Canvas requires the front page to be published, so publish it too.
  const handleSetFrontPage = async () => {
    setSaving(true);
    try {
      const updated = await api.updatePage(courseId, slug, { front_page: true, published: true });
      setPage((prev) => ({ ...prev, ...(updated || {}), front_page: true }));
    } catch (err) {
      setActionError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleTogglePublished = async () => {
    setSaving(true);
    try {
      const updated = await api.updatePage(courseId, slug, { published: !page.published });
      setPage((prev) => ({ ...prev, ...(updated || {}) }));
    } catch (err) {
      setActionError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Delete "${page.title}"? Students will no longer see it.`)) return;
    setSaving(true);
    try {
      await api.deletePage(courseId, slug);
      navigate(`/courses/${courseId}/pages`);
    } catch (err) {
      setActionError(err.message);
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
              <h3 className="text-lg font-semibold text-text-primary">{htmlMode ? 'Edit Page HTML' : 'Edit Page'}</h3>
              <button type="button" onClick={stopEditing} aria-label="Close editor" className="text-text-disabled hover:text-text-secondary">
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
            {htmlMode ? (
              <div>
                <label htmlFor="page-html" className="block text-sm font-medium text-text-secondary mb-1">HTML</label>
                <textarea
                  id="page-html"
                  value={editForm.body}
                  onChange={(e) => { setEditForm({ ...editForm, body: e.target.value }); setIsDirty(true); }}
                  aria-describedby="page-html-help"
                  spellCheck={false}
                  rows={20}
                  className="w-full border border-border-strong rounded-md px-3 py-2 font-mono text-xs leading-relaxed bg-surface-0 text-text-primary"
                />
                <p id="page-html-help" className="text-xs text-text-tertiary mt-1">
                  Saving detaches this page from its design. Its look stays; the designer can no longer open it.
                </p>
              </div>
            ) : (
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
            )}
            {isTeacher && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label htmlFor="page-editing-roles" className="block text-sm font-medium text-text-secondary mb-1">Users allowed to edit this page</label>
                  <select
                    id="page-editing-roles"
                    value={editForm.editingRoles}
                    onChange={(e) => { setEditForm({ ...editForm, editingRoles: e.target.value }); setIsDirty(true); }}
                    className="w-full border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-0"
                  >
                    {EDITING_ROLE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                  </select>
                </div>
                <div>
                  <label htmlFor="page-todo-date" className="block text-sm font-medium text-text-secondary mb-1">Add to student to-do</label>
                  <input
                    id="page-todo-date"
                    type="datetime-local"
                    value={editForm.todoDate}
                    onChange={(e) => { setEditForm({ ...editForm, todoDate: e.target.value }); setIsDirty(true); }}
                    className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
                  />
                  <p className="text-xs text-text-tertiary mt-1">Leave empty for no to-do date.</p>
                </div>
              </div>
            )}
            {showCssPanel && (
              <PageCssPanel
                value={editForm.css || ''}
                onChange={(css) => { setEditForm((prev) => ({ ...prev, css })); setIsDirty(true); }}
              />
            )}
            {actionError && <p role="alert" className="text-sm text-accent-danger">{actionError}</p>}
          </div>
          <div className="p-4 flex justify-end space-x-3 bg-surface-1">
            <button type="button" onClick={stopEditing} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
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
              {page.todo_date && (
                <p className="text-sm text-text-secondary mt-1 inline-flex items-center gap-1">
                  <CalendarCheck className="w-4 h-4" aria-hidden="true" />
                  To do: {new Date(page.todo_date).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })}
                </p>
              )}
              {isTeacher && (
                <p className="text-xs text-text-tertiary mt-1">Can edit: {editingRolesLabel(page.editing_roles)}</p>
              )}
            </div>
            {!isTeacher && canEdit && (
              <div className="flex items-center gap-2">
                <Link
                  to={`/courses/${courseId}/pages/${slug}/revisions`}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
                >
                  <History className="w-4 h-4" aria-hidden="true" />
                  History
                </Link>
                {!designed && (
                  <button
                    onClick={() => startEditing()}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
                  >
                    <Pencil className="w-4 h-4" aria-hidden="true" />
                    Edit
                  </button>
                )}
              </div>
            )}
            {isTeacher && (
              <div className="flex flex-wrap items-center justify-end gap-2">
                <button
                  onClick={handleTogglePublished}
                  disabled={saving || (page.published && !canUnpublishPage(page))}
                  title={page.published && !canUnpublishPage(page) ? 'The front page must stay published' : undefined}
                  aria-pressed={Boolean(page.published)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium disabled:opacity-50 ${
                    page.published ? 'bg-accent-success/10 text-accent-success' : 'bg-surface-2 text-text-secondary hover:bg-border-default'
                  }`}
                >
                  {page.published ? <Eye className="w-4 h-4" aria-hidden="true" /> : <EyeOff className="w-4 h-4" aria-hidden="true" />}
                  {page.published ? 'Published' : 'Unpublished'}
                </button>
                <Link
                  to={`/courses/${courseId}/pages/${slug}/revisions`}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
                >
                  <History className="w-4 h-4" aria-hidden="true" />
                  History
                </Link>
                <button
                  onClick={handleDelete}
                  disabled={saving || !canDeletePage(page)}
                  title={!canDeletePage(page) ? 'Choose another front page before deleting this one' : undefined}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-accent-danger hover:bg-border-default disabled:opacity-50"
                >
                  <Trash2 className="w-4 h-4" aria-hidden="true" />
                  Delete
                </button>
                {page.front_page ? (
                  <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-brand-50 text-brand-700">
                    <Home className="w-3 h-3" aria-hidden="true" />
                    Front Page
                  </span>
                ) : (
                  <button
                    onClick={handleSetFrontPage}
                    disabled={saving}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default disabled:opacity-50"
                  >
                    <Home className="w-4 h-4" aria-hidden="true" />
                    Use as Front Page
                  </button>
                )}
                {designed ? (
                  <>
                    <button
                      type="button"
                      onClick={() => setDetachOpen(true)}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
                    >
                      <Code2 className="w-4 h-4" aria-hidden="true" />
                      Edit as HTML
                    </button>
                    <Link
                      to={designerPath(courseId, { page: slug })}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700"
                    >
                      <Paintbrush className="w-4 h-4" aria-hidden="true" />
                      Edit design
                    </Link>
                  </>
                ) : (
                  <button
                    onClick={() => startEditing()}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default"
                  >
                    <Pencil className="w-4 h-4" />
                    Edit
                  </button>
                )}
              </div>
            )}
          </div>
          {actionError && <p role="alert" className="px-6 pt-4 text-sm text-accent-danger">{actionError}</p>}
          {designed ? (
            // A designed page brings its own layout and colours: edge to
            // edge under the header, no prose wrapper.
            <div className="overflow-hidden rounded-b-lg">
              <DesignedPageView page={page} />
            </div>
          ) : (
            <div className="p-6">
              {page.body ? (
                <PageContent page={page} fallback={<RichContentViewer content={page.body} />} />
              ) : (
                <p className="text-text-disabled italic">This page has no content yet.</p>
              )}
            </div>
          )}
        </div>
      )}
      <DetachDesignDialog open={detachOpen} onOpenChange={setDetachOpen} onConfirm={confirmDetach} />
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default PageDetailPage;
