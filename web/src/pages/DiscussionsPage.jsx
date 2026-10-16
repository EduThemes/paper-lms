import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { MessageSquare, Pin, Plus, X, Edit2, Trash2, Search } from 'lucide-react';
import { api } from '../services/api';
import { discussionsApi } from '../services/api_discussions';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import DiscussionTopicForm from '../components/discussions/DiscussionTopicForm';
import { Skeleton } from '@/components/ui/skeleton';
import { topicStatusBadges } from '../utils/discussionTopicForm';

const PER_PAGE = 20;

const toneClass = {
  muted: 'bg-surface-2 text-text-tertiary',
  info: 'bg-brand-50 text-brand-700',
  success: 'bg-accent-success/15 text-accent-success',
};

const formatDate = (dateStr) => {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
};

function TopicRow({ topic, courseId, isTeacher, onEdit, onDelete, onTogglePublished }) {
  const replies = Number(topic.discussion_subentry_count || 0);
  const unread = Number(topic.unread_count || 0);
  const badges = topicStatusBadges(topic);
  const typeLabel = topic.discussion_type === 'side_comment'
    ? 'Side Comment'
    : topic.discussion_type === 'threaded'
      ? 'Threaded'
      : topic.discussion_type;

  return (
    <li className="flex items-center justify-between p-4 hover:bg-surface-1 group">
      <Link to={`/courses/${courseId}/discussions/${topic.id}`} className="flex items-start gap-3 min-w-0 flex-1">
        <MessageSquare className={`w-5 h-5 mt-0.5 flex-shrink-0 ${unread > 0 ? 'text-brand-500' : 'text-text-tertiary'}`} aria-hidden="true" />
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`text-text-primary truncate ${unread > 0 ? 'font-semibold' : 'font-medium'}`}>{topic.title}</span>
            {topic.pinned && <Pin className="w-3.5 h-3.5 text-brand-500 flex-shrink-0" aria-label="Pinned" />}
            {badges.map((b) => (
              <span key={b.key} className={`text-xs px-1.5 py-0.5 rounded flex-shrink-0 ${toneClass[b.tone] || toneClass.muted}`}>
                {b.label}
              </span>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-x-3 text-xs text-text-tertiary mt-0.5">
            {typeLabel && <span>{typeLabel}</span>}
            <span>{replies} {replies === 1 ? 'reply' : 'replies'}</span>
            {unread > 0 && (
              <span className="px-1.5 rounded-full bg-brand-500 text-on-brand font-semibold" aria-label={`${unread} unread`}>
                {unread} unread
              </span>
            )}
            {topic.last_reply_at && <span>Last post {formatDate(topic.last_reply_at)}</span>}
            {topic.assignment?.due_at && <span>Due {formatDate(topic.assignment.due_at)}</span>}
          </div>
        </div>
      </Link>
      <div className="flex items-center gap-2 flex-shrink-0 ml-4">
        <span className="text-xs text-text-tertiary hidden sm:inline">{formatDate(topic.created_at)}</span>
        {isTeacher && (
          <>
            <button
              type="button"
              onClick={() => onTogglePublished(topic)}
              className="px-2 py-0.5 text-xs rounded border border-border-default text-text-secondary hover:text-brand-600"
            >
              {topic.published === false ? 'Publish' : 'Unpublish'}
            </button>
            <button
              type="button"
              onClick={() => onEdit(topic)}
              className="p-1 text-text-tertiary hover:text-brand-600 opacity-0 group-hover:opacity-100 focus:opacity-100"
              aria-label={`Edit ${topic.title}`}
            >
              <Edit2 className="w-4 h-4" />
            </button>
            <button
              type="button"
              onClick={() => onDelete(topic)}
              className="p-1 text-text-tertiary hover:text-accent-danger opacity-0 group-hover:opacity-100 focus:opacity-100"
              aria-label={`Delete ${topic.title}`}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </>
        )}
      </div>
    </li>
  );
}

const DiscussionsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('all');
  const [editor, setEditor] = useState(null); // null | { id?: number }
  const [actionError, setActionError] = useState(null);

  // Debounce the search box; a new search starts at page 1.
  useEffect(() => {
    const t = setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  const query = useQuery({
    queryKey: ['discussions', String(courseId), 'index', { page, search, filter }],
    queryFn: () => discussionsApi.listTopics(courseId, { page, perPage: PER_PAGE, search, filter }),
    placeholderData: keepPreviousData,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['discussions', String(courseId)] });

  const handleDelete = async (topic) => {
    if (!window.confirm(`Delete "${topic.title}"?`)) return;
    try {
      await api.deleteDiscussionTopic(courseId, topic.id);
      setActionError(null);
      refresh();
    } catch (err) {
      setActionError(err.message);
    }
  };

  // Students can't open unpublished topics (e.g. ones a course copy or
  // import created), so teachers need a way to release them.
  const handleTogglePublished = async (topic) => {
    try {
      await discussionsApi.updateTopic(courseId, topic.id, { published: topic.published === false });
      setActionError(null);
      refresh();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const result = query.data;
  const topics = result?.topics || [];
  const lastPage = result?.lastPage || 1;
  const filtering = search !== '' || filter !== 'all';

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
              type="button"
              onClick={() => setEditor(editor ? null : {})}
              className="inline-flex items-center space-x-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              {editor ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
              <span>{editor ? 'Cancel' : 'New Discussion'}</span>
            </button>
          )}
        </div>
      </div>

      {editor && (
        <DiscussionTopicForm
          key={editor.id || 'new'}
          courseId={courseId}
          topicId={editor.id}
          onCancel={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            refresh();
          }}
        />
      )}

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <div className="relative flex-1 min-w-[12rem]">
          <Search className="w-4 h-4 text-text-tertiary absolute left-2.5 top-2.5" aria-hidden="true" />
          <input
            type="search"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search discussions"
            aria-label="Search discussions"
            className="w-full border border-border-strong rounded-md pl-8 pr-3 py-2 text-sm bg-surface-0 focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div className="inline-flex rounded-md border border-border-strong overflow-hidden" role="group" aria-label="Filter discussions">
          {[
            ['all', 'All'],
            ['unread', 'Unread'],
          ].map(([value, label]) => (
            <button
              key={value}
              type="button"
              onClick={() => {
                setFilter(value);
                setPage(1);
              }}
              aria-pressed={filter === value}
              className={`px-3 py-2 text-sm ${filter === value ? 'bg-brand-600 text-on-brand' : 'bg-surface-0 text-text-secondary hover:bg-surface-1'}`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {actionError && <p className="mb-3 text-sm text-accent-danger" role="alert">{actionError}</p>}

      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold">Topics</h3>
        </div>
        {query.isLoading ? (
          <div className="space-y-3 p-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </div>
        ) : query.isError ? (
          <div className="text-center py-12">
            <p className="text-accent-danger mb-3">{query.error?.message || 'Could not load discussions'}</p>
            <button type="button" onClick={() => query.refetch()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">
              Try Again
            </button>
          </div>
        ) : topics.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">
            {filtering ? 'No discussions match.' : 'No discussions yet.'}
          </div>
        ) : (
          <ul className="divide-y">
            {topics.map((topic) => (
              <TopicRow
                key={topic.id}
                topic={topic}
                courseId={courseId}
                isTeacher={!!isTeacher}
                onEdit={(t) => setEditor({ id: t.id })}
                onDelete={handleDelete}
                onTogglePublished={handleTogglePublished}
              />
            ))}
          </ul>
        )}
      </div>

      {(page > 1 || result?.hasNext) && (
        <nav aria-label="Discussion pages" className="flex items-center justify-between mt-4 text-sm">
          <button
            type="button"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            className="px-3 py-1.5 rounded-md border border-border-strong text-text-secondary hover:bg-surface-1 disabled:opacity-40"
          >
            Previous
          </button>
          <span className="text-text-tertiary" aria-live="polite">
            Page {page} of {Math.max(lastPage, page)}
          </span>
          <button
            type="button"
            onClick={() => setPage((p) => p + 1)}
            disabled={!result?.hasNext}
            className="px-3 py-1.5 rounded-md border border-border-strong text-text-secondary hover:bg-surface-1 disabled:opacity-40"
          >
            Next
          </button>
        </nav>
      )}
    </Layout>
  );
};

export default DiscussionsPage;
