/**
 * Builds the dashboard's "Recent grades" rows for one course.
 *
 * Submissions from GET /courses/:id/students/submissions carry only
 * `assignment_id`, so the assignment title is resolved from the course's
 * assignment list (B207: every row used to fall back to "Assignment").
 */
export function buildRecentGrades(submissions, assignments, course) {
  const names = {};
  for (const a of assignments || []) names[a.id] = a.name;
  return (submissions || [])
    .filter((s) => s.workflow_state === 'graded' && s.score != null)
    .map((s) => ({
      id: s.id,
      assignment_id: s.assignment_id,
      title: names[s.assignment_id] || s.assignment_name || 'Assignment',
      score: s.score,
      graded_at: s.graded_at || s.updated_at,
      course_name: course?.name,
      course_id: course?.id,
    }));
}
