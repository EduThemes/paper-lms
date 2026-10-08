import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { FileText, Plus, Eye, EyeOff, Home, Trash2 } from 'lucide-react';
import { canDeletePage, canUnpublishPage } from '../utils/pageManagement';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import { clearRceDraft } from '../components/rce/rceDrafts';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { Skeleton } from '@/components/ui/skeleton';
import ListPager from '../components/ListPager';

const PAGES_PER_PAGE = 50;

const PagesPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [pages, setPages] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newPage, setNewPage] = useState({ title: '', body: '', published: true });
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  // B148: the API pages its results; page through them instead of
  // silently showing only the first page.
  const [page, setPage] = useState(1);
  const [pagination, setPagination] = useState({});

  useEffect(() => {
    let cancelled = false;
    const fetchPages = async () => {
      try {
        const result = await api.getPages(courseId, page, PAGES_PER_PAGE);
        if (cancelled) return;
        setPages(result.data || []);
        setPagination(result.pagination || {});
      } catch (err) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchPages();
    return () => { cancelled = true; };
  }, [courseId, page]);

  const doCreate = async () => {
    setCreating(true);
    try {
      const created = await api.createPage(courseId, {
        title: newPage.title,
        body: newPage.body,
        published: newPage.published,
      });
      clearRceDraft(`page-${courseId}-new-body`);
      setPages((prev) => [created, ...prev]);
      setNewPage({ title: '', body: '', published: true });
      setShowCreate(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  // G074: publish / unpublish and delete straight from the list.
  const [busyId, setBusyId] = useState(null);
  const [actionError, setActionError] = useState(null);

  const handleTogglePublished = async (target) => {
    setBusyId(target.page_id);
    setActionError(null);
    try {
      const updated = await api.updatePage(courseId, target.page_id, { published: !target.published });
      setPages((prev) => prev.map((p) => (p.page_id === target.page_id ? { ...p, ...(updated || {}) } : p)));
    } catch (err) {
      setActionError(err.message);
    } finally {
      setBusyId(null);
    }
  };

  const handleDeletePage = async (target) => {
    if (!window.confirm(`Delete "${target.title}"?`)) return;
    setBusyId(target.page_id);
    setActionError(null);
    try {
      await api.deletePage(courseId, target.page_id);
      setPages((prev) => prev.filter((p) => p.page_id !== target.page_id));
    } catch (err) {
      setActionError(err.message);
    } finally {
      setBusyId(null);
    }
  };

  const handleCreate = (e) => {
    e.preventDefault();
    checkAndSave(newPage.body, doCreate);
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
          <h2 className="text-2xl font-bold text-text-primary">Pages</h2>
          {isTeacher && (
            <button
              onClick={() => setShowCreate(!showCreate)}
              className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4" />
              <span>Page</span>
            </button>
          )}
        </div>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="bg-surface-0 rounded-lg shadow p-6 mb-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Title</label>
            <input
              type="text"
              required
              value={newPage.title}
              onChange={(e) => setNewPage({ ...newPage, title: e.target.value })}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              placeholder="Page title"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Content</label>
            <RichContentEditorV2
              value={newPage.body}
              onChange={(html) => setNewPage((prev) => ({ ...prev, body: html }))}
              placeholder="Page content..."
              minHeight="200px"
              courseId={courseId}
              autoSaveKey={`page-${courseId}-new-body`}
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="page-published"
              checked={newPage.published}
              onChange={(e) => setNewPage({ ...newPage, published: e.target.checked })}
              className="rounded border-border-strong"
            />
            <label htmlFor="page-published" className="text-sm text-text-secondary">Publish immediately</label>
          </div>
          <div className="flex justify-end space-x-3">
            <button type="button" onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
              Cancel
            </button>
            <button type="submit" disabled={creating} className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
              {creating ? 'Creating...' : 'Create Page'}
            </button>
          </div>
        </form>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">All Pages</h3>
          {actionError && <p role="alert" className="text-sm text-accent-danger mt-1">{actionError}</p>}
        </div>
        {pages.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No pages yet.</div>
        ) : (
          <div className="divide-y">
            {pages.map((page) => (
              <div key={page.page_id || page.url} className="flex items-center justify-between p-4 hover:bg-surface-1">
                <Link
                  to={`/courses/${courseId}/pages/${page.url}`}
                  className="flex items-center space-x-3 min-w-0 flex-1"
                >
                  <FileText className="w-5 h-5 text-text-disabled flex-shrink-0" />
                  <div className="min-w-0">
                    <span className="font-medium text-text-primary truncate block">
                      {page.title}
                      {page.front_page && (
                        <span className="ml-2 inline-flex items-center gap-1 text-xs font-medium text-brand-700 bg-brand-50 px-2 py-0.5 rounded-full align-middle">
                          <Home className="w-3 h-3" aria-hidden="true" /> Front Page
                        </span>
                      )}
                    </span>
                    {page.updated_at && (
                      <span className="text-xs text-text-disabled">
                        Last edited {formatDate(page.updated_at)}
                      </span>
                    )}
                    {page.todo_date && (
                      <span className="text-xs text-text-tertiary ml-2">To do {formatDate(page.todo_date)}</span>
                    )}
                  </div>
                </Link>
                {isTeacher && (
                  <div className="flex items-center gap-2 flex-shrink-0 ml-4">
                    <button
                      type="button"
                      onClick={() => handleTogglePublished(page)}
                      disabled={busyId === page.page_id || (page.published && !canUnpublishPage(page))}
                      aria-label={`${page.published ? 'Unpublish' : 'Publish'} ${page.title}`}
                      title={page.published && !canUnpublishPage(page) ? 'The front page must stay published' : undefined}
                      className={`inline-flex items-center space-x-1 text-xs px-2 py-0.5 rounded-full disabled:opacity-60 ${
                        page.published ? 'text-accent-success bg-accent-success/10' : 'text-text-tertiary bg-surface-2'
                      }`}
                    >
                      {page.published ? <Eye className="w-3 h-3" aria-hidden="true" /> : <EyeOff className="w-3 h-3" aria-hidden="true" />}
                      <span>{page.published ? 'Published' : 'Unpublished'}</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDeletePage(page)}
                      disabled={busyId === page.page_id || !canDeletePage(page)}
                      aria-label={`Delete ${page.title}`}
                      title={!canDeletePage(page) ? 'Choose another front page before deleting this one' : 'Delete page'}
                      className="p-1 text-text-disabled hover:text-accent-danger disabled:opacity-40"
                    >
                      <Trash2 className="w-4 h-4" aria-hidden="true" />
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
      <ListPager page={page} pagination={pagination} onPageChange={setPage} label="Pages pagination" />
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default PagesPage;
