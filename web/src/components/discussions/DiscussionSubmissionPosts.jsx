import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { MessageSquare, Paperclip } from 'lucide-react';
import RichContentViewer from '../RichContentViewer';
import { discussionsApi } from '../../services/api_discussions';
import { formatFileSize } from '../../utils/discussionMentions';

const when = (iso) => (iso ? new Date(iso).toLocaleString() : '');

/**
 * SpeedGrader view of a graded discussion submission: the student's
 * posts (replies marked as such), oldest first, with attachments and the
 * group of each post in a group discussion.
 */
export default function DiscussionSubmissionPosts({ courseId, assignmentId, userId }) {
  const query = useQuery({
    queryKey: ['discussions', String(courseId), 'studentEntries', String(assignmentId), String(userId)],
    queryFn: () => discussionsApi.studentEntries(courseId, assignmentId, userId),
    enabled: !!(courseId && assignmentId && userId),
  });

  if (query.isLoading) return <p className="text-sm text-text-tertiary">Loading discussion posts…</p>;
  if (query.isError) return <p className="text-sm text-accent-danger" role="alert">Could not load the discussion posts.</p>;

  const topic = query.data?.topic;
  const entries = query.data?.entries || [];

  return (
    <section aria-label="Discussion posts" className="space-y-3">
      {topic && (
        <p className="text-sm text-text-tertiary flex items-center gap-1.5">
          <MessageSquare className="w-4 h-4" aria-hidden="true" />
          Posts in{' '}
          <Link to={`/courses/${courseId}/discussions/${topic.id}`} className="text-brand-600 hover:underline">
            {topic.title}
          </Link>
        </p>
      )}
      {entries.length === 0 && <p className="text-sm text-text-secondary">This student has no posts.</p>}
      {entries.map((e) => (
        <article key={e.id} className="border border-border-default rounded-md p-3">
          <header className="flex flex-wrap items-center gap-2 text-xs text-text-tertiary mb-1">
            <span>{e.parent_id ? 'Reply' : 'Post'}</span>
            <span>{when(e.created_at)}</span>
            {e.group_name && <span className="px-1.5 rounded bg-surface-2">{e.group_name}</span>}
          </header>
          <RichContentViewer content={e.message} className="text-sm text-text-primary break-words" />
          {e.attachment?.url && (
            <a
              href={e.attachment.url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-2 inline-flex items-center gap-1.5 text-xs text-brand-600 hover:underline"
            >
              <Paperclip className="w-3.5 h-3.5" aria-hidden="true" />
              {e.attachment.display_name}
              <span className="text-text-tertiary">{formatFileSize(e.attachment.size)}</span>
            </a>
          )}
        </article>
      ))}
    </section>
  );
}
