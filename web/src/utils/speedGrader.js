// SpeedGrader helpers: how a student row is addressed (user id, or the
// anonymous id while students are anonymized), status filters and sort
// orders for the student list, and merging a graded submission back
// into the row.

// studentKey is the stable list key of a SpeedGrader row.
export const studentKey = (student) =>
  student?.anonymous_id ? `anon:${student.anonymous_id}` : `user:${student?.user_id}`;

// submissionKeyOf is the API address of the row's submission.
export const submissionKeyOf = (student) =>
  student?.anonymous_id ? { anonymousId: student.anonymous_id } : { userId: student?.user_id };

// studentStatus is the late-policy status Canvas shows: a grader-set
// late_policy_status wins over the computed late / missing flags.
export const studentStatus = (submission) => {
  if (!submission) return 'none';
  const set = submission.late_policy_status;
  if (set && set !== 'none') return set;
  if (set === 'none') return 'none';
  if (submission.missing) return 'missing';
  if (submission.late) return 'late';
  return 'none';
};

const isSubmitted = (sub) => !!sub && !!sub.submitted_at;
const isGraded = (sub) => !!sub && (sub.excused || sub.workflow_state === 'graded' || (sub.score !== null && sub.score !== undefined));

export const STATUS_FILTERS = [
  { value: 'all', label: 'All students' },
  { value: 'needs_grading', label: 'Needs grading' },
  { value: 'ungraded', label: 'Not graded' },
  { value: 'graded', label: 'Graded' },
  { value: 'not_submitted', label: 'Not submitted' },
  { value: 'late', label: 'Late' },
  { value: 'missing', label: 'Missing' },
  { value: 'excused', label: 'Excused' },
];

export const SORT_OPTIONS = [
  { value: 'name', label: 'Name' },
  { value: 'submitted_at', label: 'Submission date' },
  { value: 'status', label: 'Submission status' },
];

export const filterStudents = (students, filter) => {
  const list = Array.isArray(students) ? students : [];
  switch (filter) {
    case 'needs_grading':
      return list.filter((s) => {
        const sub = s.submission;
        return isSubmitted(sub) && !sub.excused && (sub.workflow_state === 'submitted' || sub.workflow_state === 'pending_review' || sub.score == null);
      });
    case 'ungraded':
      return list.filter((s) => !isGraded(s.submission));
    case 'graded':
      return list.filter((s) => isGraded(s.submission));
    case 'not_submitted':
      return list.filter((s) => !isSubmitted(s.submission));
    case 'late':
      return list.filter((s) => studentStatus(s.submission) === 'late');
    case 'missing':
      return list.filter((s) => studentStatus(s.submission) === 'missing');
    case 'excused':
      return list.filter((s) => !!s.submission?.excused);
    default:
      return list;
  }
};

const statusRank = (sub) => {
  if (!isSubmitted(sub)) return 3;
  if (sub.excused) return 2;
  if (isGraded(sub)) return 1;
  return 0; // needs grading first
};

// sortStudents orders the list. Anonymous rows keep the server's order
// (it is already shuffled per assignment) when sorting by name, so the
// list never hints at who is who.
export const sortStudents = (students, sortBy, { groupBy = false, anonymous = false } = {}) => {
  const list = [...(Array.isArray(students) ? students : [])];
  const byName = (a, b) => String(a.user_name || '').localeCompare(String(b.user_name || ''), undefined, { numeric: true });
  let cmp;
  if (sortBy === 'submitted_at') {
    cmp = (a, b) => {
      const ta = a.submission?.submitted_at ? new Date(a.submission.submitted_at).getTime() : Infinity;
      const tb = b.submission?.submitted_at ? new Date(b.submission.submitted_at).getTime() : Infinity;
      return ta - tb || byName(a, b);
    };
  } else if (sortBy === 'status') {
    cmp = (a, b) => statusRank(a.submission) - statusRank(b.submission) || byName(a, b);
  } else {
    cmp = anonymous ? () => 0 : byName;
  }
  if (groupBy) {
    const inner = cmp;
    cmp = (a, b) => String(a.group_name || '￿').localeCompare(String(b.group_name || '￿')) || inner(a, b);
  }
  return list.sort(cmp);
};

const GRADE_FIELDS = [
  'score', 'grade', 'entered_score', 'entered_grade', 'points_deducted', 'excused', 'workflow_state',
  'late', 'missing', 'custom_grade_status_id', 'late_policy_status', 'extra_attempts', 'graded_at',
  'posted_at', 'grader_id',
];

// mergeGradedSubmission copies the grade/status fields of the server's
// answer onto the row's submission, keeping the rendered content (the
// anonymous file links, body, attempt) untouched.
export const mergeGradedSubmission = (current, saved) => {
  if (!saved || typeof saved !== 'object') return current;
  const out = { ...(current || {}) };
  for (const f of GRADE_FIELDS) {
    if (f in saved) out[f] = saved[f];
  }
  return out;
};
