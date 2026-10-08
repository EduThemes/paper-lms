import React from 'react';
import { useAuth } from '../contexts/AuthContext';
import CommentThread from './grading/CommentThread';

// Comments on one student's submission (by user id): text, files,
// audio/video recordings, group comments and grader drafts. See
// grading/CommentThread for the anonymous-id variant SpeedGrader uses.
const SubmissionComments = ({ courseId, assignmentId, userId, isTeacher, isGroupAssignment = false }) => {
  const { user } = useAuth();
  return (
    <CommentThread
      courseId={courseId}
      assignmentId={assignmentId}
      submissionKey={{ userId }}
      isTeacher={!!isTeacher}
      currentUserId={user?.id ?? null}
      isGroupAssignment={isGroupAssignment}
    />
  );
};

export default SubmissionComments;
