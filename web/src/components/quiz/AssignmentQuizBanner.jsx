import React from 'react';
import { Link } from 'react-router-dom';
import { ClipboardList } from 'lucide-react';
import { useQuizForAssignment } from '../../services/api_quiz_engine';

// isQuizAssignment reports whether an assignment is the gradebook side of
// a quiz (submission_types "online_quiz").
export function isQuizAssignment(assignment) {
  if (!assignment) return false;
  const types = Array.isArray(assignment.submission_types)
    ? assignment.submission_types
    : String(assignment.submission_types || '').split(',').map((t) => t.trim());
  return types.includes('online_quiz');
}

/**
 * On an online_quiz assignment, send people to the quiz itself (G041):
 * students take it there, instructors edit dates and points there.
 */
export default function AssignmentQuizBanner({ courseId, assignment, isTeacher }) {
  const query = useQuizForAssignment(courseId, assignment?.id, isQuizAssignment(assignment));
  if (!query.data?.id) return null;
  const quiz = query.data;
  return (
    <div className="mb-6 flex flex-wrap items-center gap-3 rounded-lg border border-brand-200 bg-brand-50/60 p-4" role="note">
      <ClipboardList className="w-5 h-5 text-brand-600" aria-hidden="true" />
      <p className="text-sm text-text-primary flex-1 min-w-[12rem]">
        This assignment is the quiz <span className="font-semibold">{quiz.title}</span>.
        {isTeacher ? ' Change its questions, dates and points in the quiz editor.' : ' Your quiz score is recorded here.'}
      </p>
      <Link
        to={`/courses/${courseId}/quizzes/${quiz.id}/${isTeacher ? 'edit' : 'take'}`}
        className="inline-flex items-center px-3 py-1.5 rounded bg-brand-600 text-on-brand text-sm font-medium hover:bg-brand-700"
      >
        {isTeacher ? 'Edit quiz' : 'Go to quiz'}
      </Link>
    </div>
  );
}
