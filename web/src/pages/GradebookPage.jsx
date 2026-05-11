import React, { useState, useEffect, useMemo, useCallback, useRef, memo } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import {
  Download,
  Upload,
  MoreVertical,
  HelpCircle,
  ArrowDownAZ,
  ArrowUpAZ,
  Eye,
  EyeOff,
  Send,
  Calculator,
  Sparkles,
} from 'lucide-react';
import { Grid, useGridRef } from 'react-window';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import GradeInput from '../components/GradeInput';
import { getLetterGrade, gradeColor } from '../utils/grading';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '../components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '../components/ui/dialog';

const Skeleton = ({ className = '' }) => (
  <div className={`animate-pulse rounded-md bg-border-default ${className}`} />
);

const Badge = ({ className = '', children }) => (
  <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold ${className}`}>
    {children}
  </span>
);

const ROW_HEIGHT = 44;
const COL_WIDTH = 120;
const STUDENT_COL_WIDTH = 220;
const TOTAL_COL_WIDTH = 160;
const HEADER_HEIGHT = 72;

const cellStatus = (cell) => {
  if (!cell) return 'empty';
  if (cell.excused) return 'excused';
  if (cell.late) return 'late';
  if (cell.missing || cell.workflow_state === 'unsubmitted') return 'missing';
  if (cell.workflow_state === 'graded' && cell.score != null) return 'graded';
  if (cell.workflow_state === 'submitted' || cell.submitted_at) return 'submitted';
  return 'empty';
};

const statusBg = {
  graded: 'bg-accent-success/10',
  submitted: 'bg-accent-warning/10',
  missing: 'bg-accent-danger/10 ring-1 ring-inset ring-red-300',
  late: 'bg-accent-warning/10',
  excused: 'bg-surface-1',
  empty: '',
};

// ---------------------------------------------------------------------------
// Bulk-grade dialogs
// ---------------------------------------------------------------------------

const SetDefaultGradeDialog = ({ open, onOpenChange, assignment, students, getCellData, onApply }) => {
  const [score, setScore] = useState('0');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (open) {
      setScore('0');
      setError(null);
    }
  }, [open]);

  const ungraded = useMemo(() => {
    if (!assignment) return [];
    return students.filter((s) => {
      const cell = getCellData(s.id, assignment.id);
      return !cell || cell.score == null;
    });
  }, [assignment, students, getCellData]);

  const handleConfirm = async () => {
    if (!assignment) return;
    setSubmitting(true);
    setError(null);
    try {
      const num = parseFloat(score);
      if (isNaN(num)) throw new Error('Score must be a number');
      const grades = ungraded.map((s) => ({
        assignment_id: assignment.id,
        user_id: s.id,
        posted_grade: String(num),
      }));
      await onApply(grades, num);
      onOpenChange(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Set Default Grade</DialogTitle>
          <DialogDescription>
            Apply this score to all students with no submission yet
            {assignment ? ` for "${assignment.name}"` : ''}.
            <span className="block mt-1 text-xs">
              {ungraded.length} student{ungraded.length === 1 ? '' : 's'} will be updated.
            </span>
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <label className="text-sm font-medium">
            Score (out of {assignment?.points_possible ?? 0})
          </label>
          <input
            type="number"
            step="0.01"
            min="0"
            value={score}
            onChange={(e) => setScore(e.target.value)}
            className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:outline-none"
            autoFocus
          />
          {error && <p className="text-sm text-accent-danger">{error}</p>}
        </div>
        <DialogFooter>
          <button
            onClick={() => onOpenChange(false)}
            className="px-4 py-2 text-sm border border-border-strong rounded-md hover:bg-surface-1"
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={submitting || ungraded.length === 0}
            className="px-4 py-2 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700 disabled:opacity-50"
          >
            {submitting ? 'Applying…' : `Apply to ${ungraded.length}`}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const CurveDialog = ({ open, onOpenChange, assignment, students, getCellData, onApply }) => {
  const [targetAvg, setTargetAvg] = useState('70');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (open) {
      setError(null);
      setTargetAvg('70');
    }
  }, [open]);

  const stats = useMemo(() => {
    if (!assignment) return { count: 0, avg: 0, max: 0 };
    const scores = students
      .map((s) => getCellData(s.id, assignment.id))
      .filter((c) => c && c.score != null)
      .map((c) => parseFloat(c.score));
    if (scores.length === 0) return { count: 0, avg: 0, max: assignment.points_possible ?? 0 };
    const avg = scores.reduce((a, b) => a + b, 0) / scores.length;
    return { count: scores.length, avg, max: assignment.points_possible ?? 0 };
  }, [assignment, students, getCellData]);

  const handleConfirm = async () => {
    if (!assignment) return;
    setSubmitting(true);
    setError(null);
    try {
      const target = parseFloat(targetAvg);
      const max = assignment.points_possible ?? 0;
      if (isNaN(target)) throw new Error('Target average must be a number');
      const offset = target - stats.avg;
      const grades = [];
      for (const s of students) {
        const cell = getCellData(s.id, assignment.id);
        if (!cell || cell.score == null) continue;
        const orig = parseFloat(cell.score);
        let curved = orig + offset;
        if (max > 0) curved = Math.max(0, Math.min(max, curved));
        else curved = Math.max(0, curved);
        curved = Math.round(curved * 100) / 100;
        grades.push({
          assignment_id: assignment.id,
          user_id: s.id,
          posted_grade: String(curved),
        });
      }
      await onApply(grades);
      onOpenChange(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Curve Grades</DialogTitle>
          <DialogDescription>
            Shifts every existing score by (target average − current average). Out-of-bounds scores are clamped to 0…max.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="grid grid-cols-3 gap-3 text-sm bg-surface-1 p-3 rounded-md">
            <div>
              <div className="text-xs text-text-tertiary">Graded</div>
              <div className="font-medium">{stats.count}</div>
            </div>
            <div>
              <div className="text-xs text-text-tertiary">Current avg</div>
              <div className="font-medium">{stats.avg.toFixed(2)}</div>
            </div>
            <div>
              <div className="text-xs text-text-tertiary">Max points</div>
              <div className="font-medium">{stats.max}</div>
            </div>
          </div>
          <label className="text-sm font-medium block">Target average</label>
          <input
            type="number"
            step="0.01"
            value={targetAvg}
            onChange={(e) => setTargetAvg(e.target.value)}
            className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:outline-none"
            autoFocus
          />
          {error && <p className="text-sm text-accent-danger">{error}</p>}
        </div>
        <DialogFooter>
          <button
            onClick={() => onOpenChange(false)}
            className="px-4 py-2 text-sm border border-border-strong rounded-md hover:bg-surface-1"
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={submitting || stats.count === 0}
            className="px-4 py-2 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700 disabled:opacity-50"
          >
            {submitting ? 'Curving…' : `Curve ${stats.count} scores`}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const MessageStudentsDialog = ({ open, onOpenChange, assignment, students, getCellData, onSend }) => {
  const [filter, setFilter] = useState('not_submitted');
  const [threshold, setThreshold] = useState('70');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (open && assignment) {
      setSubject(`Re: ${assignment.name}`);
      setBody('');
      setError(null);
    }
  }, [open, assignment]);

  const recipients = useMemo(() => {
    if (!assignment) return [];
    const t = parseFloat(threshold);
    return students.filter((s) => {
      const cell = getCellData(s.id, assignment.id);
      const score = cell && cell.score != null ? parseFloat(cell.score) : null;
      if (filter === 'not_submitted') {
        return !cell || cell.score == null;
      }
      if (filter === 'less_than') return score != null && !isNaN(t) && score < t;
      if (filter === 'greater_than') return score != null && !isNaN(t) && score > t;
      return false;
    });
  }, [assignment, students, getCellData, filter, threshold]);

  const handleSend = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await onSend({
        recipientIds: recipients.map((r) => r.id),
        subject,
        body,
      });
      onOpenChange(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Message Students Who…</DialogTitle>
          <DialogDescription>
            Send a message to a filtered group of students based on this assignment.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-medium block mb-1">Filter</label>
              <select
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="w-full border border-border-strong rounded-md px-2 py-1.5 text-sm"
              >
                <option value="not_submitted">Haven&apos;t submitted</option>
                <option value="less_than">Scored less than…</option>
                <option value="greater_than">Scored greater than…</option>
              </select>
            </div>
            {filter !== 'not_submitted' && (
              <div>
                <label className="text-xs font-medium block mb-1">Score threshold</label>
                <input
                  type="number"
                  step="0.01"
                  value={threshold}
                  onChange={(e) => setThreshold(e.target.value)}
                  className="w-full border border-border-strong rounded-md px-2 py-1.5 text-sm"
                />
              </div>
            )}
          </div>
          <div className="text-xs text-text-secondary bg-surface-1 rounded p-2">
            {recipients.length} recipient{recipients.length === 1 ? '' : 's'}:{' '}
            {recipients.slice(0, 6).map((r) => r.name).join(', ')}
            {recipients.length > 6 ? ` + ${recipients.length - 6} more` : ''}
          </div>
          <div>
            <label className="text-xs font-medium block mb-1">Subject</label>
            <input
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              className="w-full border border-border-strong rounded-md px-2 py-1.5 text-sm"
            />
          </div>
          <div>
            <label className="text-xs font-medium block mb-1">Message</label>
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              rows={5}
              className="w-full border border-border-strong rounded-md px-2 py-1.5 text-sm"
            />
          </div>
          {error && <p className="text-sm text-accent-danger">{error}</p>}
        </div>
        <DialogFooter>
          <button
            onClick={() => onOpenChange(false)}
            className="px-4 py-2 text-sm border border-border-strong rounded-md hover:bg-surface-1"
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            onClick={handleSend}
            disabled={submitting || recipients.length === 0 || !subject || !body}
            className="px-4 py-2 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700 disabled:opacity-50"
          >
            {submitting ? 'Sending…' : `Send to ${recipients.length}`}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const KeyboardShortcutsDialog = ({ open, onOpenChange }) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Keyboard Shortcuts</DialogTitle>
        <DialogDescription>Move and edit grades without touching the mouse.</DialogDescription>
      </DialogHeader>
      <ul className="text-sm space-y-2">
        {[
          ['↑ ↓ ← →', 'Move between cells'],
          ['Enter', 'Edit focused cell'],
          ['Tab / Shift+Tab', 'Move to next / previous cell'],
          ['Escape', 'Cancel edit'],
          ['Home / End', 'Jump to first / last cell in row'],
          ['g g', 'Jump to top of column'],
          ['g e', 'Jump to bottom of column'],
          ['?', 'Open this help dialog'],
        ].map(([keys, desc]) => (
          <li key={keys} className="flex items-center justify-between border-b border-border-subtle pb-1">
            <kbd className="font-mono text-xs bg-surface-2 px-2 py-0.5 rounded border border-border-strong">
              {keys}
            </kbd>
            <span className="text-text-secondary">{desc}</span>
          </li>
        ))}
      </ul>
      <DialogFooter>
        <button
          onClick={() => onOpenChange(false)}
          className="px-4 py-2 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700"
        >
          Close
        </button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);

// ---------------------------------------------------------------------------
// Column header dropdown menu
// ---------------------------------------------------------------------------

const ColumnMenu = memo(function ColumnMenu({
  assignment,
  onSetDefault,
  onCurve,
  onMessage,
  onPost,
  onHide,
  onSortAsc,
  onSortDesc,
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Options for ${assignment.name}`}
          className="rounded p-0.5 text-text-tertiary hover:bg-border-default hover:text-text-primary focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          <MoreVertical className="h-3.5 w-3.5" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="truncate" title={assignment.name}>
          {assignment.name}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => onSetDefault(assignment)}>
          <Sparkles className="h-4 w-4 text-text-tertiary" />
          Set Default Grade…
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onCurve(assignment)}>
          <Calculator className="h-4 w-4 text-text-tertiary" />
          Curve Grades…
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onMessage(assignment)}>
          <Send className="h-4 w-4 text-text-tertiary" />
          Message Students Who…
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {assignment.gradesPosted ? (
          <DropdownMenuItem onSelect={() => onHide(assignment.id)}>
            <EyeOff className="h-4 w-4 text-text-tertiary" />
            Hide Grades
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onSelect={() => onPost(assignment.id)}>
            <Eye className="h-4 w-4 text-text-tertiary" />
            Post Grades
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => onSortAsc(assignment)}>
          <ArrowUpAZ className="h-4 w-4 text-text-tertiary" />
          Sort by score (asc)
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => onSortDesc(assignment)}>
          <ArrowDownAZ className="h-4 w-4 text-text-tertiary" />
          Sort by score (desc)
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
});

// ---------------------------------------------------------------------------
// Cells & headers
// ---------------------------------------------------------------------------

const GradebookCell = memo(function GradebookCell({
  rowIndex,
  columnIndex,
  style,
  ariaAttributes,
  student,
  assignment,
  cell,
  onCommit,
  isFocused,
  onFocusCell,
}) {
  const status = cellStatus(cell);
  const isExcused = status === 'excused';
  const isMissing = status === 'missing';
  const isLate = status === 'late';

  return (
    <div
      {...ariaAttributes}
      role="gridcell"
      aria-rowindex={rowIndex + 2}
      aria-colindex={columnIndex + 3}
      data-cell-row={rowIndex}
      data-cell-col={columnIndex}
      tabIndex={isFocused ? 0 : -1}
      onClick={() => onFocusCell(rowIndex, columnIndex)}
      style={style}
      className={`relative flex items-center justify-center border-b border-r border-border-subtle px-2 ${statusBg[status]} ${
        isFocused ? 'ring-2 ring-brand-500 ring-offset-1 z-10' : ''
      }`}
    >
      {isExcused ? (
        <span className="text-xs text-text-disabled italic">EX</span>
      ) : (
        <div className={isMissing ? 'line-through text-accent-danger' : ''}>
          <GradeInput
            value={cell?.score ?? null}
            pointsPossible={assignment.points_possible ?? 0}
            studentName={student.name}
            onSave={(score) => onCommit(student.id, assignment.id, score)}
          />
        </div>
      )}
      {isLate && (
        <span
          aria-label="Late submission"
          title="Late"
          className="absolute top-1 right-1 h-1.5 w-1.5 rounded-full bg-accent-danger"
        />
      )}
    </div>
  );
});

const StudentRowHeader = memo(function StudentRowHeader({
  student,
  total,
  letterGrade,
  rowIndex,
}) {
  return (
    <div
      role="row"
      aria-rowindex={rowIndex + 2}
      className="flex items-stretch border-b border-border-subtle bg-surface-0 hover:bg-surface-1"
      style={{ height: ROW_HEIGHT }}
    >
      <div
        role="rowheader"
        aria-colindex={1}
        className="flex items-center px-4 text-sm font-medium text-text-primary border-r border-border-default truncate"
        style={{ width: STUDENT_COL_WIDTH, minWidth: STUDENT_COL_WIDTH }}
        title={student.name}
      >
        {student.name}
      </div>
      <div
        role="gridcell"
        aria-colindex={2}
        className="flex items-center justify-center text-xs px-3 border-r border-border-default whitespace-nowrap"
        style={{ width: TOTAL_COL_WIDTH, minWidth: TOTAL_COL_WIDTH }}
      >
        {total.percentage !== null ? (
          <span className="font-medium">
            {total.earned}/{total.possible}
            <span className="text-text-tertiary ml-1">({total.percentage}%)</span>
            <span className={`ml-2 font-semibold ${gradeColor(letterGrade)}`}>{letterGrade}</span>
          </span>
        ) : (
          <span className="text-text-disabled">&mdash;</span>
        )}
      </div>
    </div>
  );
});

const AssignmentColumnHeader = memo(function AssignmentColumnHeader({
  assignment,
  courseId,
  postingAssignment,
  onPost,
  onHide,
  onSetDefault,
  onCurve,
  onMessage,
  onSortAsc,
  onSortDesc,
}) {
  return (
    <div
      role="columnheader"
      aria-sort="none"
      className="flex flex-col items-center justify-center px-2 py-2 text-xs border-r border-border-default bg-surface-1 relative"
      style={{ width: COL_WIDTH, minWidth: COL_WIDTH, height: HEADER_HEIGHT }}
    >
      <div className="absolute top-1 right-1">
        <ColumnMenu
          assignment={assignment}
          onSetDefault={onSetDefault}
          onCurve={onCurve}
          onMessage={onMessage}
          onPost={onPost}
          onHide={onHide}
          onSortAsc={onSortAsc}
          onSortDesc={onSortDesc}
        />
      </div>
      <Link
        to={`/courses/${courseId}/assignments/${assignment.id}`}
        className="text-brand-600 hover:underline block truncate w-full text-center font-medium pr-4"
        title={assignment.name}
      >
        {assignment.name}
      </Link>
      <div className="text-text-tertiary mt-0.5">{assignment.points_possible ?? 0} pts</div>
      {assignment.post_policy === 'manual' && (
        <button
          onClick={() =>
            assignment.gradesPosted ? onHide(assignment.id) : onPost(assignment.id)
          }
          disabled={postingAssignment === assignment.id}
          className={`mt-1 text-[10px] px-1.5 py-0.5 rounded ${
            assignment.gradesPosted
              ? 'bg-accent-success/20 text-accent-success hover:bg-green-200'
              : 'bg-orange-100 text-orange-700 hover:bg-orange-200'
          } transition-colors`}
        >
          {postingAssignment === assignment.id
            ? '...'
            : assignment.gradesPosted
            ? 'Posted'
            : 'Post'}
        </button>
      )}
    </div>
  );
});

const FrozenHeaderRow = memo(function FrozenHeaderRow({
  assignments,
  courseId,
  postingAssignment,
  onPost,
  onHide,
  onSetDefault,
  onCurve,
  onMessage,
  onSortAsc,
  onSortDesc,
  scrollRef,
  width,
}) {
  return (
    <div className="sticky top-0 z-20 bg-surface-1 border-b border-border-default">
      <div className="flex">
        <div
          role="columnheader"
          aria-colindex={1}
          aria-sort="ascending"
          className="flex items-center px-4 font-semibold text-sm text-text-secondary border-r border-border-default bg-surface-1"
          style={{ width: STUDENT_COL_WIDTH, minWidth: STUDENT_COL_WIDTH, height: HEADER_HEIGHT }}
        >
          Student
        </div>
        <div
          role="columnheader"
          aria-colindex={2}
          aria-sort="none"
          className="flex items-center justify-center font-semibold text-sm text-text-secondary border-r border-border-default bg-surface-1"
          style={{ width: TOTAL_COL_WIDTH, minWidth: TOTAL_COL_WIDTH, height: HEADER_HEIGHT }}
        >
          Total / Grade
        </div>
        <div ref={scrollRef} className="overflow-hidden" style={{ width: width || '100%' }}>
          <div className="flex" style={{ width: assignments.length * COL_WIDTH }}>
            {assignments.map((a) => (
              <AssignmentColumnHeader
                key={a.id}
                assignment={a}
                courseId={courseId}
                postingAssignment={postingAssignment}
                onPost={onPost}
                onHide={onHide}
                onSetDefault={onSetDefault}
                onCurve={onCurve}
                onMessage={onMessage}
                onSortAsc={onSortAsc}
                onSortDesc={onSortDesc}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
});

const FrozenStudentColumn = memo(function FrozenStudentColumn({
  students,
  totals,
  letterGrades,
  scrollRef,
  height,
}) {
  return (
    <div
      ref={scrollRef}
      className="overflow-hidden border-r border-border-default bg-surface-0"
      style={{
        width: STUDENT_COL_WIDTH + TOTAL_COL_WIDTH,
        minWidth: STUDENT_COL_WIDTH + TOTAL_COL_WIDTH,
        height,
      }}
    >
      <div style={{ height: students.length * ROW_HEIGHT }}>
        {students.map((s, i) => (
          <StudentRowHeader
            key={s.id}
            student={s}
            total={totals[s.id]}
            letterGrade={letterGrades[s.id]}
            rowIndex={i}
          />
        ))}
      </div>
    </div>
  );
});

// ---------------------------------------------------------------------------
// Virtualized grid
// ---------------------------------------------------------------------------

const GradebookGrid = ({
  students,
  assignments,
  totals,
  letterGrades,
  gradebook,
  courseId,
  postingAssignment,
  onPost,
  onHide,
  onCommit,
  onSetDefault,
  onCurve,
  onMessage,
  onSortAsc,
  onSortDesc,
  focusedCell,
  onFocusCell,
  height,
  width,
  gridContainerRef,
}) => {
  const gridRef = useGridRef();
  const headerScrollRef = useRef(null);
  const studentScrollRef = useRef(null);

  const getCellData = useCallback(
    (studentId, assignmentId) => {
      if (!gradebook) return null;
      const subs = gradebook.submissions || gradebook;
      const studentSubs = subs[studentId] || subs[String(studentId)];
      if (!studentSubs) return null;
      return studentSubs[assignmentId] || studentSubs[String(assignmentId)] || null;
    },
    [gradebook]
  );

  const cellProps = useMemo(
    () => ({ students, assignments, getCellData, onCommit, focusedCell, onFocusCell }),
    [students, assignments, getCellData, onCommit, focusedCell, onFocusCell]
  );

  const Cell = useCallback(
    ({ rowIndex, columnIndex, style, ariaAttributes, students: s, assignments: a, getCellData: g, onCommit: c, focusedCell: fc, onFocusCell: ofc }) => {
      const student = s[rowIndex];
      const assignment = a[columnIndex];
      if (!student || !assignment) return <div style={style} />;
      const cell = g(student.id, assignment.id);
      const isFocused = fc && fc.row === rowIndex && fc.col === columnIndex;
      return (
        <GradebookCell
          rowIndex={rowIndex}
          columnIndex={columnIndex}
          style={style}
          ariaAttributes={ariaAttributes}
          student={student}
          assignment={assignment}
          cell={cell}
          onCommit={c}
          isFocused={isFocused}
          onFocusCell={ofc}
        />
      );
    },
    []
  );

  const handleGridScroll = useCallback((e) => {
    const { scrollLeft, scrollTop } = e.currentTarget;
    if (headerScrollRef.current) headerScrollRef.current.scrollLeft = scrollLeft;
    if (studentScrollRef.current) studentScrollRef.current.scrollTop = scrollTop;
  }, []);

  useEffect(() => {
    const grid = gridRef.current?.element;
    if (!grid) return;
    grid.addEventListener('scroll', handleGridScroll, { passive: true });
    return () => grid.removeEventListener('scroll', handleGridScroll);
  }, [gridRef, handleGridScroll]);

  // Scroll into view + focus when focusedCell changes
  useEffect(() => {
    if (!focusedCell) return;
    const apiObj = gridRef.current;
    if (apiObj && typeof apiObj.scrollToCell === 'function') {
      apiObj.scrollToCell({
        rowIndex: focusedCell.row,
        columnIndex: focusedCell.col,
        rowAlign: 'smart',
        columnAlign: 'smart',
      });
    }
    // After scroll, focus the rendered cell
    requestAnimationFrame(() => {
      const grid = gridRef.current?.element;
      if (!grid) return;
      const el = grid.querySelector(
        `[data-cell-row="${focusedCell.row}"][data-cell-col="${focusedCell.col}"]`
      );
      if (el) {
        const btn = el.querySelector('button[type="button"]');
        if (btn) btn.focus();
        else el.focus();
      }
    });
  }, [focusedCell, gridRef]);

  const gridWidth = Math.max(width - (STUDENT_COL_WIDTH + TOTAL_COL_WIDTH), 200);
  const bodyHeight = Math.max(height - HEADER_HEIGHT, 200);

  return (
    <div
      ref={gridContainerRef}
      role="grid"
      aria-rowcount={students.length + 1}
      aria-colcount={assignments.length + 2}
      className="bg-surface-0 rounded-lg shadow border border-border-default overflow-hidden"
      style={{ width, height }}
    >
      <FrozenHeaderRow
        assignments={assignments}
        courseId={courseId}
        postingAssignment={postingAssignment}
        onPost={onPost}
        onHide={onHide}
        onSetDefault={onSetDefault}
        onCurve={onCurve}
        onMessage={onMessage}
        onSortAsc={onSortAsc}
        onSortDesc={onSortDesc}
        scrollRef={headerScrollRef}
        width={gridWidth}
      />
      <div className="flex" style={{ height: bodyHeight }}>
        <FrozenStudentColumn
          students={students}
          totals={totals}
          letterGrades={letterGrades}
          scrollRef={studentScrollRef}
          height={bodyHeight}
        />
        <Grid
          gridRef={gridRef}
          cellComponent={Cell}
          cellProps={cellProps}
          columnCount={assignments.length}
          columnWidth={COL_WIDTH}
          rowCount={students.length}
          rowHeight={ROW_HEIGHT}
          defaultHeight={bodyHeight}
          defaultWidth={gridWidth}
          overscanCount={4}
          style={{ height: bodyHeight, width: gridWidth }}
        />
      </div>
    </div>
  );
};

const GradebookSkeleton = () => (
  <div className="bg-surface-0 rounded-lg shadow p-4 space-y-2">
    <div className="flex gap-2">
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-10 flex-1" />
      ))}
    </div>
    {Array.from({ length: 5 }).map((_, r) => (
      <div key={r} className="flex gap-2">
        {Array.from({ length: 5 }).map((_, c) => (
          <Skeleton key={c} className="h-9 flex-1" />
        ))}
      </div>
    ))}
  </div>
);

const Legend = () => (
  <div className="flex items-center gap-2 flex-wrap">
    <Badge className="border-green-300 bg-accent-success/10 text-accent-success">Graded</Badge>
    <Badge className="border-yellow-300 bg-accent-warning/10 text-accent-warning">Submitted</Badge>
    <Badge className="border-red-300 bg-accent-danger/10 text-accent-danger">Missing</Badge>
    <Badge className="border-amber-300 bg-accent-warning/10 text-amber-800">
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-accent-danger mr-1" />
      Late
    </Badge>
    <Badge className="border-border-strong bg-surface-1 text-text-secondary italic">EX = Excused</Badge>
  </div>
);

const colLabel = (i) => {
  // Excel-style: 0 -> A, 25 -> Z, 26 -> AA
  let s = '';
  let n = i;
  while (n >= 0) {
    s = String.fromCharCode((n % 26) + 65) + s;
    n = Math.floor(n / 26) - 1;
  }
  return s;
};

const GradebookPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [course, setCourse] = useState(null);
  const [gradebook, setGradebook] = useState(null);
  const [assignments, setAssignments] = useState([]);
  const [assignmentGroups, setAssignmentGroups] = useState([]);
  const [students, setStudents] = useState([]);
  const [gradingScale, setGradingScale] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState(null);
  const [postingAssignment, setPostingAssignment] = useState(null);
  const [viewport, setViewport] = useState({ width: 1200, height: 600 });
  const containerRef = useRef(null);
  const gridContainerRef = useRef(null);
  const lastGKeyRef = useRef(0);

  // Bulk-action dialog state
  const [defaultGradeDialog, setDefaultGradeDialog] = useState({ open: false, assignment: null });
  const [curveDialog, setCurveDialog] = useState({ open: false, assignment: null });
  const [messageDialog, setMessageDialog] = useState({ open: false, assignment: null });
  const [shortcutsDialog, setShortcutsDialog] = useState(false);

  // Sort override
  const [sortOverride, setSortOverride] = useState(null); // { assignmentId, direction }

  // Keyboard cell focus
  const [focusedCell, setFocusedCell] = useState({ row: 0, col: 0 });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [courseData, assignmentResult, enrollmentResult, assignmentGroupResult] = await Promise.all([
          api.getCourse(courseId),
          api.getAssignments(courseId, 1, 100),
          api.getEnrollments(courseId, 1, 200),
          api.getAssignmentGroups(courseId, 1, 50).catch(() => ({ data: [] })),
        ]);

        setCourse(courseData);
        setAssignments(assignmentResult.data || []);

        try {
          const standards = await api.getGradingStandards(courseId);
          if (Array.isArray(standards) && standards.length > 0) {
            const latest = standards[standards.length - 1];
            if (Array.isArray(latest.data)) setGradingScale(latest.data);
          }
        } catch {}

        setAssignmentGroups(assignmentGroupResult.data || []);

        const enrollmentList = enrollmentResult.data || [];
        const studentEnrollments = enrollmentList.filter(
          (e) =>
            e.type === 'StudentEnrollment' ||
            e.role === 'StudentEnrollment' ||
            e.enrollment_type === 'student'
        );

        const seen = new Set();
        const uniqueStudents = [];
        for (const enrollment of studentEnrollments) {
          const uid = enrollment.user_id || enrollment.user?.id;
          if (uid && !seen.has(uid)) {
            seen.add(uid);
            uniqueStudents.push({
              id: uid,
              name: enrollment.user?.name || enrollment.user?.display_name || `User ${uid}`,
              sortable_name: enrollment.user?.sortable_name || enrollment.user?.name || `User ${uid}`,
            });
          }
        }
        uniqueStudents.sort((a, b) => a.sortable_name.localeCompare(b.sortable_name));
        setStudents(uniqueStudents);

        try {
          const gb = await api.getGradebook(courseId);
          setGradebook(gb);
        } catch {
          const subResult = await api.getCourseSubmissions(courseId);
          const subs = subResult.data || [];
          const gradebookData = {};
          for (const sub of subs) {
            const uid = sub.user_id;
            if (!gradebookData[uid]) gradebookData[uid] = {};
            gradebookData[uid][sub.assignment_id] = {
              score: sub.score,
              grade: sub.grade,
              workflow_state: sub.workflow_state,
              submitted_at: sub.submitted_at,
              late: sub.late,
              missing: sub.missing,
              excused: sub.excused,
            };
          }
          setGradebook(gradebookData);
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId]);

  useEffect(() => {
    const measure = () => {
      const el = containerRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      setViewport({
        width: Math.max(rect.width, 600),
        height: Math.max(window.innerHeight - rect.top - 80, 480),
      });
    };
    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  }, [loading]);

  const orderedAssignments = useMemo(() => {
    if (assignmentGroups.length === 0) return assignments;
    const groupOrder = new Map(assignmentGroups.map((g, i) => [g.id, i]));
    return [...assignments].sort((a, b) => {
      const ga = groupOrder.has(a.assignment_group_id) ? groupOrder.get(a.assignment_group_id) : Infinity;
      const gb = groupOrder.has(b.assignment_group_id) ? groupOrder.get(b.assignment_group_id) : Infinity;
      if (ga !== gb) return ga - gb;
      return (a.position ?? 0) - (b.position ?? 0);
    });
  }, [assignments, assignmentGroups]);

  const getCellData = useCallback(
    (studentId, assignmentId) => {
      if (!gradebook) return null;
      const subs = gradebook.submissions || gradebook;
      const studentSubs = subs[studentId] || subs[String(studentId)];
      if (!studentSubs) return null;
      return studentSubs[assignmentId] || studentSubs[String(assignmentId)] || null;
    },
    [gradebook]
  );

  const studentRows = useMemo(() => {
    if (!sortOverride) return students;
    const { assignmentId, direction } = sortOverride;
    const sign = direction === 'asc' ? 1 : -1;
    const arr = [...students];
    arr.sort((a, b) => {
      const ca = getCellData(a.id, assignmentId);
      const cb = getCellData(b.id, assignmentId);
      const sa = ca && ca.score != null ? parseFloat(ca.score) : Number.NEGATIVE_INFINITY;
      const sb = cb && cb.score != null ? parseFloat(cb.score) : Number.NEGATIVE_INFINITY;
      if (sa === sb) return a.sortable_name.localeCompare(b.sortable_name);
      return (sa - sb) * sign;
    });
    return arr;
  }, [students, sortOverride, getCellData]);

  const totals = useMemo(() => {
    const result = {};
    const useWeights =
      course?.apply_assignment_group_weights &&
      assignmentGroups.length > 0 &&
      assignmentGroups.some((g) => g.group_weight > 0);

    for (const student of students) {
      if (useWeights) {
        let weightedSum = 0;
        let weightTotal = 0;
        let totalEarned = 0;
        let totalPossible = 0;
        const groupedIds = new Set();

        for (const group of assignmentGroups) {
          if (!group.group_weight || group.group_weight <= 0) continue;
          const groupAssignments = assignments.filter((a) => a.assignment_group_id === group.id);
          groupAssignments.forEach((a) => groupedIds.add(a.id));
          let groupEarned = 0;
          let groupPossible = 0;
          for (const a of groupAssignments) {
            const cell = getCellData(student.id, a.id);
            if (cell && cell.score != null) {
              groupEarned += parseFloat(cell.score) || 0;
              groupPossible += parseFloat(a.points_possible) || 0;
            }
          }
          totalEarned += groupEarned;
          totalPossible += groupPossible;
          if (groupPossible > 0) {
            weightedSum += (groupEarned / groupPossible) * 100 * group.group_weight;
            weightTotal += group.group_weight;
          }
        }

        for (const a of assignments) {
          if (groupedIds.has(a.id)) continue;
          const cell = getCellData(student.id, a.id);
          if (cell && cell.score != null) {
            totalEarned += parseFloat(cell.score) || 0;
            totalPossible += parseFloat(a.points_possible) || 0;
          }
        }

        result[student.id] =
          weightTotal === 0
            ? { earned: 0, possible: 0, percentage: null, weighted: true }
            : {
                earned: totalEarned,
                possible: totalPossible,
                percentage: (weightedSum / weightTotal).toFixed(1),
                weighted: true,
              };
      } else {
        let earned = 0;
        let possible = 0;
        for (const a of assignments) {
          const cell = getCellData(student.id, a.id);
          if (cell && cell.score != null) {
            earned += parseFloat(cell.score) || 0;
            possible += parseFloat(a.points_possible) || 0;
          }
        }
        result[student.id] =
          possible === 0
            ? { earned: 0, possible: 0, percentage: null, weighted: false }
            : { earned, possible, percentage: ((earned / possible) * 100).toFixed(1), weighted: false };
      }
    }
    return result;
  }, [students, assignments, assignmentGroups, course, getCellData]);

  const letterGrades = useMemo(() => {
    const out = {};
    for (const s of students) out[s.id] = getLetterGrade(totals[s.id]?.percentage, gradingScale);
    return out;
  }, [students, totals, gradingScale]);

  const handleCommit = useCallback(
    async (studentId, assignmentId, score) => {
      await api.gradeSubmission(courseId, assignmentId, studentId, { posted_grade: String(score) });
      setGradebook((prev) => {
        if (!prev) return prev;
        const sid = String(studentId);
        const aid = String(assignmentId);
        const hasSubmissions = !!prev.submissions;
        const oldSubs = hasSubmissions ? prev.submissions : prev;
        const oldStudent = oldSubs[sid] || {};
        const newCell = {
          ...(oldStudent[aid] || {}),
          score: score == null ? null : parseFloat(score),
          grade: score == null ? null : String(score),
          workflow_state: score != null ? 'graded' : 'unsubmitted',
        };
        const newStudent = { ...oldStudent, [aid]: newCell };
        const newSubs = { ...oldSubs, [sid]: newStudent };
        return hasSubmissions ? { ...prev, submissions: newSubs } : newSubs;
      });
    },
    [courseId]
  );

  const handlePostGrades = useCallback(
    async (assignmentId) => {
      setPostingAssignment(assignmentId);
      try {
        await api.postGrades(courseId, assignmentId);
        setAssignments((prev) =>
          prev.map((a) => (a.id === assignmentId ? { ...a, gradesPosted: true } : a))
        );
      } catch (err) {
        setError(err.message);
      } finally {
        setPostingAssignment(null);
      }
    },
    [courseId]
  );

  const handleHideGrades = useCallback(
    async (assignmentId) => {
      setPostingAssignment(assignmentId);
      try {
        await api.hideGrades(courseId, assignmentId);
        setAssignments((prev) =>
          prev.map((a) => (a.id === assignmentId ? { ...a, gradesPosted: false } : a))
        );
      } catch (err) {
        setError(err.message);
      } finally {
        setPostingAssignment(null);
      }
    },
    [courseId]
  );

  // ---------- Bulk grade application (Set Default / Curve) ----------
  const applyBulkGrades = useCallback(
    async (grades) => {
      if (!grades || grades.length === 0) return;
      // Snapshot for rollback
      let snapshot;
      setGradebook((prev) => {
        snapshot = prev;
        return prev;
      });
      try {
        if (typeof api.bulkGrade === 'function') {
          await api.bulkGrade(courseId, grades);
        } else {
          // TODO: replace with real bulk endpoint when available
          for (const g of grades) {
            await api.gradeSubmission(courseId, g.assignment_id, g.user_id, { posted_grade: g.posted_grade });
          }
        }
        // Optimistically merge into local state
        setGradebook((prev) => {
          if (!prev) return prev;
          const hasSubmissions = !!prev.submissions;
          const oldSubs = hasSubmissions ? prev.submissions : prev;
          const newSubs = { ...oldSubs };
          for (const g of grades) {
            const sid = String(g.user_id);
            const aid = String(g.assignment_id);
            const num = parseFloat(g.posted_grade);
            const oldStudent = newSubs[sid] || {};
            newSubs[sid] = {
              ...oldStudent,
              [aid]: {
                ...(oldStudent[aid] || {}),
                score: isNaN(num) ? null : num,
                grade: g.posted_grade,
                workflow_state: 'graded',
              },
            };
          }
          return hasSubmissions ? { ...prev, submissions: newSubs } : newSubs;
        });
      } catch (err) {
        // Rollback
        setGradebook(snapshot);
        throw err;
      }
    },
    [courseId]
  );

  const handleSendBulkMessage = useCallback(
    async ({ recipientIds, subject, body }) => {
      if (!recipientIds || recipientIds.length === 0) throw new Error('No recipients');
      if (typeof api.createConversation === 'function') {
        await api.createConversation({
          recipients: recipientIds.map((id) => String(id)),
          subject,
          body,
          context_code: `course_${courseId}`,
          group_conversation: false,
        });
      } else {
        // TODO: hook up to a real bulk-message endpoint
        // eslint-disable-next-line no-console
        console.warn('createConversation not available; message not sent');
      }
    },
    [courseId]
  );

  // ---------- Sort handlers ----------
  const handleSortAsc = useCallback((assignment) => {
    setSortOverride({ assignmentId: assignment.id, direction: 'asc' });
  }, []);
  const handleSortDesc = useCallback((assignment) => {
    setSortOverride({ assignmentId: assignment.id, direction: 'desc' });
  }, []);

  // ---------- Dialog openers (memoized) ----------
  const openDefaultGradeDialog = useCallback((assignment) => {
    setDefaultGradeDialog({ open: true, assignment });
  }, []);
  const openCurveDialog = useCallback((assignment) => {
    setCurveDialog({ open: true, assignment });
  }, []);
  const openMessageDialog = useCallback((assignment) => {
    setMessageDialog({ open: true, assignment });
  }, []);

  // ---------- Keyboard cell navigation ----------
  const numRows = studentRows.length;
  const numCols = orderedAssignments.length;

  const moveFocus = useCallback(
    (dRow, dCol) => {
      setFocusedCell((fc) => {
        const row = Math.max(0, Math.min(numRows - 1, fc.row + dRow));
        const col = Math.max(0, Math.min(numCols - 1, fc.col + dCol));
        return { row, col };
      });
    },
    [numRows, numCols]
  );

  useEffect(() => {
    const handler = (e) => {
      // Skip if any dialog is open
      if (defaultGradeDialog.open || curveDialog.open || messageDialog.open || shortcutsDialog) {
        return;
      }
      const tag = (e.target?.tagName || '').toLowerCase();
      const isEditing =
        tag === 'input' || tag === 'textarea' || tag === 'select' || e.target?.isContentEditable;

      // ? opens help dialog (only when not editing text)
      if (e.key === '?' && !isEditing) {
        e.preventDefault();
        setShortcutsDialog(true);
        return;
      }

      if (isEditing) {
        // GradeInput handles Enter/Escape internally; only react to Tab here
        if (e.key === 'Tab') {
          e.preventDefault();
          const dCol = e.shiftKey ? -1 : 1;
          // Defer so GradeInput's onBlur fires first to commit
          setTimeout(() => moveFocus(0, dCol), 0);
        }
        return;
      }

      if (!gridContainerRef.current) return;

      switch (e.key) {
        case 'ArrowUp':
          e.preventDefault();
          moveFocus(-1, 0);
          break;
        case 'ArrowDown':
          e.preventDefault();
          moveFocus(1, 0);
          break;
        case 'ArrowLeft':
          e.preventDefault();
          moveFocus(0, -1);
          break;
        case 'ArrowRight':
          e.preventDefault();
          moveFocus(0, 1);
          break;
        case 'Tab':
          e.preventDefault();
          moveFocus(0, e.shiftKey ? -1 : 1);
          break;
        case 'Home':
          e.preventDefault();
          setFocusedCell((fc) => ({ ...fc, col: 0 }));
          break;
        case 'End':
          e.preventDefault();
          setFocusedCell((fc) => ({ ...fc, col: Math.max(0, numCols - 1) }));
          break;
        case 'Enter': {
          const grid = gridContainerRef.current;
          if (!grid) break;
          const el = grid.querySelector(
            `[data-cell-row="${focusedCell.row}"][data-cell-col="${focusedCell.col}"] button[type="button"]`
          );
          if (el) {
            e.preventDefault();
            el.click();
          }
          break;
        }
        case 'g': {
          const now = Date.now();
          if (now - lastGKeyRef.current < 500) {
            // gg = top
            setFocusedCell((fc) => ({ ...fc, row: 0 }));
            lastGKeyRef.current = 0;
            e.preventDefault();
          } else {
            lastGKeyRef.current = now;
          }
          break;
        }
        case 'e':
          if (Date.now() - lastGKeyRef.current < 500) {
            // ge = bottom
            setFocusedCell((fc) => ({ ...fc, row: Math.max(0, numRows - 1) }));
            lastGKeyRef.current = 0;
            e.preventDefault();
          }
          break;
        default:
          break;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [
    moveFocus,
    focusedCell,
    numRows,
    numCols,
    defaultGradeDialog.open,
    curveDialog.open,
    messageDialog.open,
    shortcutsDialog,
  ]);

  const handleFocusCell = useCallback((row, col) => {
    setFocusedCell({ row, col });
  }, []);

  // Clamp focused cell when grid dimensions shrink
  useEffect(() => {
    setFocusedCell((fc) => ({
      row: Math.min(fc.row, Math.max(0, numRows - 1)),
      col: Math.min(fc.col, Math.max(0, numCols - 1)),
    }));
  }, [numRows, numCols]);

  // ---------- CSV ----------
  const exportGradebookCSV = () => {
    const escCSV = (val) => {
      const s = String(val ?? '');
      return s.includes(',') || s.includes('"') || s.includes('\n')
        ? `"${s.replace(/"/g, '""')}"`
        : s;
    };
    const headers = [
      'Student',
      'Student ID',
      ...orderedAssignments.map((a) => a.name),
      'Total Points',
      'Total Possible',
      'Percentage',
      'Letter Grade',
    ];
    const rows = students.map((student) => {
      const total = totals[student.id];
      const cells = orderedAssignments.map((a) => {
        const cell = getCellData(student.id, a.id);
        return cell && cell.score != null ? cell.score : '';
      });
      return [
        student.name || `User ${student.id}`,
        student.id,
        ...cells,
        total.earned,
        total.possible,
        total.percentage ?? '',
        letterGrades[student.id],
      ];
    });
    const csv = [headers, ...rows].map((row) => row.map(escCSV).join(',')).join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `gradebook_${course?.course_code || courseId}_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const importGradebookCSV = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = '';
    setImporting(true);
    setImportResult(null);
    try {
      const text = await file.text();
      const lines = text.split('\n').filter((l) => l.trim());
      if (lines.length < 2) throw new Error('CSV must have a header row and at least one data row');
      const headers = lines[0].split(',').map((h) => h.trim().replace(/^"(.*)"$/, '$1'));
      const studentIdIdx = headers.findIndex((h) => /student\s*id/i.test(h));
      if (studentIdIdx === -1) throw new Error('CSV must include a "Student ID" column');
      const assignmentMap = {};
      for (const a of assignments) assignmentMap[a.name.toLowerCase().trim()] = a;
      let skipped = 0;
      const gradeData = [];
      for (let i = 1; i < lines.length; i++) {
        const cells = lines[i].split(',').map((c) => c.trim().replace(/^"(.*)"$/, '$1'));
        const studentId = parseInt(cells[studentIdIdx], 10);
        if (!studentId) {
          skipped++;
          continue;
        }
        for (let col = 0; col < headers.length; col++) {
          if (col === 0 || col === studentIdIdx) continue;
          const headerName = headers[col].toLowerCase().trim();
          if (/total|percentage|letter/i.test(headerName)) continue;
          const assignment = assignmentMap[headerName];
          if (!assignment) continue;
          const scoreStr = cells[col];
          if (scoreStr === '' || scoreStr === undefined) continue;
          const score = parseFloat(scoreStr);
          if (isNaN(score)) continue;
          gradeData.push({ assignment_id: assignment.id, user_id: studentId, posted_grade: String(score) });
        }
      }
      const errors = [];
      let updated = 0;
      for (let i = 0; i < gradeData.length; i += 500) {
        const batch = gradeData.slice(i, i + 500);
        try {
          const result = await api.bulkGrade(courseId, batch);
          for (const r of result?.results || []) {
            if (r.error) errors.push(`Assignment ${r.assignment_id}, User ${r.user_id}: ${r.error}`);
            else updated++;
          }
        } catch (err) {
          errors.push(`Batch ${Math.floor(i / 500) + 1}: ${err.message}`);
        }
      }
      setImportResult({ success: true, updated, skipped, errors });
      try {
        const gb = await api.getGradebook(courseId);
        setGradebook(gb);
      } catch {
        const subResult = await api.getCourseSubmissions(courseId);
        const subs = subResult.data || [];
        const gradebookData = {};
        for (const sub of subs) {
          const uid = sub.user_id;
          if (!gradebookData[uid]) gradebookData[uid] = {};
          gradebookData[uid][sub.assignment_id] = {
            score: sub.score,
            grade: sub.grade,
            workflow_state: sub.workflow_state,
            submitted_at: sub.submitted_at,
            late: sub.late,
            missing: sub.missing,
            excused: sub.excused,
          };
        }
        setGradebook(gradebookData);
      }
    } catch (err) {
      setImportResult({ success: false, message: err.message });
    } finally {
      setImporting(false);
    }
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null)
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
          <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
          </svg>
          Loading...
        </div>
      </Layout>
    );

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="mb-4">
          <Skeleton className="h-7 w-64 mb-2" />
          <Skeleton className="h-4 w-40" />
        </div>
        <GradebookSkeleton />
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="text-brand-600 hover:text-brand-800 text-sm font-medium"
          >
            Try Again
          </button>
        </div>
      </Layout>
    );
  }

  const focusedAssignment = orderedAssignments[focusedCell.col];
  const focusedStudent = studentRows[focusedCell.row];

  return (
    <Layout>
      <CourseNav />
      <div className="mb-4">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2 flex-wrap gap-3">
          <div>
            <h2 className="text-2xl font-bold text-text-primary">
              Gradebook{course ? `: ${course.name}` : ''}
            </h2>
            <p className="text-text-tertiary text-sm mt-1">
              {students.length} {students.length === 1 ? 'student' : 'students'},{' '}
              {assignments.length} {assignments.length === 1 ? 'assignment' : 'assignments'}
              {sortOverride && (
                <button
                  onClick={() => setSortOverride(null)}
                  className="ml-2 text-brand-600 hover:underline text-xs"
                >
                  Clear sort
                </button>
              )}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Legend />
            <button
              onClick={() => setShortcutsDialog(true)}
              className="inline-flex items-center gap-2 bg-surface-0 border border-border-strong text-text-secondary px-3 py-2 rounded-md hover:bg-surface-1 text-sm font-medium"
              aria-label="Keyboard shortcuts"
              title="Keyboard shortcuts (?)"
            >
              <HelpCircle className="w-4 h-4" />
            </button>
            <label
              className={`inline-flex items-center gap-2 bg-surface-0 border border-border-strong text-text-secondary px-4 py-2 rounded-md hover:bg-surface-1 text-sm font-medium cursor-pointer ${
                importing ? 'opacity-50 pointer-events-none' : ''
              }`}
            >
              <Upload className="w-4 h-4" />
              {importing ? 'Importing...' : 'Import CSV'}
              <input
                type="file"
                accept=".csv"
                onChange={importGradebookCSV}
                className="hidden"
                disabled={importing}
              />
            </label>
            <button
              onClick={exportGradebookCSV}
              className="inline-flex items-center gap-2 bg-surface-0 border border-border-strong text-text-secondary px-4 py-2 rounded-md hover:bg-surface-1 text-sm font-medium"
            >
              <Download className="w-4 h-4" />
              Export CSV
            </button>
          </div>
        </div>
      </div>

      {importResult && (
        <div
          className={`mb-4 p-3 rounded-md text-sm ${
            importResult.success
              ? 'bg-accent-success/10 text-accent-success border border-accent-success/30'
              : 'bg-accent-danger/10 text-accent-danger border border-accent-danger/30'
          }`}
        >
          {importResult.success ? (
            <div>
              <p className="font-medium">
                Import complete: {importResult.updated} grade
                {importResult.updated !== 1 ? 's' : ''} updated
                {importResult.skipped > 0
                  ? `, ${importResult.skipped} row${importResult.skipped !== 1 ? 's' : ''} skipped`
                  : ''}
              </p>
              {importResult.errors.length > 0 && (
                <details className="mt-1">
                  <summary className="cursor-pointer text-accent-warning">
                    {importResult.errors.length} error
                    {importResult.errors.length !== 1 ? 's' : ''}
                  </summary>
                  <ul className="mt-1 text-xs list-disc pl-4">
                    {importResult.errors.slice(0, 10).map((err, i) => (
                      <li key={i}>{err}</li>
                    ))}
                    {importResult.errors.length > 10 && (
                      <li>...and {importResult.errors.length - 10} more</li>
                    )}
                  </ul>
                </details>
              )}
            </div>
          ) : (
            <p>{importResult.message}</p>
          )}
          <button
            onClick={() => setImportResult(null)}
            className="mt-1 text-xs underline"
          >
            Dismiss
          </button>
        </div>
      )}

      <span className="sr-only" aria-live="polite">
        Gradebook: students by assignments grade matrix.
      </span>

      <div ref={containerRef} className="w-full">
        {students.length === 0 || orderedAssignments.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">
            {students.length === 0 ? 'No students enrolled.' : 'No assignments yet.'}
          </div>
        ) : (
          <GradebookGrid
            students={studentRows}
            assignments={orderedAssignments}
            totals={totals}
            letterGrades={letterGrades}
            gradebook={gradebook}
            courseId={courseId}
            postingAssignment={postingAssignment}
            onPost={handlePostGrades}
            onHide={handleHideGrades}
            onCommit={handleCommit}
            onSetDefault={openDefaultGradeDialog}
            onCurve={openCurveDialog}
            onMessage={openMessageDialog}
            onSortAsc={handleSortAsc}
            onSortDesc={handleSortDesc}
            focusedCell={focusedCell}
            onFocusCell={handleFocusCell}
            height={viewport.height}
            width={viewport.width}
            gridContainerRef={gridContainerRef}
          />
        )}
      </div>

      {/* Status bar */}
      {numRows > 0 && numCols > 0 && (
        <div
          role="status"
          aria-live="polite"
          className="mt-2 flex items-center justify-between text-xs text-text-secondary bg-surface-1 border border-border-default rounded-md px-3 py-1.5"
        >
          <span>
            Cell <strong>{colLabel(focusedCell.col)}{focusedCell.row + 1}</strong> of {numRows}×{numCols}
            {focusedAssignment && focusedStudent && (
              <span className="ml-2 text-text-tertiary">
                — {focusedStudent.name} · {focusedAssignment.name}
              </span>
            )}
          </span>
          <span>
            Press <kbd className="font-mono bg-surface-0 border border-border-strong rounded px-1">?</kbd> for shortcuts
          </span>
        </div>
      )}

      {/* Bulk dialogs */}
      <SetDefaultGradeDialog
        open={defaultGradeDialog.open}
        onOpenChange={(o) => setDefaultGradeDialog((s) => ({ ...s, open: o }))}
        assignment={defaultGradeDialog.assignment}
        students={students}
        getCellData={getCellData}
        onApply={applyBulkGrades}
      />
      <CurveDialog
        open={curveDialog.open}
        onOpenChange={(o) => setCurveDialog((s) => ({ ...s, open: o }))}
        assignment={curveDialog.assignment}
        students={students}
        getCellData={getCellData}
        onApply={applyBulkGrades}
      />
      <MessageStudentsDialog
        open={messageDialog.open}
        onOpenChange={(o) => setMessageDialog((s) => ({ ...s, open: o }))}
        assignment={messageDialog.assignment}
        students={students}
        getCellData={getCellData}
        onSend={handleSendBulkMessage}
      />
      <KeyboardShortcutsDialog open={shortcutsDialog} onOpenChange={setShortcutsDialog} />
    </Layout>
  );
};

export default GradebookPage;
