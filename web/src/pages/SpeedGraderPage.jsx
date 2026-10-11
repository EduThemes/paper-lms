import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import {
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronUp,
  CheckCircle,
  Clock,
  AlertCircle,
  MinusCircle,
  Award,
  FileText,
  Grid,
  History,
  Users,
  Scale,
} from 'lucide-react';
import { api } from '../services/api';
import gradingApi from '../services/api_grading_ui';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import { SubmissionBody } from '../components/SafeHTML';
import { useLiveRegion } from '../components/LiveRegion';
import { Card } from '../components/ui/card';
import { Avatar, AvatarFallback } from '../components/ui/avatar';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Separator } from '../components/ui/separator';
import { Skeleton } from '../components/ui/skeleton';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '../components/ui/tooltip';
import { normalizeRubricScores } from '../utils/grading';
import DiscussionSubmissionPosts from '../components/discussions/DiscussionSubmissionPosts';
import CommentThread from '../components/grading/CommentThread';
import AttachmentPreview from '../components/grading/AttachmentPreview';
import DocumentViewer from '../components/DocumentViewer';
import { DocumentImage } from '../components/grading/AnnotatableDocument';
import {
  studentKey,
  submissionKeyOf,
  filterStudents,
  sortStudents,
  studentStatus,
  STATUS_FILTERS,
  SORT_OPTIONS,
  mergeGradedSubmission,
} from '../utils/speedGrader';

const STATUS_CONFIG = {
  submitted: { label: 'Submitted', color: 'bg-brand-100 text-brand-800', icon: CheckCircle, dot: 'bg-brand-500' },
  graded: { label: 'Graded', color: 'bg-accent-success/20 text-accent-success', icon: Award, dot: 'bg-accent-success' },
  pending_review: { label: 'Pending Review', color: 'bg-accent-warning/20 text-accent-warning', icon: Clock, dot: 'bg-accent-warning' },
  unsubmitted: { label: 'Not Submitted', color: 'bg-surface-2 text-text-secondary', icon: MinusCircle, dot: 'bg-gray-400' },
};

const getStatusConfig = (student) => {
  if (!student?.submission) return STATUS_CONFIG.unsubmitted;
  const state = student.submission.workflow_state || 'unsubmitted';
  return STATUS_CONFIG[state] || STATUS_CONFIG.unsubmitted;
};

const formatDate = (dateStr) => (dateStr ? new Date(dateStr).toLocaleString() : '');

const initialsOf = (name, fallbackId) => {
  if (!name) return `U${fallbackId ?? ''}`.slice(0, 2).toUpperCase();
  return name
    .split(/\s+/)
    .map((n) => n[0])
    .filter(Boolean)
    .slice(0, 2)
    .join('')
    .toUpperCase();
};

const displayName = (student) => student?.user_name || (student?.user_id ? `User ${student.user_id}` : 'Student');

const StudentListItem = React.memo(function StudentListItem({ student, statuses, isSelected, pointsPossible, onSelect, onKeyDown, itemRef }) {
  const config = getStatusConfig(student);
  const score = student.submission?.score;
  const hasScore = score !== null && score !== undefined;
  const key = studentKey(student);
  const status = studentStatus(student.submission);
  const custom = statuses.find((s) => s.id === student.submission?.custom_grade_status_id);
  return (
    <button
      ref={itemRef}
      onClick={() => onSelect(key)}
      onKeyDown={onKeyDown}
      role="option"
      aria-selected={isSelected}
      className={`w-full text-left px-3 py-2.5 border-b border-border-subtle flex items-center space-x-2 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 ${
        isSelected ? 'bg-brand-50 border-l-4 border-l-blue-500' : 'hover:bg-surface-1 border-l-4 border-l-transparent'
      }`}
    >
      <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${config.dot}`} aria-hidden="true" />
      <Avatar className="h-7 w-7 flex-shrink-0">
        <AvatarFallback className="text-xs">{initialsOf(student.user_name, key)}</AvatarFallback>
      </Avatar>
      <div className="flex-1 min-w-0">
        <p className={`text-sm truncate ${isSelected ? 'font-semibold text-blue-900' : 'text-text-secondary'}`}>{displayName(student)}</p>
        <p className="text-xs text-text-secondary flex items-center gap-1">
          <span>{student.submission?.excused ? 'Excused' : hasScore ? `${score}/${pointsPossible ?? 0}` : config.label}</span>
          {status === 'late' && <span className="text-accent-danger">· Late</span>}
          {status === 'missing' && <span className="text-accent-danger">· Missing</span>}
          {status === 'extended' && <span className="text-text-tertiary">· Extended</span>}
          {custom && (
            <span className="inline-flex items-center gap-1">
              · <span className="inline-block h-2 w-2 rounded-full" style={{ backgroundColor: custom.color }} aria-hidden="true" />
              {custom.name}
            </span>
          )}
        </p>
        {student.group_name && <p className="text-[11px] text-text-tertiary truncate">{student.group_name}</p>}
      </div>
    </button>
  );
});

const RubricCriterionRow = React.memo(function RubricCriterionRow({ criterion, score, comment, disabled, onScoreChange, onCommentChange }) {
  const cId = criterion.id || criterion.description;
  const ratings = criterion.ratings || [];
  return (
    <div className="border-b border-border-subtle pb-3 last:border-0 last:pb-0">
      <div className="flex items-center justify-between mb-1">
        <p className="text-xs font-semibold text-text-secondary">{criterion.description}</p>
        <Badge variant="outline" className="text-xs">{criterion.points} pts</Badge>
      </div>
      {criterion.long_description && <p className="text-xs text-text-secondary mb-2">{criterion.long_description}</p>}
      {ratings.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {ratings.map((rating) => {
            const isSelected = parseFloat(score) === rating.points;
            return (
              <button
                key={rating.id || rating.description}
                type="button"
                onClick={() => onScoreChange(cId, rating.points)}
                className={`text-xs px-2 py-1 rounded border transition-colors ${
                  isSelected
                    ? 'bg-brand-100 border-blue-400 text-brand-800 font-medium'
                    : 'bg-surface-0 border-border-default text-text-secondary hover:border-blue-300 hover:bg-brand-50'
                }`}
                title={rating.description}
                disabled={disabled}
              >
                {rating.points} - {rating.description}
              </button>
            );
          })}
        </div>
      )}
      <div className="flex items-center gap-2">
        <input
          type="number"
          step="any"
          min="0"
          max={criterion.points}
          value={score ?? ''}
          onChange={(e) => onScoreChange(cId, e.target.value)}
          placeholder="Pts"
          aria-label={`Points for ${criterion.description}`}
          className="w-16 border border-border-strong rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-brand-500"
          disabled={disabled}
        />
        <input
          type="text"
          value={comment || ''}
          onChange={(e) => onCommentChange(cId, e.target.value)}
          placeholder="Comment..."
          aria-label={`Comment for ${criterion.description}`}
          className="flex-1 border border-border-strong rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-brand-500"
          disabled={disabled}
        />
      </div>
    </div>
  );
});

// SubmissionContent renders one attempt (the current submission or an
// entry of submission_history).
const SubmissionContent = ({ sub, assignment, student, courseId }) => {
  if (!sub) return null;
  const isAnnotation = sub.submission_type === 'student_annotation';
  return (
    <div className="space-y-4">
      {sub.body && <SubmissionBody className="prose max-w-none text-text-secondary" body={sub.body} />}
      {sub.url && (
        <a href={sub.url} target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline break-all block">
          {sub.url}
        </a>
      )}
      {sub.attachments?.length > 0 && (
        <div className="space-y-4">
          {sub.attachments.map((file, idx) => (
            <AttachmentPreview key={file.id || idx} file={file} height={640} />
          ))}
        </div>
      )}
      {isAnnotation && assignment.annotatable_attachment_id && (
        student.user_id ? (
          <div className="h-[720px]">
            <DocumentViewer
              courseId={courseId}
              assignmentId={assignment.id}
              userId={student.user_id}
              submissionId={sub.id}
              documentNode={<DocumentImage doc={{ url: `/api/v1/courses/${courseId}/assignments/${assignment.id}/annotatable_document`, content_type: 'application/pdf' }} />}
            />
          </div>
        ) : (
          <div className="space-y-2">
            <DocumentImage doc={{ url: `/api/v1/courses/${courseId}/assignments/${assignment.id}/annotatable_document`, content_type: 'application/pdf' }} height={560} />
            <p className="text-xs text-text-tertiary">
              The student&apos;s annotations open in DocViewer once anonymity is lifted (after grades are posted).
            </p>
          </div>
        )
      )}
      {sub.submission_type === 'discussion_topic' && student.user_id && (
        <DiscussionSubmissionPosts courseId={courseId} assignmentId={assignment.id} userId={student.user_id} />
      )}
      {sub.submission_type === 'basic_lti_launch' && !sub.url && (
        <p className="text-sm text-text-secondary">Submitted through an external tool.</p>
      )}
      {!sub.body && !sub.url && !sub.attachments?.length && !isAnnotation && sub.submission_type !== 'basic_lti_launch' && sub.submission_type !== 'discussion_topic' && (
        <p className="text-text-secondary italic">No content available for this submission type.</p>
      )}
      {sub.submission_type === 'online_text_entry' && student.user_id && (
        <Link
          to={`/courses/${courseId}/assignments/${assignment.id}/submissions/${student.user_id}/docviewer`}
          className="inline-block text-xs text-brand-600 hover:underline"
        >
          Annotate in DocViewer
        </Link>
      )}
    </div>
  );
};

const SubmissionPreview = ({ student, assignment, courseId, viewAttempt, onViewAttempt }) => {
  if (!student) {
    return (
      <Card className="flex-1 flex items-center justify-center text-text-secondary p-6">
        <p>Select a student to view their submission</p>
      </Card>
    );
  }
  const config = getStatusConfig(student);
  const sub = student.submission;
  const history = Array.isArray(student.submission_history) ? student.submission_history : [];
  const isUnsubmitted = !sub || sub.workflow_state === 'unsubmitted' || (!sub.submitted_at && !sub.submission_type);
  const shown = viewAttempt != null ? history.find((h) => h.attempt === viewAttempt) : null;
  const current = shown ? { ...shown } : sub;

  return (
    <Card className="flex-1 overflow-hidden flex flex-col">
      <div className="p-4 border-b bg-surface-1 flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center space-x-3">
          <Avatar className="h-9 w-9">
            <AvatarFallback>{initialsOf(student.user_name, studentKey(student))}</AvatarFallback>
          </Avatar>
          <div>
            <p className="font-semibold text-text-primary">{displayName(student)}</p>
            {current?.submitted_at && (
              <p className="text-xs text-text-tertiary">
                Submitted {formatDate(current.submitted_at)}
                {current.late && <span className="ml-2 text-accent-danger font-medium">LATE</span>}
              </p>
            )}
            {student.group_name && (
              <p className="text-xs text-text-tertiary flex items-center gap-1"><Users className="w-3 h-3" /> {student.group_name}</p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {history.length > 1 && (
            <label className="flex items-center gap-1 text-xs text-text-secondary">
              <History className="w-3.5 h-3.5" />
              <span className="sr-only">Attempt</span>
              <select
                aria-label="Submission attempt"
                value={viewAttempt ?? ''}
                onChange={(e) => onViewAttempt(e.target.value === '' ? null : Number(e.target.value))}
                className="border border-border-strong rounded px-1.5 py-1 text-xs"
              >
                <option value="">Current attempt{sub?.attempt ? ` (${sub.attempt})` : ''}</option>
                {history
                  .filter((h) => h.attempt !== sub?.attempt)
                  .map((h) => (
                    <option key={h.attempt} value={h.attempt}>
                      Attempt {h.attempt} — {formatDate(h.submitted_at)}
                    </option>
                  ))}
              </select>
            </label>
          )}
          {sub?.excused ? (
            <Badge variant="outline" className="bg-surface-2">Excused</Badge>
          ) : (
            <Badge className={config.color} variant="outline">{config.label}</Badge>
          )}
        </div>
      </div>

      <div className="p-6 flex-1 overflow-y-auto">
        {isUnsubmitted && !shown ? (
          <div className="flex flex-col items-center justify-center h-full text-text-secondary">
            <AlertCircle className="w-12 h-12 mb-3" />
            <p className="text-lg font-medium">No Submission</p>
            <p className="text-sm">This student has not submitted this assignment.</p>
          </div>
        ) : (
          <div>
            {current?.submission_type && (
              <div className="flex items-center space-x-2 mb-4 text-sm text-text-tertiary">
                <FileText className="w-4 h-4" />
                <span>
                  Type: {current.submission_type}
                  {current.attempt > 0 && ` (Attempt ${current.attempt})`}
                </span>
              </div>
            )}
            <SubmissionContent sub={{ ...current, id: sub?.id }} assignment={assignment} student={student} courseId={courseId} />
          </div>
        )}
      </div>
    </Card>
  );
};

const SpeedGraderSkeleton = () => (
  <Layout>
    <div className="mb-4">
      <Skeleton className="h-4 w-32 mb-2" />
      <Skeleton className="h-8 w-48" />
    </div>
    <div className="flex gap-4">
      <div className="w-64 flex-shrink-0 space-y-2">
        <Skeleton className="h-10 w-full" />
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
      <div className="flex-1 space-y-4">
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-[60vh] w-full" />
      </div>
      <div className="w-80 flex-shrink-0 space-y-4">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    </div>
  </Layout>
);

const SpeedGraderPage = () => {
  const { courseId, assignmentId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const { announce } = useLiveRegion();
  const [assignment, setAssignment] = useState(null);
  const [students, setStudents] = useState([]);
  const [statuses, setStatuses] = useState([]);
  const [selectedKey, setSelectedKey] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionError, setActionError] = useState(null);

  const [statusFilter, setStatusFilter] = useState('all');
  const [sortBy, setSortBy] = useState('name');
  const [groupBy, setGroupBy] = useState(false);
  const [viewAttempt, setViewAttempt] = useState(null);

  const [gradeInput, setGradeInput] = useState('');
  const [grading, setGrading] = useState(false);
  const [gradeSuccess, setGradeSuccess] = useState(false);

  const [rubric, setRubric] = useState(null);
  const [rubricCriteria, setRubricCriteria] = useState([]);
  const [rubricScores, setRubricScores] = useState({});
  const [rubricExpanded, setRubricExpanded] = useState(true);
  const [rubricSaving, setRubricSaving] = useState(false);
  const [existingAssessmentId, setExistingAssessmentId] = useState(null);

  const studentRefs = useRef({});

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const result = await api.getSpeedGraderData(courseId, assignmentId);
      setAssignment(result.assignment);
      setStudents(result.students || []);
      setStatuses(Array.isArray(result.custom_grade_statuses) ? result.custom_grade_statuses : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId, assignmentId]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const anonymous = !!assignment?.anonymize_students;
  const moderatedPending = !!assignment?.moderated_grading && !assignment?.grades_published;

  const visibleStudents = useMemo(() => {
    const filtered = filterStudents(students, statusFilter);
    return sortStudents(filtered, sortBy, { groupBy: groupBy && !!assignment?.is_group_assignment, anonymous });
  }, [students, statusFilter, sortBy, groupBy, assignment, anonymous]);

  useEffect(() => {
    if (visibleStudents.length === 0) return;
    if (!selectedKey || !visibleStudents.some((s) => studentKey(s) === selectedKey)) {
      setSelectedKey(studentKey(visibleStudents[0]));
    }
  }, [visibleStudents, selectedKey]);

  const selectedIndex = visibleStudents.findIndex((s) => studentKey(s) === selectedKey);
  const selectedStudent = selectedIndex >= 0 ? visibleStudents[selectedIndex] : null;

  const updateStudent = useCallback((key, fn) => {
    setStudents((prev) => prev.map((s) => (studentKey(s) === key ? fn(s) : s)));
  }, []);

  // Rubric
  useEffect(() => {
    if (!courseId || !assignmentId) return;
    api.getAssignmentRubric(courseId, assignmentId)
      .then((data) => {
        setRubric(data);
        try {
          const rubricData = data?.rubric?.data;
          const criteria = typeof rubricData === 'string' ? JSON.parse(rubricData) : rubricData;
          setRubricCriteria(Array.isArray(criteria) ? criteria : []);
        } catch {
          setRubricCriteria([]);
        }
      })
      .catch(() => { setRubric(null); setRubricCriteria([]); });
  }, [courseId, assignmentId]);

  useEffect(() => {
    setRubricScores({});
    setExistingAssessmentId(null);
    if (!rubric || !selectedStudent) return;
    const assocId = rubric.rubric_association?.id;
    if (!assocId) return;
    const parse = (raw) => {
      try { return (typeof raw === 'string' ? JSON.parse(raw) : raw) || {}; } catch { return {}; }
    };
    if (selectedStudent.anonymous_id) {
      gradingApi.getAnonymousRubricAssessment(courseId, assignmentId, selectedStudent.anonymous_id)
        .then((a) => {
          if (a) { setExistingAssessmentId(a.id); setRubricScores(parse(a.data)); }
        })
        .catch(() => {});
      return;
    }
    api.getRubricAssessments(courseId, assocId, 1, 200)
      .then((result) => {
        const existing = (result.data || []).find((a) => a.user_id === selectedStudent.user_id);
        if (existing) {
          setExistingAssessmentId(existing.id);
          setRubricScores(parse(existing.data));
        }
      })
      .catch(() => {});
  }, [courseId, assignmentId, rubric, selectedStudent?.user_id, selectedStudent?.anonymous_id]); // eslint-disable-line react-hooks/exhaustive-deps

  // The grade box shows the ENTERED score (B021) — or, while a moderated
  // assignment is unpublished, the caller's own provisional grade.
  useEffect(() => {
    if (!selectedStudent) return;
    const sub = selectedStudent.submission;
    let score = sub?.entered_score ?? sub?.score;
    if (moderatedPending) {
      const mine = (selectedStudent.provisional_grades || []).find((p) => p.mine && !p.final);
      score = mine ? mine.score : null;
    }
    setGradeInput(score !== null && score !== undefined ? String(score) : '');
    setGradeSuccess(false);
    setViewAttempt(null);
    setActionError(null);
  }, [selectedKey, selectedStudent?.submission?.score, moderatedPending]); // eslint-disable-line react-hooks/exhaustive-deps

  const submittedCount = useMemo(
    () => students.filter((s) => s.submission && s.submission.submitted_at).length,
    [students]
  );
  const gradedCount = useMemo(
    () => students.filter((s) => s.submission && (s.submission.workflow_state === 'graded' || s.submission.excused)).length,
    [students]
  );

  const rubricTotal = useMemo(
    () => Object.values(rubricScores).reduce((sum, c) => sum + (parseFloat(c?.points) || 0), 0),
    [rubricScores]
  );
  const rubricMax = useMemo(
    () => rubric?.rubric?.points_possible ?? rubricCriteria.reduce((s, c) => s + (c.points || 0), 0),
    [rubric, rubricCriteria]
  );

  const hasExistingGrade = selectedStudent?.submission?.score !== null && selectedStudent?.submission?.score !== undefined;
  const canPost = !!selectedStudent && (gradeInput !== '' || (hasExistingGrade && !moderatedPending)) && !grading;

  const handleSelectStudent = useCallback((key) => setSelectedKey(key), []);

  const handleStudentKeyDown = useCallback((e) => {
    const focusByIndex = (i) => {
      const target = visibleStudents[i];
      if (!target) return;
      const key = studentKey(target);
      setSelectedKey(key);
      studentRefs.current[key]?.focus();
      announce(`${displayName(target)} selected.`);
    };
    if (e.key === 'ArrowDown') { e.preventDefault(); focusByIndex(Math.min(selectedIndex + 1, visibleStudents.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); focusByIndex(Math.max(selectedIndex - 1, 0)); }
    else if (e.key === 'Home') { e.preventDefault(); focusByIndex(0); }
    else if (e.key === 'End') { e.preventDefault(); focusByIndex(visibleStudents.length - 1); }
  }, [visibleStudents, selectedIndex, announce]);

  // applyUpdate sends a submission update (grade, excuse, status, extra
  // attempts) for the selected student and merges the server's answer.
  const applyUpdate = useCallback(async (student, update, message) => {
    const key = studentKey(student);
    const saved = await gradingApi.updateSubmission(courseId, assignmentId, submissionKeyOf(student), update);
    updateStudent(key, (s) => ({ ...s, submission: mergeGradedSubmission(s.submission, saved) }));
    // A group graded together: every member got the same grade.
    if (assignment?.is_group_assignment && !assignment?.grade_group_students_individually && student.group_id && ('posted_grade' in update || 'excused' in update)) {
      setStudents((prev) => prev.map((s) => (
        s.group_id === student.group_id && studentKey(s) !== key
          ? { ...s, submission: mergeGradedSubmission(s.submission, { ...saved, attachments: undefined, body: undefined, url: undefined }) }
          : s
      )));
    }
    if (message) announce(message);
    return saved;
  }, [courseId, assignmentId, assignment, updateStudent, announce]);

  const postScore = useCallback(async (value) => {
    if (!selectedStudent) return;
    const clearing = String(value).trim() === '';
    setGrading(true);
    setGradeSuccess(false);
    setActionError(null);
    try {
      if (moderatedPending) {
        const key = studentKey(selectedStudent);
        const pg = await gradingApi.saveProvisionalGrade(courseId, assignmentId, {
          userId: selectedStudent.user_id,
          anonymousId: selectedStudent.anonymous_id,
          postedGrade: String(value),
        });
        updateStudent(key, (s) => {
          const others = (s.provisional_grades || []).filter((p) => !(p.mine && !p.final));
          return { ...s, provisional_grades: [...others, { ...pg, mine: true, scorer_name: 'You' }] };
        });
        announce(`Provisional grade ${value} saved for ${displayName(selectedStudent)}.`);
      } else {
        await applyUpdate(selectedStudent, { posted_grade: clearing ? '' : String(value) },
          clearing ? `Grade cleared for ${displayName(selectedStudent)}.` : `Grade ${value} saved for ${displayName(selectedStudent)}.`);
      }
      setGradeSuccess(true);
      setTimeout(() => setGradeSuccess(false), 2000);
    } catch (err) {
      setActionError(err.message);
      announce(`Error saving grade: ${err.message}`, 'assertive');
    } finally {
      setGrading(false);
    }
  }, [selectedStudent, moderatedPending, courseId, assignmentId, updateStudent, applyUpdate, announce]);

  const handlePost = useCallback((e) => {
    e.preventDefault();
    if (gradeInput.trim() === '' && (!hasExistingGrade || moderatedPending)) return;
    postScore(gradeInput);
  }, [gradeInput, hasExistingGrade, moderatedPending, postScore]);

  const toggleExcused = useCallback(async () => {
    if (!selectedStudent) return;
    const next = !selectedStudent.submission?.excused;
    setActionError(null);
    try {
      await applyUpdate(selectedStudent, { excused: next }, next ? 'Student excused.' : 'Excuse removed.');
    } catch (err) {
      setActionError(err.message);
    }
  }, [selectedStudent, applyUpdate]);

  const setStatus = useCallback(async (value) => {
    if (!selectedStudent) return;
    setActionError(null);
    try {
      if (value.startsWith('custom:')) {
        await applyUpdate(selectedStudent, { custom_grade_status_id: Number(value.slice(7)) }, 'Status updated.');
      } else {
        const update = { late_policy_status: value };
        if (selectedStudent.submission?.custom_grade_status_id) update.custom_grade_status_id = 0;
        await applyUpdate(selectedStudent, update, 'Status updated.');
      }
    } catch (err) {
      setActionError(err.message);
    }
  }, [selectedStudent, applyUpdate]);

  const setExtraAttempts = useCallback(async (n) => {
    if (!selectedStudent) return;
    setActionError(null);
    try {
      await applyUpdate(selectedStudent, { extra_attempts: Math.max(0, parseInt(n, 10) || 0) }, 'Extra attempts updated.');
    } catch (err) {
      setActionError(err.message);
    }
  }, [selectedStudent, applyUpdate]);

  const selectProvisional = useCallback(async (pg) => {
    if (!selectedStudent) return;
    setActionError(null);
    try {
      await gradingApi.selectProvisionalGrade(courseId, assignmentId, {
        userId: selectedStudent.user_id,
        anonymousId: selectedStudent.anonymous_id,
        provisionalGradeId: pg.id,
      });
      updateStudent(studentKey(selectedStudent), (s) => ({
        ...s,
        provisional_grades: (s.provisional_grades || []).map((p) => ({ ...p, selected: p.id === pg.id })),
      }));
      announce('Provisional grade selected.');
    } catch (err) {
      setActionError(err.message);
    }
  }, [selectedStudent, courseId, assignmentId, updateStudent, announce]);

  const handleCriterionScoreChange = useCallback((criterionId, value) => {
    setRubricScores((prev) => ({ ...prev, [criterionId]: { ...prev[criterionId], points: value } }));
  }, []);
  const handleCriterionCommentChange = useCallback((criterionId, value) => {
    setRubricScores((prev) => ({ ...prev, [criterionId]: { ...prev[criterionId], comments: value } }));
  }, []);

  // Saving the rubric also posts its total as the grade (Canvas "use
  // this rubric for grading").
  const handleSaveRubric = useCallback(async () => {
    if (!rubric || !selectedStudent || !rubric.rubric_association?.id) return;
    const assocId = rubric.rubric_association.id;
    setRubricSaving(true);
    setActionError(null);
    try {
      // B120: a typed criterion box holds a string ("4"); save numbers.
      const dataStr = JSON.stringify(normalizeRubricScores(rubricScores));
      if (selectedStudent.anonymous_id) {
        const saved = await gradingApi.saveAnonymousRubricAssessment(courseId, assignmentId, selectedStudent.anonymous_id, dataStr);
        if (saved?.id) setExistingAssessmentId(saved.id);
      } else if (existingAssessmentId) {
        await api.updateRubricAssessment(courseId, assocId, existingAssessmentId, { data: dataStr, assessment_type: 'grading' });
      } else {
        const created = await api.createRubricAssessment(courseId, assocId, {
          user_id: selectedStudent.user_id,
          data: dataStr,
          assessment_type: 'grading',
        });
        setExistingAssessmentId(created.id);
      }
      setGradeInput(String(rubricTotal));
      await postScore(String(rubricTotal));
      announce(`Rubric saved. Total ${rubricTotal} of ${rubricMax}.`);
    } catch (err) {
      setActionError(err.message);
      announce(`Error saving rubric: ${err.message}`, 'assertive');
    } finally {
      setRubricSaving(false);
    }
  }, [rubric, selectedStudent, rubricScores, existingAssessmentId, courseId, assignmentId, rubricTotal, rubricMax, postScore, announce]);

  const navigateStudent = useCallback((direction) => {
    const next = selectedIndex + direction;
    if (next < 0 || next >= visibleStudents.length) return;
    const target = visibleStudents[next];
    setSelectedKey(studentKey(target));
    announce(`${displayName(target)} selected. ${next + 1} of ${visibleStudents.length}.`);
  }, [selectedIndex, visibleStudents, announce]);

  const setStudentRef = useCallback((key) => (node) => {
    if (node) studentRefs.current[key] = node;
    else delete studentRefs.current[key];
  }, []);

  const onCommentsChange = useCallback((next) => {
    if (!selectedStudent) return;
    updateStudent(studentKey(selectedStudent), (s) => ({ ...s, comments: next }));
  }, [selectedStudent, updateStudent]);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null || loading) return <SpeedGraderSkeleton />;

  if (error) {
    return (
      <Layout>
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <Button variant="link" onClick={() => { setError(null); window.location.reload(); }} className="text-brand-600">
            Try Again
          </Button>
        </div>
      </Layout>
    );
  }

  if (!assignment) {
    return <Layout><div className="text-center py-12">Assignment not found</div></Layout>;
  }

  const sub = selectedStudent?.submission;
  const currentStatus = sub?.custom_grade_status_id
    ? `custom:${sub.custom_grade_status_id}`
    : studentStatus(sub) === 'none' ? 'none' : studentStatus(sub);
  const groupTogether = assignment.is_group_assignment && !assignment.grade_group_students_individually;

  return (
    <TooltipProvider delayDuration={200}>
      <Layout>
        <div className="mb-4">
          <Link to={`/courses/${courseId}/assignments/${assignmentId}`} className="text-brand-600 hover:underline text-sm">
            &larr; Back to Assignment
          </Link>
          <div className="flex items-center justify-between mt-2 flex-wrap gap-2">
            <h2 className="text-2xl font-bold text-text-primary">SpeedGrader</h2>
            <div className="flex items-center gap-3 text-sm text-text-tertiary flex-wrap">
              {assignment.anonymous_grading && (
                <Badge variant="secondary" className="bg-purple-100 text-purple-700 hover:bg-purple-100">
                  {anonymous ? 'Students anonymized' : 'Anonymous grading (lifted)'}
                </Badge>
              )}
              {assignment.moderated_grading && (
                <Badge variant="secondary" className="bg-sky-100 text-sky-700 hover:bg-sky-100">
                  {moderatedPending ? 'Moderated: provisional grades' : 'Moderated: published'}
                </Badge>
              )}
              {assignment.moderated_grading && assignment.is_moderator && (
                <Link to={`/courses/${courseId}/assignments/${assignmentId}/moderate`} className="inline-flex items-center gap-1 text-brand-600 hover:underline">
                  <Scale className="w-4 h-4" /> Moderate
                </Link>
              )}
              <span>{submittedCount}/{students.length} submitted</span>
              <Separator orientation="vertical" className="h-4" />
              <span>{gradedCount}/{students.length} graded</span>
            </div>
          </div>
          <p className="text-text-tertiary text-sm mt-1">
            {assignment.name} &middot; {assignment.points_possible ?? 0} points
          </p>
        </div>

        {actionError && (
          <div className="mb-3 p-2 rounded border border-accent-danger/30 bg-accent-danger/10 text-sm text-accent-danger" role="alert">
            {actionError}
          </div>
        )}

        <div className="flex gap-4 items-start">
          {/* Left rail: student list with sort / filter */}
          <Card className="w-64 flex-shrink-0 sticky top-0 self-start max-h-screen overflow-y-auto flex flex-col">
            <div className="p-3 border-b bg-surface-1 space-y-2">
              <h3 className="font-semibold text-sm text-text-secondary">
                Students ({visibleStudents.length}{visibleStudents.length !== students.length ? ` of ${students.length}` : ''})
              </h3>
              <label className="block text-xs text-text-tertiary">
                Show
                <select
                  aria-label="Filter students"
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                  className="mt-0.5 w-full border border-border-strong rounded px-1.5 py-1 text-xs"
                >
                  {STATUS_FILTERS.map((f) => <option key={f.value} value={f.value}>{f.label}</option>)}
                </select>
              </label>
              <label className="block text-xs text-text-tertiary">
                Sort by
                <select
                  aria-label="Sort students"
                  value={sortBy}
                  onChange={(e) => setSortBy(e.target.value)}
                  className="mt-0.5 w-full border border-border-strong rounded px-1.5 py-1 text-xs"
                >
                  {SORT_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </label>
              {assignment.is_group_assignment && !anonymous && (
                <label className="flex items-center gap-1 text-xs text-text-tertiary">
                  <input type="checkbox" checked={groupBy} onChange={(e) => setGroupBy(e.target.checked)} />
                  Group students by group
                </label>
              )}
            </div>
            <div className="overflow-y-auto flex-1" role="listbox" aria-label="Students">
              {visibleStudents.length === 0 ? (
                <div className="p-4 text-center text-text-tertiary text-sm">No students match</div>
              ) : (
                visibleStudents.map((student) => (
                  <StudentListItem
                    key={studentKey(student)}
                    student={student}
                    statuses={statuses}
                    isSelected={studentKey(student) === selectedKey}
                    pointsPossible={assignment.points_possible}
                    onSelect={handleSelectStudent}
                    onKeyDown={handleStudentKeyDown}
                    itemRef={setStudentRef(studentKey(student))}
                  />
                ))
              )}
            </div>
          </Card>

          {/* Center: submission preview */}
          <div className="flex-1 flex flex-col gap-4 min-w-0">
            <Card className="px-4 py-3 flex items-center justify-between">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="sm" onClick={() => navigateStudent(-1)} disabled={selectedIndex <= 0}>
                    <ChevronLeft className="w-4 h-4" />
                    Previous
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Previous student</TooltipContent>
              </Tooltip>
              <span className="text-sm font-medium text-text-secondary">
                {visibleStudents.length > 0 ? `${selectedIndex + 1} of ${visibleStudents.length}` : 'No students'}
                {selectedStudent && <span className="text-text-tertiary"> &mdash; {displayName(selectedStudent)}</span>}
              </span>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="ghost" size="sm" onClick={() => navigateStudent(1)} disabled={selectedIndex >= visibleStudents.length - 1}>
                    Next
                    <ChevronRight className="w-4 h-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Next student</TooltipContent>
              </Tooltip>
            </Card>

            <SubmissionPreview
              student={selectedStudent}
              assignment={assignment}
              courseId={courseId}
              viewAttempt={viewAttempt}
              onViewAttempt={setViewAttempt}
            />
          </div>

          {/* Right rail: grade, status, rubric, comments */}
          <div className="w-80 flex-shrink-0 sticky top-0 self-start max-h-screen overflow-y-auto flex flex-col gap-4">
            <Card className="p-4 space-y-3">
              <h3 className="font-semibold text-sm text-text-secondary flex items-center space-x-2">
                <Award className="w-4 h-4" />
                <span>{moderatedPending ? 'Provisional grade' : 'Grade'}</span>
              </h3>
              {groupTogether && selectedStudent?.group_name && (
                <p className="text-xs text-text-tertiary">This grade applies to everyone in {selectedStudent.group_name}.</p>
              )}
              <form onSubmit={handlePost} className="space-y-3">
                <div className="flex items-center space-x-2">
                  <input
                    type="text"
                    inputMode={['letter_grade', 'gpa_scale', 'pass_fail', 'percent'].includes(assignment.grading_type) ? 'text' : 'decimal'}
                    value={gradeInput}
                    onChange={(e) => setGradeInput(e.target.value)}
                    placeholder="Score"
                    className="flex-1 border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                    disabled={!selectedStudent || !!sub?.excused}
                    aria-label="Grade score"
                  />
                  <span className="text-sm text-text-tertiary whitespace-nowrap">/ {assignment.points_possible ?? 0}</span>
                </div>

                {assignment.points_possible > 0 && gradeInput !== '' && !Number.isNaN(Number(gradeInput)) && (!assignment.grading_type || assignment.grading_type === 'points') && (
                  <div className="text-xs text-text-tertiary">
                    {((parseFloat(gradeInput) / assignment.points_possible) * 100).toFixed(1)}%
                  </div>
                )}
                {sub?.points_deducted > 0 && (
                  <div className="text-xs text-accent-danger">
                    Late penalty -{sub.points_deducted} pts (final score {sub.score})
                  </div>
                )}
                <Button type="submit" disabled={!canPost || !!sub?.excused} className="w-full">
                  {grading
                    ? 'Saving...'
                    : moderatedPending
                    ? 'Save provisional grade'
                    : gradeInput.trim() === '' && hasExistingGrade ? 'Clear Grade' : 'Update Grade'}
                </Button>
                {gradeSuccess && (
                  <div className="flex items-center space-x-1 text-accent-success text-xs">
                    <CheckCircle className="w-3 h-3" />
                    <span>Grade saved</span>
                  </div>
                )}
              </form>

              {!moderatedPending && (
                <label className="flex items-center gap-2 text-sm text-text-secondary">
                  <input type="checkbox" checked={!!sub?.excused} onChange={toggleExcused} disabled={!selectedStudent} />
                  Excuse this student
                </label>
              )}

              <label className="block text-xs text-text-tertiary">
                Status
                <select
                  aria-label="Submission status"
                  value={currentStatus}
                  onChange={(e) => setStatus(e.target.value)}
                  disabled={!selectedStudent}
                  className="mt-0.5 w-full border border-border-strong rounded px-2 py-1 text-sm"
                >
                  <option value="none">None</option>
                  <option value="late">Late</option>
                  <option value="missing">Missing</option>
                  <option value="extended">Extended</option>
                  {statuses.map((st) => <option key={st.id} value={`custom:${st.id}`}>{st.name}</option>)}
                </select>
              </label>

              {assignment.allowed_attempts > 0 && selectedStudent && (
                <div className="text-xs text-text-tertiary space-y-1">
                  <p>
                    {selectedStudent.attempts_remaining != null
                      ? `${selectedStudent.attempts_remaining} attempt${selectedStudent.attempts_remaining === 1 ? '' : 's'} remaining`
                      : ''}
                    {' '}of {assignment.allowed_attempts} allowed
                  </p>
                  <label className="flex items-center gap-2">
                    Extra attempts
                    <input
                      key={studentKey(selectedStudent)}
                      type="number"
                      min="0"
                      defaultValue={sub?.extra_attempts ?? 0}
                      onBlur={(e) => { if (String(e.target.value) !== String(sub?.extra_attempts ?? 0)) setExtraAttempts(e.target.value); }}
                      className="w-16 border border-border-strong rounded px-2 py-0.5 text-xs"
                      aria-label="Extra attempts"
                    />
                  </label>
                </div>
              )}

              {assignment.moderated_grading && Array.isArray(selectedStudent?.provisional_grades) && selectedStudent.provisional_grades.length > 0 && (
                <div className="border-t pt-2">
                  <p className="text-xs font-medium text-text-secondary mb-1">Provisional grades</p>
                  <ul className="space-y-1">
                    {selectedStudent.provisional_grades.map((pg) => (
                      <li key={pg.id} className="flex items-center justify-between text-xs">
                        <span>
                          {pg.mine ? 'You' : pg.scorer_name}{pg.final ? ' (final)' : ''}: <strong>{pg.score ?? pg.grade ?? '—'}</strong>
                        </span>
                        {assignment.is_moderator && moderatedPending && (
                          pg.selected ? (
                            <span className="text-accent-success">Selected</span>
                          ) : (
                            <button type="button" onClick={() => selectProvisional(pg)} className="text-brand-600 hover:underline">Select</button>
                          )
                        )}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </Card>

            {rubricCriteria.length > 0 && (
              <Card className="overflow-hidden">
                <button
                  onClick={() => setRubricExpanded((v) => !v)}
                  className="w-full p-3 border-b bg-surface-1 flex items-center justify-between hover:bg-surface-2 transition-colors"
                  aria-expanded={rubricExpanded}
                >
                  <h3 className="font-semibold text-sm text-text-secondary flex items-center space-x-2">
                    <Grid className="w-4 h-4" />
                    <span>Rubric ({rubricCriteria.length} criteria)</span>
                  </h3>
                  {rubricExpanded ? <ChevronUp className="w-4 h-4 text-text-disabled" /> : <ChevronDown className="w-4 h-4 text-text-disabled" />}
                </button>
                {rubricExpanded && (
                  <div className="p-3 space-y-4">
                    {rubricCriteria.map((criterion) => {
                      const cId = criterion.id || criterion.description;
                      const cs = rubricScores[cId] || {};
                      return (
                        <RubricCriterionRow
                          key={cId}
                          criterion={criterion}
                          score={cs.points}
                          comment={cs.comments}
                          disabled={!selectedStudent}
                          onScoreChange={handleCriterionScoreChange}
                          onCommentChange={handleCriterionCommentChange}
                        />
                      );
                    })}
                    <Separator />
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-semibold text-text-secondary">Total: {rubricTotal} / {rubricMax}</span>
                      <Button size="sm" onClick={handleSaveRubric} disabled={!selectedStudent || rubricSaving || !!sub?.excused}>
                        {rubricSaving ? 'Saving...' : 'Save rubric & grade'}
                      </Button>
                    </div>
                  </div>
                )}
              </Card>
            )}

            <Card className="p-4">
              {selectedStudent ? (
                <CommentThread
                  key={studentKey(selectedStudent)}
                  courseId={courseId}
                  assignmentId={assignmentId}
                  submissionKey={submissionKeyOf(selectedStudent)}
                  isTeacher
                  currentUserId={user?.id ?? null}
                  isGroupAssignment={!!assignment.is_group_assignment && !anonymous}
                  initialComments={selectedStudent.comments || []}
                  onCommentsChange={onCommentsChange}
                  compact
                />
              ) : (
                <p className="text-sm text-text-secondary text-center py-4">No student selected</p>
              )}
            </Card>
          </div>
        </div>
      </Layout>
    </TooltipProvider>
  );
};

export default SpeedGraderPage;
