import React, { useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, History, RotateCcw, Eye, GitCompare } from 'lucide-react';
import Page from '../components/Page';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentViewer from '../components/RichContentViewer';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import { pagesFilesApi } from '../services/api_pages_files';
import { diffRevisionText, diffStats } from '../utils/pageRevisionDiff';

const formatWhen = (iso) => (iso
  ? new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit' })
  : '');

function DiffView({ parts }) {
  if (!parts.length) return <p className="text-sm text-text-disabled italic">No text.</p>;
  return (
    <div className="whitespace-pre-wrap text-sm leading-relaxed text-text-secondary" data-testid="revision-diff">
      {parts.map((p, i) => {
        if (p.type === 'add') {
          return <ins key={i} className="bg-accent-success/20 text-accent-success no-underline rounded-sm">{p.text}</ins>;
        }
        if (p.type === 'del') {
          return <del key={i} className="bg-accent-danger/15 text-accent-danger rounded-sm">{p.text}</del>;
        }
        return <span key={i}>{p.text}</span>;
      })}
    </div>
  );
}

// PageHistoryPage lists a page's revisions (G065). Picking one shows it
// either as rendered content or as a word diff against the revision
// before it; course staff can restore it.
export default function PageHistoryPage() {
  const { courseId, slug } = useParams();
  const queryClient = useQueryClient();
  // Restoring needs course-staff rights (the server enforces it too).
  const isTeacher = useIsTeacher(courseId);
  const [selected, setSelected] = useState(null);
  const [mode, setMode] = useState('diff');
  const [message, setMessage] = useState(null);

  const pageQuery = useQuery({
    queryKey: ['courses', String(courseId), 'pages', slug],
    queryFn: () => api.getPage(courseId, slug),
  });
  const revisionsQuery = useQuery({
    queryKey: ['courses', String(courseId), 'pages', slug, 'revisions'],
    queryFn: () => pagesFilesApi.listPageRevisions(courseId, slug),
    enabled: Boolean(pageQuery.data?.can_edit),
  });

  const restore = useMutation({
    mutationFn: (revision) => pagesFilesApi.restorePageRevision(courseId, slug, revision),
    onSuccess: (rev) => {
      setMessage(`Restored. The page now matches revision ${selected}; this was saved as revision ${rev?.revision_id}.`);
      setSelected(rev?.revision_id ?? null);
      queryClient.invalidateQueries({ queryKey: ['courses', String(courseId), 'pages', slug] });
    },
    onError: (err) => setMessage(err.message),
  });

  const revisions = revisionsQuery.data || [];
  const current = revisions.find((r) => r.revision_id === selected) || revisions[0];
  const previous = current ? revisions.find((r) => r.revision_id < current.revision_id) : null;
  const diff = useMemo(
    () => (current ? diffRevisionText(previous?.body || '', current.body || '') : []),
    [current, previous],
  );
  const stats = diffStats(diff);

  return (
    <Page query={pageQuery} title="Page History">
      {(page) => (
        <Layout>
          <CourseNav />
          <div className="mb-4">
            <Link to={`/courses/${courseId}/pages/${slug}`} className="text-brand-600 hover:underline text-sm inline-flex items-center gap-1">
              <ArrowLeft className="w-3 h-3" aria-hidden="true" />
              Back to {page.title}
            </Link>
          </div>
          <h1 className="text-2xl font-bold text-text-primary mb-4 flex items-center gap-2">
            <History className="w-6 h-6 text-text-tertiary" aria-hidden="true" />
            {page.title}: history
          </h1>

          {!page.can_edit ? (
            <p className="text-text-tertiary">You don&apos;t have permission to view this page&apos;s history.</p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-[18rem_1fr] gap-4">
              <nav aria-label="Revisions" className="bg-surface-0 rounded-lg shadow divide-y max-h-[70vh] overflow-y-auto">
                {revisionsQuery.isLoading && <p className="p-4 text-sm text-text-disabled">Loading revisions…</p>}
                {revisionsQuery.isError && <p className="p-4 text-sm text-accent-danger">{revisionsQuery.error.message}</p>}
                {revisions.map((rev) => {
                  const active = current && rev.revision_id === current.revision_id;
                  return (
                    <button
                      key={rev.revision_id}
                      type="button"
                      onClick={() => { setSelected(rev.revision_id); setMessage(null); }}
                      aria-current={active ? 'true' : undefined}
                      className={`w-full text-left px-4 py-3 text-sm hover:bg-surface-1 ${active ? 'bg-brand-50' : ''}`}
                    >
                      <span className="font-medium text-text-primary">Revision {rev.revision_id}</span>
                      {rev.latest && <span className="ml-2 text-xs text-accent-success">(current)</span>}
                      <span className="block text-xs text-text-tertiary">{formatWhen(rev.updated_at)}</span>
                    </button>
                  );
                })}
              </nav>

              <section className="bg-surface-0 rounded-lg shadow" aria-live="polite">
                {current ? (
                  <>
                    <div className="p-4 border-b flex flex-wrap items-center gap-2 justify-between">
                      <div>
                        <h2 className="font-semibold text-text-primary">{current.title}</h2>
                        <p className="text-xs text-text-tertiary">
                          Revision {current.revision_id} · {formatWhen(current.updated_at)}
                          {previous && ` · +${stats.added} / −${stats.removed} words vs. revision ${previous.revision_id}`}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <div role="group" aria-label="View mode" className="inline-flex rounded-md border border-border-default overflow-hidden">
                          <button type="button" onClick={() => setMode('diff')} aria-pressed={mode === 'diff'}
                            className={`px-3 py-1.5 text-sm inline-flex items-center gap-1 ${mode === 'diff' ? 'bg-surface-2 font-medium' : ''}`}>
                            <GitCompare className="w-4 h-4" aria-hidden="true" /> Changes
                          </button>
                          <button type="button" onClick={() => setMode('view')} aria-pressed={mode === 'view'}
                            className={`px-3 py-1.5 text-sm inline-flex items-center gap-1 ${mode === 'view' ? 'bg-surface-2 font-medium' : ''}`}>
                            <Eye className="w-4 h-4" aria-hidden="true" /> Content
                          </button>
                        </div>
                        {isTeacher && !current.latest && (
                          <button
                            type="button"
                            disabled={restore.isPending}
                            onClick={() => {
                              if (window.confirm(`Restore revision ${current.revision_id}? The current content stays in the history.`)) {
                                restore.mutate(current.revision_id);
                              }
                            }}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50"
                          >
                            <RotateCcw className="w-4 h-4" aria-hidden="true" />
                            {restore.isPending ? 'Restoring…' : 'Restore this revision'}
                          </button>
                        )}
                      </div>
                    </div>
                    {message && <p className="px-4 pt-3 text-sm text-text-secondary" role="status">{message}</p>}
                    <div className="p-4">
                      {mode === 'diff' ? (
                        previous ? <DiffView parts={diff} /> : <p className="text-sm text-text-tertiary mb-2">First revision: nothing to compare with.</p>
                      ) : null}
                      {(mode === 'view' || !previous) && (
                        current.body ? <RichContentViewer content={current.body} /> : <p className="text-text-disabled italic">Empty page.</p>
                      )}
                    </div>
                  </>
                ) : (
                  !revisionsQuery.isLoading && <p className="p-6 text-text-tertiary">No revisions yet.</p>
                )}
              </section>
            </div>
          )}
        </Layout>
      )}
    </Page>
  );
}
