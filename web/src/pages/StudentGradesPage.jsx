import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { getLetterGrade, gradeColor, assignmentGradeWeight } from '../utils/grading';
import { Calculator, RotateCcw } from 'lucide-react';
import { gradingPolicyApi } from '../services/api_grading_policy';

const isRecentlyGraded = (sub) => {
  if (!sub?.graded_at) return false;
  const gradedAt = new Date(sub.graded_at);
  const cutoff = Date.now() - 48 * 60 * 60 * 1000;
  return gradedAt.getTime() > cutoff;
};

const statusBadge = (sub) => {
  if (sub?.excused) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-600">Excused</span>;
  if (sub?.missing) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-red-100 text-red-700">Missing</span>;
  if (sub?.late) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-red-50 text-red-700">Late</span>;
  if (!sub) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-500">Not submitted</span>;
  if (sub.workflow_state === 'graded') return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-green-100 text-green-700">Graded</span>;
  if (sub.workflow_state === 'submitted' || sub.submitted_at) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-yellow-100 text-yellow-700">Submitted</span>;
  if (sub.workflow_state === 'pending_review') return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-blue-100 text-blue-700">Pending review</span>;
  return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-500">Not submitted</span>;
};

const formatDate = (dateStr) => {
  if (!dateStr) return '-';
  return new Date(dateStr).toLocaleDateString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
  });
};

const StudentGradesPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const { user } = useAuth();
  const [searchParams] = useSearchParams();
  const observeeId = searchParams.get('observee_id');
  const targetUserId = observeeId ? parseInt(observeeId, 10) : user?.id;
  const isObserverView = !!observeeId;
  const [course, setCourse] = useState(null);
  const [observeeName, setObserveeName] = useState(null);
  const [assignments, setAssignments] = useState([]);
  const [groups, setGroups] = useState([]);
  const [submissions, setSubmissions] = useState({});
  const [quizzes, setQuizzes] = useState([]);
  const [quizSubmissions, setQuizSubmissions] = useState({});
  const [gradingScale, setGradingScale] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Server-computed grade (G020/G021/G024/G025/G029) and the student's
  // own effective dates (G019/G040).
  const [serverGrade, setServerGrade] = useState(null);
  const [gradingPeriods, setGradingPeriods] = useState([]);
  const [periodFilter, setPeriodFilter] = useState('all');
  const [myDates, setMyDates] = useState(null); // assignment id -> dates

  // What-If grade calculator state
  const [whatIfMode, setWhatIfMode] = useState(false);
  const [whatIfScores, setWhatIfScores] = useState({}); // keyed by assignment ID

  const fetchData = async () => {
    if (!user || !targetUserId) return;
      try {
        const [courseData, assignmentResult, groupResult, quizResult] = await Promise.all([
          api.getCourse(courseId),
          api.getAssignments(courseId, 1, 200),
          api.getAssignmentGroups(courseId, 1, 50).catch(() => ({ data: [] })),
          api.getQuizzes(courseId, 1, 200).catch(() => ({ data: [] })),
        ]);

        // Fetch grading standard for custom scale
        try {
          const standards = await api.getGradingStandards(courseId);
          if (Array.isArray(standards) && standards.length > 0) {
            const latest = standards[standards.length - 1];
            if (Array.isArray(latest.data)) setGradingScale(latest.data);
          }
        } catch {}

        // If observer view, resolve the observee's name from enrollments
        if (isObserverView) {
          try {
            const enrollResult = await api.getAllEnrollments(courseId);
            const enrollments = enrollResult.data || [];
            const observeeEnrollment = enrollments.find(e =>
              (e.user_id === targetUserId || e.user?.id === targetUserId) && e.type === 'StudentEnrollment'
            );
            if (observeeEnrollment) {
              setObserveeName(observeeEnrollment.user?.name || observeeEnrollment.user_name || `Student #${targetUserId}`);
            }
          } catch {}
        }

        setCourse(courseData);
        const assignmentList = (assignmentResult.data || []).filter(a => a.published !== false);
        setAssignments(assignmentList);
        setGroups(groupResult.data || []);

        const publishedQuizzes = (quizResult.data || []).filter(q => q.published !== false);
        setQuizzes(publishedQuizzes);

        // Fetch all submissions for the target user in one bulk call
        // For observers viewing observee data, pass the observee's user_id
        const userIdParam = isObserverView ? String(targetUserId) : 'self';
        const subMap = {};
        try {
          const subResult = await api.getCourseSubmissions(courseId, 1, 10000, userIdParam);
          const subs = subResult.data || subResult || [];
          for (const sub of (Array.isArray(subs) ? subs : [])) {
            if (sub.user_id === targetUserId || String(sub.user_id) === String(targetUserId)) {
              subMap[sub.assignment_id] = sub;
            }
          }
        } catch {
          // Fallback: fetch per-assignment if bulk endpoint fails
          const batchSize = 6;
          for (let i = 0; i < assignmentList.length; i += batchSize) {
            const batch = assignmentList.slice(i, i + batchSize);
            const results = await Promise.allSettled(
              batch.map(a => api.getSubmission(courseId, a.id, targetUserId))
            );
            results.forEach((result, idx) => {
              if (result.status === 'fulfilled' && result.value) {
                subMap[batch[idx].id] = result.value;
              }
            });
          }
        }
        setSubmissions(subMap);

        try {
          const [gradeRes, datesRes] = await Promise.all([
            gradingPolicyApi.getCourseGrades(courseId, isObserverView ? { userId: targetUserId } : {}),
            gradingPolicyApi.getAssignmentDates(courseId, isObserverView ? targetUserId : undefined),
          ]);
          setServerGrade((gradeRes?.students || [])[0] || null);
          setGradingPeriods(Array.isArray(gradeRes?.grading_periods) ? gradeRes.grading_periods : []);
          setMyDates(datesRes?.assignments || null);
        } catch {
          setServerGrade(null);
          setMyDates(null);
        }

        // Fetch quiz submissions for student
        if (publishedQuizzes.length > 0) {
          const qsMap = {};
          const qsResults = await Promise.allSettled(
            publishedQuizzes.map(q => api.getQuizSubmissions(courseId, q.id, 1, 50))
          );
          qsResults.forEach((result, idx) => {
            if (result.status === 'fulfilled') {
              const subs = result.value?.data || [];
              const mySub = subs
                .filter(s => s.user_id === targetUserId || String(s.user_id) === String(targetUserId))
                .sort((a, b) => (b.attempt || 0) - (a.attempt || 0))[0];
              if (mySub) qsMap[publishedQuizzes[idx].id] = mySub;
            }
          });
          setQuizSubmissions(qsMap);
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
  };

  useEffect(() => {
    fetchData();
  }, [courseId, user, targetUserId]);

  // G019: assignments not assigned to this student are hidden; G025:
  // a selected grading period shows only work due in it.
  const visibleAssignments = assignments.filter((a) => {
    if (!myDates) return true;
    const d = myDates[String(a.id)];
    if (!d) return false;
    return periodFilter === 'all' || String(d.grading_period_id || '') === String(periodFilter);
  });
  const droppedIds = new Set();
  {
    const src = periodFilter === 'all'
      ? serverGrade
      : (serverGrade?.grading_periods || []).find((p) => String(p.grading_period_id) === String(periodFilter));
    for (const g of src?.assignment_groups || []) for (const id of g.dropped || []) droppedIds.add(id);
  }

  // Group assignments by assignment_group_id
  const getGrouped = () => {
    const assignments = visibleAssignments;
    if (groups.length === 0) {
      return [{ id: null, name: 'Assignments', weight: null, items: assignments }];
    }
    const grouped = groups.map(g => ({
      id: g.id,
      name: g.name,
      weight: g.group_weight,
      items: assignments.filter(a => a.assignment_group_id === g.id),
    }));
    const groupedIds = new Set(groups.map(g => g.id));
    const ungrouped = assignments.filter(a => !groupedIds.has(a.assignment_group_id));
    if (ungrouped.length > 0) {
      grouped.push({ id: null, name: 'Other', weight: null, items: ungrouped });
    }
    return grouped.filter(g => g.items.length > 0);
  };

  // Calculate totals (with weighted grading support)
  const calcTotals = () => {
    const useWeights = course?.apply_assignment_group_weights && groups.length > 0 &&
      groups.some(g => g.group_weight > 0);

    // B205: quizzes are not assignments in Paper, so neither the teacher
    // gradebook nor the server's course grade counts them; the student
    // total must not either (it used to, so the two disagreed).
    // B115: excused / unpublished / not_graded work never counts and a
    // 0-point assignment is extra credit (assignmentGradeWeight).

    if (useWeights) {
      let weightedSum = 0;
      let weightTotal = 0;
      let totalEarned = 0;
      let totalPossible = 0;
      const groupedAssignmentIds = new Set();

      for (const group of groups) {
        if (!group.group_weight || group.group_weight <= 0) continue;
        const groupAssignments = assignments.filter(a => a.assignment_group_id === group.id);
        groupAssignments.forEach(a => groupedAssignmentIds.add(a.id));
        let groupEarned = 0;
        let groupPossible = 0;

        for (const a of groupAssignments) {
          const sub = submissions[a.id];
          const w = assignmentGradeWeight(a, sub);
          if (w.counts && sub && sub.score !== null && sub.score !== undefined) {
            groupEarned += parseFloat(sub.score) || 0;
            groupPossible += w.possible;
          }
        }

        totalEarned += groupEarned;
        totalPossible += groupPossible;

        if (groupPossible > 0) {
          const groupPct = (groupEarned / groupPossible) * 100;
          weightedSum += groupPct * group.group_weight;
          weightTotal += group.group_weight;
        }
      }

      // Include ungrouped assignments as raw points in totals
      const ungrouped = assignments.filter(a => !groupedAssignmentIds.has(a.id));
      for (const a of ungrouped) {
        const sub = submissions[a.id];
        const w = assignmentGradeWeight(a, sub);
        if (w.counts && sub && sub.score !== null && sub.score !== undefined) {
          totalEarned += parseFloat(sub.score) || 0;
          totalPossible += w.possible;
        }
      }

      if (weightTotal === 0) return { earned: 0, possible: 0, pct: null, weighted: true };
      return {
        earned: totalEarned,
        possible: totalPossible,
        pct: (weightedSum / weightTotal).toFixed(1),
        weighted: true,
      };
    }

    // Unweighted: simple point totals
    let earned = 0;
    let possible = 0;
    for (const a of assignments) {
      const sub = submissions[a.id];
      const w = assignmentGradeWeight(a, sub);
      if (w.counts && sub && sub.score !== null && sub.score !== undefined) {
        earned += parseFloat(sub.score) || 0;
        possible += w.possible;
      }
    }
    if (possible === 0) return { earned: 0, possible: 0, pct: null, weighted: false };
    return { earned, possible, pct: ((earned / possible) * 100).toFixed(1), weighted: false };
  };

  // --- What-If mode helpers ---

  const enterWhatIfMode = useCallback(() => {
    // Initialize whatIfScores from actual submission scores
    const initial = {};
    for (const a of assignments) {
      const sub = submissions[a.id];
      if (sub && sub.score !== null && sub.score !== undefined) {
        initial[`a_${a.id}`] = String(parseFloat(sub.score));
      }
    }
    for (const q of quizzes) {
      const qs = quizSubmissions[q.id];
      if (qs && qs.score !== null && qs.score !== undefined) {
        initial[`q_${q.id}`] = String(parseFloat(qs.score));
      }
    }
    setWhatIfScores(initial);
    setWhatIfMode(true);
  }, [assignments, submissions, quizzes, quizSubmissions]);

  const exitWhatIfMode = useCallback(() => {
    setWhatIfMode(false);
    setWhatIfScores({});
  }, []);

  const handleWhatIfChange = useCallback((key, value) => {
    // Allow empty string, digits, and one decimal point
    if (value !== '' && !/^\d*\.?\d*$/.test(value)) return;
    setWhatIfScores(prev => ({ ...prev, [key]: value }));
  }, []);

  // Resolve the effective score for an assignment (What-If aware)
  const getEffectiveAssignmentScore = useCallback((assignmentId) => {
    if (!whatIfMode) {
      const sub = submissions[assignmentId];
      return (sub && sub.score !== null && sub.score !== undefined) ? parseFloat(sub.score) : null;
    }
    const val = whatIfScores[`a_${assignmentId}`];
    if (val === undefined || val === '') return null;
    const num = parseFloat(val);
    return isNaN(num) ? null : num;
  }, [whatIfMode, whatIfScores, submissions]);

  // Calculate totals using effective scores (What-If aware)
  const calcWhatIfTotals = useCallback(() => {
    const useWeights = course?.apply_assignment_group_weights && groups.length > 0 &&
      groups.some(g => g.group_weight > 0);

    // Same inclusion rules as calcTotals (quizzes excluded, B205/B115).

    if (useWeights) {
      let weightedSum = 0;
      let weightTotal = 0;
      let totalEarned = 0;
      let totalPossible = 0;
      const groupedAssignmentIds = new Set();

      for (const group of groups) {
        if (!group.group_weight || group.group_weight <= 0) continue;
        const groupAssignments = assignments.filter(a => a.assignment_group_id === group.id);
        groupAssignments.forEach(a => groupedAssignmentIds.add(a.id));
        let groupEarned = 0;
        let groupPossible = 0;

        for (const a of groupAssignments) {
          const score = getEffectiveAssignmentScore(a.id);
          const w = assignmentGradeWeight(a, { ...submissions[a.id], score });
          if (w.counts && score !== null) {
            groupEarned += score;
            groupPossible += w.possible;
          }
        }

        totalEarned += groupEarned;
        totalPossible += groupPossible;

        if (groupPossible > 0) {
          const groupPct = (groupEarned / groupPossible) * 100;
          weightedSum += groupPct * group.group_weight;
          weightTotal += group.group_weight;
        }
      }

      const ungrouped = assignments.filter(a => !groupedAssignmentIds.has(a.id));
      for (const a of ungrouped) {
        const score = getEffectiveAssignmentScore(a.id);
        const w = assignmentGradeWeight(a, { ...submissions[a.id], score });
        if (w.counts && score !== null) {
          totalEarned += score;
          totalPossible += w.possible;
        }
      }

      if (weightTotal === 0) return { earned: 0, possible: 0, pct: null, weighted: true };
      return {
        earned: totalEarned,
        possible: totalPossible,
        pct: (weightedSum / weightTotal).toFixed(1),
        weighted: true,
      };
    }

    // Unweighted: simple point totals
    let earned = 0;
    let possible = 0;
    for (const a of assignments) {
      const score = getEffectiveAssignmentScore(a.id);
      const w = assignmentGradeWeight(a, { ...submissions[a.id], score });
      if (w.counts && score !== null) {
        earned += score;
        possible += w.possible;
      }
    }
    if (possible === 0) return { earned: 0, possible: 0, pct: null, weighted: false };
    return { earned, possible, pct: ((earned / possible) * 100).toFixed(1), weighted: false };
  }, [course, groups, assignments, submissions, getEffectiveAssignmentScore]);

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-gray-500">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  {t('studentGradesPage.loading')}
</div></Layout>;
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-red-600 mb-3">{error}</p>
  <button onClick={() => { setError(null); setLoading(true); fetchData(); }} className="text-blue-600 hover:text-blue-800 text-sm font-medium">{t('common.tryAgain')}</button>
</div></Layout>;
  }

  const grouped = getGrouped();
  let totals = whatIfMode ? calcWhatIfTotals() : calcTotals();
  let letter = getLetterGrade(totals.pct, gradingScale);
  let overrideNote = null;
  if (!whatIfMode && serverGrade) {
    const set = periodFilter === 'all'
      ? serverGrade
      : (serverGrade.grading_periods || []).find((p) => String(p.grading_period_id) === String(periodFilter));
    if (set && set.current_score !== null && set.current_score !== undefined) {
      totals = {
        earned: Math.round((set.current_points || 0) * 100) / 100,
        possible: Math.round((set.current_possible || 0) * 100) / 100,
        pct: Number(set.current_score).toFixed(1),
      };
      letter = periodFilter === 'all' && serverGrade.current_grade ? serverGrade.current_grade : getLetterGrade(totals.pct, gradingScale);
    } else if (set) {
      totals = { earned: 0, possible: 0, pct: null };
      letter = '-';
    }
    // A final grade override replaces the computed grade (G029).
    const ov = periodFilter === 'all'
      ? serverGrade.override_score
      : serverGrade.period_overrides?.[periodFilter];
    if (ov !== null && ov !== undefined) {
      overrideNote = totals.pct;
      totals = { ...totals, pct: Number(ov).toFixed(1) };
      letter = periodFilter === 'all' && serverGrade.override_grade ? serverGrade.override_grade : getLetterGrade(totals.pct, gradingScale);
    }
  }
  // G024: quizzes graded through an assignment are already listed (and
  // counted) as assignments.
  const assignmentIds = new Set(assignments.map((a) => a.id));
  const standaloneQuizzes = quizzes.filter((q) => !q.assignment_id || !assignmentIds.has(q.assignment_id));

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-blue-600 hover:underline text-sm">
          &larr; {t('course.backToCourse')}
        </Link>
        <div className="flex items-center justify-between mt-2">
          <div>
            <h2 className="text-2xl font-bold text-gray-900">
              {isObserverView
                ? `Grades for ${observeeName || 'Student'}${course ? `: ${course.name}` : ''}`
                : `My Grades${course ? `: ${course.name}` : ''}`}
            </h2>
            {isObserverView && (
              <p className="text-sm text-gray-500 mt-1">Viewing as parent/observer</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            {whatIfMode && (
              <button
                onClick={exitWhatIfMode}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-gray-600 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
                title="Reset to actual grades"
              >
                <RotateCcw className="w-4 h-4" />
                Reset
              </button>
            )}
            <button
              onClick={whatIfMode ? exitWhatIfMode : enterWhatIfMode}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-lg transition-colors ${
                whatIfMode
                  ? 'bg-purple-600 text-white hover:bg-purple-700'
                  : 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50'
              }`}
              title={whatIfMode ? 'Exit What-If mode' : 'Enter What-If mode'}
            >
              <Calculator className="w-4 h-4" />
              {whatIfMode ? 'Exit What-If' : 'What-If Grades'}
            </button>
          </div>
        </div>
      </div>

      {/* What-If mode banner */}
      {whatIfMode && (
        <div className="bg-purple-50 border border-purple-200 rounded-lg px-4 py-3 mb-6 flex items-center gap-3">
          <Calculator className="w-5 h-5 text-purple-600 flex-shrink-0" />
          <div>
            <p className="text-sm font-medium text-purple-800">What-If mode: these are hypothetical grades, not your actual grades.</p>
            <p className="text-xs text-purple-600 mt-0.5">Edit any score below to see how it affects your overall grade. Click &quot;Reset&quot; to restore actual grades.</p>
          </div>
        </div>
      )}

      {/* Overall Grade Summary */}
      <div className="bg-white rounded-lg shadow p-6 mb-6">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm text-gray-500 uppercase tracking-wide font-medium">
              {whatIfMode ? <span className="text-purple-600">Hypothetical Grade</span> : 'Current Grade'}
            </div>
            <div className="mt-1 flex items-baseline gap-3">
              <span className={`text-4xl font-bold ${gradeColor(letter)}`}>{letter}</span>
              {totals.pct !== null && (
                <span className="text-2xl text-gray-600">{totals.pct}%</span>
              )}
            </div>
            <div className="text-sm text-gray-500 mt-1">
              {totals.earned} / {totals.possible} points earned
            </div>
            {overrideNote !== null && (
              <div className="text-xs text-gray-500 mt-1">
                Final grade set by your instructor (calculated: {overrideNote ?? '—'}%)
              </div>
            )}
          </div>
          {gradingPeriods.length > 0 && !whatIfMode && (
            <select
              value={periodFilter}
              onChange={(e) => setPeriodFilter(e.target.value)}
              aria-label="Grading period"
              className="border border-gray-300 rounded-md px-2 py-1.5 text-sm"
            >
              <option value="all">All grading periods</option>
              {gradingPeriods.map((p) => (
                <option key={p.id} value={p.id}>{p.title}</option>
              ))}
            </select>
          )}
          <div className="text-right text-sm text-gray-500">
            {assignments.length} assignment{assignments.length !== 1 ? 's' : ''}
            {quizzes.length > 0 && (<>, {quizzes.length} quiz{quizzes.length !== 1 ? 'zes' : ''}</>)}
          </div>
        </div>
      </div>

      {/* Assignment Groups */}
      {grouped.map(group => (
        <div key={group.id || 'other'} className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-lg font-semibold text-gray-800">{group.name}</h3>
            {group.weight != null && (
              <span className="text-sm text-gray-500">{group.weight}% of total</span>
            )}
          </div>
          <div className="bg-white rounded-lg shadow overflow-hidden">
            <div className="overflow-x-auto">
            <table className="min-w-full" aria-label="Grades">
              <thead>
                <tr className="bg-gray-50 border-b">
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Assignment</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Due</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Score</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {group.items.map(a => {
                  const sub = submissions[a.id];
                  // B109: the API withholds the score until the grade is
                  // posted and serializes posted_at, so posted_at decides.
                  const scoreVisible = !sub || !!sub.posted_at;
                  const score = scoreVisible ? sub?.score : null;
                  const pts = a.points_possible ?? 0;
                  const pct = (score !== null && score !== undefined && pts > 0)
                    ? ((parseFloat(score) / pts) * 100).toFixed(0)
                    : null;

                  return (
                    <tr key={a.id} className="hover:bg-gray-50">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Link
                            to={`/courses/${courseId}/assignments/${a.id}`}
                            className="text-blue-600 hover:underline text-sm font-medium"
                          >
                            {a.name}
                          </Link>
                          {droppedIds.has(a.id) && (
                            <span className="inline-flex items-center px-1.5 py-0.5 text-xs rounded bg-gray-100 text-gray-600" title="This score does not count toward your grade">
                              Dropped
                            </span>
                          )}
                          {isRecentlyGraded(sub) && (
                            <span className="inline-flex items-center px-1.5 py-0.5 text-xs font-semibold rounded bg-orange-100 text-orange-700 animate-pulse">
                              New
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">
                        {formatDate(myDates?.[String(a.id)]?.due_at ?? a.due_at)}
                      </td>
                      <td className="px-4 py-3 text-center">
                        {statusBadge(sub)}
                      </td>
                      <td className="px-4 py-3 text-right whitespace-nowrap">
                        {whatIfMode ? (() => {
                          const wiKey = `a_${a.id}`;
                          const wiVal = whatIfScores[wiKey] ?? '';
                          const wiScore = wiVal !== '' ? parseFloat(wiVal) : null;
                          const wiPct = (wiScore !== null && !isNaN(wiScore) && pts > 0)
                            ? ((wiScore / pts) * 100).toFixed(0)
                            : null;
                          return (
                            <div className="flex items-center justify-end gap-1">
                              <input
                                type="text"
                                inputMode="decimal"
                                value={wiVal}
                                onChange={(e) => handleWhatIfChange(wiKey, e.target.value)}
                                className="w-16 px-2 py-1 text-sm text-right font-semibold border border-purple-300 rounded focus:outline-none focus:ring-2 focus:ring-purple-400 bg-purple-50"
                                placeholder="-"
                                aria-label={`What-if score for ${a.name}`}
                              />
                              <span className="text-sm text-gray-500">/{pts}</span>
                              {wiPct !== null && (
                                <span className={`ml-1 text-xs font-medium ${gradeColor(getLetterGrade(wiPct, gradingScale))}`}>
                                  {wiPct}%
                                </span>
                              )}
                            </div>
                          );
                        })() : (
                          sub?.excused && scoreVisible ? (
                            <span className="text-xs text-gray-500 italic">Excused</span>
                          ) : !scoreVisible && a.post_policy === 'manual' && sub?.submitted_at ? (
                            <span className="text-xs text-gray-400 italic">Not posted</span>
                          ) : score !== null && score !== undefined && ['letter_grade', 'gpa_scale', 'pass_fail', 'percent'].includes(a.grading_type) && sub?.grade ? (
                            <span className="text-sm font-semibold text-gray-900">{sub.grade}</span>
                          ) : score !== null && score !== undefined ? (
                            <div>
                              <span className="text-sm font-semibold text-gray-900">
                                {parseFloat(score)}/{pts}
                              </span>
                              {pct !== null && (
                                <span className={`ml-2 text-xs font-medium ${gradeColor(getLetterGrade(pct, gradingScale))}`}>
                                  {pct}%
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-sm text-gray-400">-/{pts}</span>
                          )
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            </div>
          </div>
        </div>
      ))}

      {/* Quizzes */}
      {standaloneQuizzes.length > 0 && (
        <div className="mb-6">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-lg font-semibold text-gray-800">Quizzes</h3>
            <span className="text-xs text-gray-500">Not included in the course total</span>
          </div>
          <div className="bg-white rounded-lg shadow overflow-hidden">
            <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="bg-gray-50 border-b">
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Quiz</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Due</th>
                  <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Score</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {standaloneQuizzes.map(q => {
                  const qs = quizSubmissions[q.id];
                  const score = qs?.score;
                  const pts = q.points_possible ?? 0;
                  const pct = (score !== null && score !== undefined && pts > 0)
                    ? ((parseFloat(score) / pts) * 100).toFixed(0)
                    : null;

                  const quizStatus = () => {
                    if (!qs) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-500">Not attempted</span>;
                    if (qs.workflow_state === 'complete') return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-green-100 text-green-700">Completed</span>;
                    if (qs.workflow_state === 'pending_review') return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-blue-100 text-blue-700">Pending review</span>;
                    if (qs.workflow_state === 'untaken' && qs.started_at) return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-yellow-100 text-yellow-700">In progress</span>;
                    return <span className="inline-block px-2 py-0.5 text-xs rounded-full bg-gray-100 text-gray-500">Not attempted</span>;
                  };

                  const quizLink = qs && qs.workflow_state === 'complete'
                    ? `/courses/${courseId}/quizzes/${q.id}/submissions/${qs.id}/review`
                    : `/courses/${courseId}/quizzes/${q.id}`;

                  return (
                    <tr key={q.id} className="hover:bg-gray-50">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Link
                            to={quizLink}
                            className="text-blue-600 hover:underline text-sm font-medium"
                          >
                            {q.title}
                          </Link>
                          {qs && qs.workflow_state === 'complete' && (
                            <Link
                              to={`/courses/${courseId}/quizzes/${q.id}/submissions/${qs.id}/review`}
                              className="text-xs text-purple-600 hover:underline"
                            >
                              Review
                            </Link>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">
                        {formatDate(q.due_at)}
                      </td>
                      <td className="px-4 py-3 text-center">
                        {quizStatus()}
                      </td>
                      <td className="px-4 py-3 text-right whitespace-nowrap">
                        {whatIfMode ? (() => {
                          const wiKey = `q_${q.id}`;
                          const wiVal = whatIfScores[wiKey] ?? '';
                          const wiScore = wiVal !== '' ? parseFloat(wiVal) : null;
                          const wiPct = (wiScore !== null && !isNaN(wiScore) && pts > 0)
                            ? ((wiScore / pts) * 100).toFixed(0)
                            : null;
                          return (
                            <div className="flex items-center justify-end gap-1">
                              <input
                                type="text"
                                inputMode="decimal"
                                value={wiVal}
                                onChange={(e) => handleWhatIfChange(wiKey, e.target.value)}
                                className="w-16 px-2 py-1 text-sm text-right font-semibold border border-purple-300 rounded focus:outline-none focus:ring-2 focus:ring-purple-400 bg-purple-50"
                                placeholder="-"
                                aria-label={`What-if score for ${q.title}`}
                              />
                              <span className="text-sm text-gray-500">/{pts}</span>
                              {wiPct !== null && (
                                <span className={`ml-1 text-xs font-medium ${gradeColor(getLetterGrade(wiPct, gradingScale))}`}>
                                  {wiPct}%
                                </span>
                              )}
                            </div>
                          );
                        })() : (
                          score !== null && score !== undefined ? (
                            <div>
                              <span className="text-sm font-semibold text-gray-900">
                                {parseFloat(score)}/{pts}
                              </span>
                              {pct !== null && (
                                <span className={`ml-2 text-xs font-medium ${gradeColor(getLetterGrade(pct, gradingScale))}`}>
                                  {pct}%
                                </span>
                              )}
                            </div>
                          ) : (
                            <span className="text-sm text-gray-400">-/{pts}</span>
                          )
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            </div>
          </div>
        </div>
      )}

      {assignments.length === 0 && quizzes.length === 0 && (
        <div className="bg-white rounded-lg shadow p-8 text-center text-gray-500">
          No assignments or quizzes in this course yet.
        </div>
      )}
    </Layout>
  );
};

export default StudentGradesPage;
