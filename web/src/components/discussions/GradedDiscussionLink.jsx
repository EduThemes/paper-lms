import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { MessageSquare } from 'lucide-react';
import { discussionsApi } from '../../services/api_discussions';

/** True when an assignment's submission types include discussion_topic. */
export function isDiscussionAssignment(assignment) {
  if (!assignment) return false;
  const types = Array.isArray(assignment.submission_types)
    ? assignment.submission_types
    : String(assignment.submission_types || '').split(',');
  return types.map((t) => String(t).trim()).includes('discussion_topic');
}

/**
 * On a graded discussion's assignment page: students submit by posting,
 * so point them at the discussion instead of a submission form.
 */
export default function GradedDiscussionLink({ courseId, assignment }) {
  const enabled = isDiscussionAssignment(assignment);
  const query = useQuery({
    queryKey: ['discussions', String(courseId), 'assignmentTopic', String(assignment?.id)],
    queryFn: () => discussionsApi.assignmentTopic(courseId, assignment.id),
    enabled,
    retry: false,
  });
  if (!enabled || !query.data?.id) return null;
  return (
    <div className="mb-4 flex items-center gap-2 rounded-md border border-brand-200 bg-brand-50 px-4 py-3 text-sm text-brand-800">
      <MessageSquare className="w-4 h-4" aria-hidden="true" />
      <span>This is a graded discussion. Post in it to submit.</span>
      <Link to={`/courses/${courseId}/discussions/${query.data.id}`} className="ml-auto font-medium text-brand-700 hover:underline">
        Go to the discussion
      </Link>
    </div>
  );
}
