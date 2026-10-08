// The inbox "Submission Comments" mailbox (G092): comment threads on the
// student's own submissions, or on submissions in the courses a teacher
// teaches. Each row opens the place the thread is read and answered (the
// assignment page, or SpeedGrader for teachers).

import React, { useMemo } from 'react';
import { Link } from 'react-router-dom';
import { MessageSquare } from 'lucide-react';
import { useSubmissionCommentThreads } from '../services/queries_notify_inbox';
import { submissionThreadLink, threadsInContext } from '../utils/notifyInbox';

const formatDate = (s) => (s ? new Date(s).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : '');

const SubmissionCommentsMailbox = ({ filter = '' }) => {
  const query = useSubmissionCommentThreads();
  const threads = useMemo(() => {
    const seen = new Set();
    const out = [];
    (query.data?.pages || []).forEach((p) => (p.data || []).forEach((t) => {
      if (!seen.has(t.submission_id)) { seen.add(t.submission_id); out.push(t); }
    }));
    return threadsInContext(out, filter);
  }, [query.data, filter]);

  if (query.isLoading) {
    return <div className="p-6 text-sm text-text-tertiary">Loading submission comments...</div>;
  }
  if (query.isError) {
    return (
      <div className="p-6 text-sm text-accent-danger" role="alert">
        {query.error?.message || 'Could not load submission comments'}
        <button type="button" onClick={() => query.refetch()} className="ml-2 underline">Try again</button>
      </div>
    );
  }
  if (threads.length === 0) {
    return (
      <div className="p-6 text-center text-text-tertiary text-sm">
        <MessageSquare className="w-8 h-8 mx-auto mb-2 text-gray-300" aria-hidden="true" />
        No submission comments.
      </div>
    );
  }

  return (
    <ul className="divide-y divide-gray-100" aria-label="Submission comments">
      {threads.map((t) => (
        <li key={t.submission_id}>
          <Link to={submissionThreadLink(t)} className="block p-3 hover:bg-surface-1">
            <div className="flex items-start justify-between gap-2">
              <p className="text-sm font-medium text-text-primary truncate">
                {t.assignment_name}
                {!t.caller_is_student && t.student_name && <span className="text-text-tertiary font-normal"> &middot; {t.student_name}</span>}
              </p>
              <span className="text-xs text-text-disabled flex-shrink-0">{formatDate(t.last_comment_at)}</span>
            </div>
            <p className="text-xs text-text-tertiary truncate mt-0.5">
              {t.last_author_name ? `${t.last_author_name}: ` : ''}{t.last_comment}
            </p>
            <p className="text-xs text-text-tertiary mt-1">
              {t.course_name}
              {t.comment_count > 1 && <> &middot; {t.comment_count} comments</>}
            </p>
          </Link>
        </li>
      ))}
      {query.hasNextPage && (
        <li className="p-3 text-center">
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
            className="text-sm text-brand-600 hover:text-brand-800 font-medium disabled:opacity-50"
          >
            {query.isFetchingNextPage ? 'Loading...' : 'Load more'}
          </button>
        </li>
      )}
    </ul>
  );
};

export default SubmissionCommentsMailbox;
