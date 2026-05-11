import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Calendar, Award, Clock, User, CheckCircle, AlertCircle, MinusCircle, Zap, Eye, EyeOff, Users, Pencil, X, Save, MessageCircle, ArrowRight, Send, Star, Target } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import useUnsavedChanges from '../hooks/useUnsavedChanges';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import SubmissionForm from '../components/SubmissionForm';
import SubmissionComments from '../components/SubmissionComments';
import RichContentViewer, { sanitizeHTML } from '../components/RichContentViewer';
import RichContentEditor from '../components/RichContentEditor';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';

function toLocalDatetimeValue(date) {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  const h = String(date.getHours()).padStart(2, '0');
  const min = String(date.getMinutes()).padStart(2, '0');
  return `${y}-${m}-${d}T${h}:${min}`;
}

const STATUS_CONFIG = {
  submitted: { label: 'Submitted', color: 'bg-brand-100 text-brand-800', icon: CheckCircle },
  graded: { label: 'Graded', color: 'bg-accent-success/20 text-accent-success', icon: Award },
  pending_review: { label: 'Pending Review', color: 'bg-accent-warning/20 text-accent-warning', icon: Clock },
  unsubmitted: { label: 'Not Submitted', color: 'bg-surface-2 text-text-secondary', icon: MinusCircle },
};

const AssignmentPage = () => {
  const { courseId, assignmentId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [assignment, setAssignment] = useState(null);
  const [submissions, setSubmissions] = useState([]);
  const [mySubmission, setMySubmission] = useState(null);
  const [enrollments, setEnrollments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [overrides, setOverrides] = useState([]);
  const [sections, setSections] = useState([]);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [assignmentGroups, setAssignmentGroups] = useState([]);
  const [editForm, setEditForm] = useState({});
  const [isDirty, setIsDirty] = useState(false);
  const [groupCategories, setGroupCategories] = useState([]);
  const [peerReviews, setPeerReviews] = useState([]);
  const [myPeerReviews, setMyPeerReviews] = useState([]);
  const [peerReviewCount, setPeerReviewCount] = useState(1);
  const [assigningReviews, setAssigningReviews] = useState(false);
  const [reviewForms, setReviewForms] = useState({});
  const [submittingReview, setSubmittingReview] = useState(null);
  const [expandedReview, setExpandedReview] = useState(null);
  const [alignments, setAlignments] = useState([]);
  const [courseOutcomes, setCourseOutcomes] = useState([]);
  const [selectedOutcomeId, setSelectedOutcomeId] = useState('');
  const [addingAlignment, setAddingAlignment] = useState(false);

  useUnsavedChanges(isDirty);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  useEffect(() => {
    if (isTeacher === null) return;
    const fetchData = async () => {
      try {
        const [assignmentData, enrollmentResult] = await Promise.all([
          api.getAssignment(courseId, assignmentId),
          api.getEnrollments(courseId, 1, 100),
        ]);

        setAssignment(assignmentData);
        setEnrollments(enrollmentResult.data || []);

        if (isTeacher) {
          const [overrideData, sectionData, groupsData, groupCatsData] = await Promise.allSettled([
            api.getAssignmentOverrides(courseId, assignmentId),
            api.getSections(courseId),
            api.getAssignmentGroups(courseId),
            api.getGroupCategories(courseId),
          ]);
          if (overrideData.status === 'fulfilled') {
            setOverrides(Array.isArray(overrideData.value) ? overrideData.value : []);
          }
          if (sectionData.status === 'fulfilled') {
            setSections(sectionData.value?.data || []);
          }
          if (groupsData.status === 'fulfilled') {
            setAssignmentGroups(groupsData.value?.data || []);
          }
          if (groupCatsData.status === 'fulfilled') {
            setGroupCategories(groupCatsData.value?.data || []);
          }
          try {
            const submissionResult = await api.getSubmissions(courseId, assignmentId, 1, 100);
            setSubmissions(submissionResult.data || []);
          } catch {
            setSubmissions([]);
          }
          // Fetch peer reviews for teachers
          try {
            const reviews = await api.listPeerReviews(courseId, assignmentId);
            setPeerReviews(reviews || []);
          } catch {
            setPeerReviews([]);
          }
        } else {
          try {
            const sub = await api.getSubmission(courseId, assignmentId, user?.id);
            setMySubmission(sub);
          } catch {
            setMySubmission(null);
          }
          // Fetch my peer reviews for students
          try {
            const myReviews = await api.listMyPeerReviews(courseId, assignmentId);
            setMyPeerReviews(myReviews || []);
          } catch {
            setMyPeerReviews([]);
          }
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [courseId, assignmentId, user?.id, isTeacher]);

  // Fetch outcome alignments and available outcomes for teachers
  useEffect(() => {
    if (!isTeacher || !courseId || !assignmentId) return;
    const fetchAlignments = async () => {
      try {
        const [allAlignments, outcomes] = await Promise.allSettled([
          api.getOutcomeAlignments(courseId),
          api.getCourseOutcomes(courseId),
        ]);
        if (allAlignments.status === 'fulfilled') {
          const filtered = (allAlignments.value || []).filter(
            (a) => String(a.assignment_id) === String(assignmentId)
          );
          setAlignments(filtered);
        }
        if (outcomes.status === 'fulfilled') {
          setCourseOutcomes(outcomes.value || []);
        }
      } catch {
        // silently fail - alignments are non-critical
      }
    };
    fetchAlignments();
  }, [isTeacher, courseId, assignmentId]);

  const handleAddAlignment = async () => {
    if (!selectedOutcomeId) return;
    setAddingAlignment(true);
    try {
      const result = await api.createOutcomeAlignment(courseId, {
        learning_outcome_id: Number(selectedOutcomeId),
        assignment_id: Number(assignmentId),
      });
      const outcome = courseOutcomes.find((o) => String(o.id) === String(selectedOutcomeId));
      setAlignments((prev) => [...prev, { ...result, outcome_title: outcome?.title || outcome?.display_name || `Outcome ${selectedOutcomeId}` }]);
      setSelectedOutcomeId('');
    } catch (err) {
      setError(err.message);
    } finally {
      setAddingAlignment(false);
    }
  };

  const handleRemoveAlignment = async (alignmentId) => {
    try {
      await api.deleteOutcomeAlignment(courseId, alignmentId);
      setAlignments((prev) => prev.filter((a) => a.id !== alignmentId));
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSubmit = async (submissionData) => {
    let result;
    if (submissionData.submission_type === 'online_upload' && submissionData.file) {
      // Upload file first, then create submission with file ID
      const uploadResult = await api.uploadCourseFile(courseId, submissionData.file);
      const fileData = uploadResult.data || uploadResult;
      result = await api.createSubmission(courseId, assignmentId, {
        submission_type: 'online_upload',
        file_ids: [fileData.id],
      });
    } else {
      result = await api.createSubmission(courseId, assignmentId, submissionData);
    }
    setMySubmission(result);
    return result;
  };

  const startEditing = () => {
    const types = Array.isArray(assignment.submission_types)
      ? assignment.submission_types
      : String(assignment.submission_types || 'online_text_entry').split(',').map((t) => t.trim());
    setEditForm({
      name: assignment.name || '',
      description: assignment.description || '',
      points_possible: assignment.points_possible ?? 0,
      due_at: assignment.due_at ? toLocalDatetimeValue(new Date(assignment.due_at)) : '',
      submission_types: types,
      assignment_group_id: assignment.assignment_group_id || '',
      anonymous_grading: assignment.anonymous_grading || false,
      post_policy: assignment.post_policy || 'automatic',
      is_group_assignment: assignment.is_group_assignment || false,
      group_category_id: assignment.group_category_id || '',
    });
    setEditing(true);
  };

  const doSaveEdit = async () => {
    setSaving(true);
    try {
      const payload = {
        name: editForm.name,
        description: editForm.description,
        points_possible: Number(editForm.points_possible),
        due_at: editForm.due_at ? new Date(editForm.due_at).toISOString() : null,
        submission_types: editForm.submission_types,
        anonymous_grading: editForm.anonymous_grading,
        post_policy: editForm.post_policy,
      };
      if (editForm.assignment_group_id) {
        payload.assignment_group_id = Number(editForm.assignment_group_id);
      }
      if (editForm.is_group_assignment && editForm.group_category_id) {
        payload.group_category_id = Number(editForm.group_category_id);
      } else {
        payload.group_category_id = null;
      }
      await api.updateAssignment(courseId, assignmentId, payload);
      setAssignment((prev) => ({
        ...prev,
        ...payload,
        is_group_assignment: !!(editForm.is_group_assignment && editForm.group_category_id),
      }));
      setIsDirty(false);
      setEditing(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleSaveEdit = (e) => {
    e.preventDefault();
    checkAndSave(editForm.description, doSaveEdit);
  };

  const toggleSubmissionType = (type) => {
    setEditForm((prev) => {
      const types = prev.submission_types.includes(type)
        ? prev.submission_types.filter((t) => t !== type)
        : [...prev.submission_types, type];
      return { ...prev, submission_types: types.length > 0 ? types : [type] };
    });
    setIsDirty(true);
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return 'No due date';
    return new Date(dateStr).toLocaleString(undefined, {
      month: 'numeric', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    });
  };

  const getStatusConfig = (submission) => {
    const state = submission?.workflow_state || 'unsubmitted';
    return STATUS_CONFIG[state] || STATUS_CONFIG.unsubmitted;
  };

  const getStudentName = (submission) => {
    if (submission.user?.name) return submission.user.name;
    if (submission.user?.display_name) return submission.user.display_name;
    // Try to find in enrollments
    const enrollment = enrollments.find(
      (e) => e.user_id === submission.user_id || e.user?.id === submission.user_id
    );
    return enrollment?.user?.name || `User ${submission.user_id}`;
  };

  const handleAssignPeerReviews = async () => {
    setAssigningReviews(true);
    try {
      await api.assignPeerReviews(courseId, assignmentId, peerReviewCount);
      const reviews = await api.listPeerReviews(courseId, assignmentId);
      setPeerReviews(reviews || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setAssigningReviews(false);
    }
  };

  const handleSubmitPeerReview = async (reviewId) => {
    const form = reviewForms[reviewId];
    if (!form?.score && form?.score !== 0) return;
    setSubmittingReview(reviewId);
    try {
      await api.submitPeerReview(reviewId, Number(form.score), form.comments || '');
      // Refresh my peer reviews
      const myReviews = await api.listMyPeerReviews(courseId, assignmentId);
      setMyPeerReviews(myReviews || []);
      setReviewForms((prev) => {
        const next = { ...prev };
        delete next[reviewId];
        return next;
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmittingReview(null);
    }
  };

  const updateReviewForm = (reviewId, field, value) => {
    setReviewForms((prev) => ({
      ...prev,
      [reviewId]: { ...prev[reviewId], [field]: value },
    }));
  };

  const getPeerReviewStatusConfig = (status) => {
    switch (status) {
      case 'completed':
        return { label: 'Completed', color: 'bg-accent-success/20 text-accent-success', icon: CheckCircle };
      case 'submitted':
        return { label: 'Submitted', color: 'bg-accent-success/20 text-accent-success', icon: CheckCircle };
      default:
        return { label: 'Pending', color: 'bg-accent-warning/20 text-accent-warning', icon: Clock };
    }
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading assignment...
</div></Layout>;
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-accent-danger mb-3">{error}</p>
  <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
</div></Layout>;
  }
  if (!assignment) {
    return <Layout><div className="text-center py-12">Assignment not found</div></Layout>;
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          ← Back to Course
        </Link>
      </div>

      {/* Assignment Header / Edit Form */}
      {editing ? (
        <form onSubmit={handleSaveEdit} className="bg-surface-0 rounded-lg shadow p-6 mb-6 space-y-4">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-lg font-semibold text-text-primary">Edit Assignment</h3>
            <button type="button" onClick={() => { setEditing(false); setIsDirty(false); }} className="text-text-disabled hover:text-text-secondary">
              <X className="w-5 h-5" />
            </button>
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input
              type="text"
              required
              value={editForm.name}
              onChange={(e) => { setEditForm({ ...editForm, name: e.target.value }); setIsDirty(true); }}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
            <RichContentEditor
              value={editForm.description}
              onChange={(html) => { setEditForm((prev) => ({ ...prev, description: html })); setIsDirty(true); }}
              placeholder="Assignment instructions..."
              minHeight="160px"
              courseId={courseId}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Points</label>
              <input
                type="number"
                min="0"
                value={editForm.points_possible}
                onChange={(e) => { setEditForm({ ...editForm, points_possible: e.target.value }); setIsDirty(true); }}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Due Date</label>
              <input
                type="datetime-local"
                value={editForm.due_at}
                onChange={(e) => { setEditForm({ ...editForm, due_at: e.target.value }); setIsDirty(true); }}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Submission Types</label>
            <div className="flex flex-wrap gap-3">
              {[
                { value: 'online_text_entry', label: 'Text Entry' },
                { value: 'online_upload', label: 'File Upload' },
                { value: 'online_url', label: 'URL' },
                { value: 'on_paper', label: 'On Paper' },
                { value: 'none', label: 'No Submission' },
              ].map((opt) => (
                <label key={opt.value} className="flex items-center gap-1.5 text-sm text-text-secondary">
                  <input
                    type="checkbox"
                    checked={editForm.submission_types?.includes(opt.value)}
                    onChange={() => toggleSubmissionType(opt.value)}
                    className="rounded border-border-strong"
                  />
                  {opt.label}
                </label>
              ))}
            </div>
          </div>
          {assignmentGroups.length > 0 && (
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Assignment Group</label>
              <select
                value={editForm.assignment_group_id}
                onChange={(e) => { setEditForm({ ...editForm, assignment_group_id: e.target.value }); setIsDirty(true); }}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              >
                <option value="">None</option>
                {assignmentGroups.map((g) => (
                  <option key={g.id} value={g.id}>{g.name}</option>
                ))}
              </select>
            </div>
          )}
          <div className="grid grid-cols-2 gap-4">
            <label className="flex items-center gap-2 text-sm text-text-secondary">
              <input
                type="checkbox"
                checked={editForm.anonymous_grading}
                onChange={(e) => { setEditForm({ ...editForm, anonymous_grading: e.target.checked }); setIsDirty(true); }}
                className="rounded border-border-strong"
              />
              Anonymous Grading
            </label>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Grade Posting Policy</label>
              <select
                value={editForm.post_policy}
                onChange={(e) => { setEditForm({ ...editForm, post_policy: e.target.value }); setIsDirty(true); }}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              >
                <option value="automatic">Automatic</option>
                <option value="manual">Manual</option>
              </select>
            </div>
          </div>
          <div>
            <label className="flex items-center gap-2 text-sm text-text-secondary">
              <input
                type="checkbox"
                checked={editForm.is_group_assignment || false}
                onChange={(e) => {
                  const checked = e.target.checked;
                  setEditForm({ ...editForm, is_group_assignment: checked, group_category_id: checked ? editForm.group_category_id : '' });
                  setIsDirty(true);
                }}
                className="rounded border-border-strong"
              />
              <Users className="w-4 h-4 text-text-tertiary" />
              Group Assignment
            </label>
            {editForm.is_group_assignment && (
              <div className="mt-2 ml-6">
                <label className="block text-sm font-medium text-text-secondary mb-1">Group Category</label>
                {groupCategories.length === 0 ? (
                  <p className="text-sm text-text-disabled">No group categories found for this course. Create one on the Groups page first.</p>
                ) : (
                  <select
                    value={editForm.group_category_id}
                    onChange={(e) => { setEditForm({ ...editForm, group_category_id: e.target.value }); setIsDirty(true); }}
                    className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
                  >
                    <option value="">Select a group category...</option>
                    {groupCategories.map((gc) => (
                      <option key={gc.id} value={gc.id}>{gc.name}</option>
                    ))}
                  </select>
                )}
              </div>
            )}
          </div>
          <div className="flex justify-end space-x-3 pt-2">
            <button type="button" onClick={() => { setEditing(false); setIsDirty(false); }} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="inline-flex items-center gap-1.5 px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </form>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <div className="flex items-start justify-between">
            <h2 className="text-2xl font-bold text-text-primary mb-2">{assignment.name}</h2>
            {isTeacher && (
              <div className="flex items-center gap-2">
                <button
                  onClick={startEditing}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors bg-surface-2 text-text-secondary hover:bg-border-default"
                >
                  <Pencil className="w-4 h-4" />
                  Edit
                </button>
                <button
                  onClick={async () => {
                    const newPublished = !assignment.published;
                    try {
                      await api.updateAssignment(courseId, assignmentId, { published: newPublished });
                      setAssignment((prev) => ({ ...prev, published: newPublished }));
                    } catch (err) {
                      setError(err.message);
                    }
                  }}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                    assignment.published !== false
                      ? 'bg-accent-success/10 text-accent-success hover:bg-accent-success/20'
                      : 'bg-surface-2 text-text-secondary hover:bg-border-default'
                  }`}
                >
                  {assignment.published !== false ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
                  {assignment.published !== false ? 'Published' : 'Unpublished'}
                </button>
              </div>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-4 text-sm text-text-tertiary mb-4">
            <div className="flex items-center space-x-1">
              <Calendar className="w-4 h-4" />
              <span>Due: {formatDate(assignment.due_at)}</span>
            </div>
            <div className="flex items-center space-x-1">
              <Award className="w-4 h-4" />
              <span>{assignment.points_possible ?? 0} points</span>
            </div>
            {assignment.is_group_assignment && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-full bg-indigo-100 text-indigo-700">
                <Users className="w-3 h-3" />
                Group Assignment
              </span>
            )}
            {assignment.anonymous_grading && (
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-purple-100 text-purple-700">
                Anonymous
              </span>
            )}
            {assignment.post_policy === 'manual' && (
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-orange-100 text-orange-700">
                Manual Posting
              </span>
            )}
            {assignment.submission_types && (
              <div className="text-text-disabled">
                Type: {(() => {
                  const typeLabels = {
                    online_text_entry: 'Text Entry',
                    online_upload: 'File Upload',
                    online_url: 'URL',
                    media_recording: 'Media Recording',
                    on_paper: 'On Paper',
                    none: 'No Submission',
                  };
                  const types = Array.isArray(assignment.submission_types)
                    ? assignment.submission_types
                    : String(assignment.submission_types).split(',');
                  return types.map((t) => typeLabels[t.trim()] || t.trim()).join(', ');
                })()}
              </div>
            )}
          </div>
          {assignment.description && (
            <RichContentViewer content={assignment.description} />
          )}
        </div>
      )}

      {/* Section Date Overrides (teacher only) */}
      {isTeacher && overrides.length > 0 && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-6">
          <h3 className="text-sm font-semibold text-text-secondary mb-3 flex items-center space-x-2">
            <Users className="w-4 h-4" />
            <span>Section Due Dates</span>
          </h3>
          <div className="divide-y divide-gray-100">
            {overrides.map((override) => {
              const section = sections.find((s) => s.id === override.course_section_id);
              const label = override.title || section?.name || `Section ${override.course_section_id || ''}`;
              return (
                <div key={override.id} className="py-2 flex items-center justify-between text-sm">
                  <span className="text-text-secondary font-medium">{label}</span>
                  <div className="flex items-center gap-4 text-text-tertiary">
                    {override.due_at && (
                      <span>Due: {formatDate(override.due_at)}</span>
                    )}
                    {override.unlock_at && (
                      <span className="text-xs">Unlocks: {formatDate(override.unlock_at)}</span>
                    )}
                    {override.lock_at && (
                      <span className="text-xs">Locks: {formatDate(override.lock_at)}</span>
                    )}
                    {!override.due_at && !override.unlock_at && !override.lock_at && (
                      <span className="text-xs text-text-disabled">No dates set</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
          <Link
            to={`/courses/${courseId}/assignments/${assignmentId}/overrides`}
            className="text-brand-600 hover:underline text-xs mt-2 inline-block"
          >
            Manage section dates
          </Link>
        </div>
      )}

      {/* Aligned Outcomes (teacher only) */}
      {isTeacher && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-6">
          <h3 className="text-sm font-semibold text-text-secondary mb-3 flex items-center space-x-2">
            <Target className="w-4 h-4 text-accent-success" />
            <span>Aligned Outcomes</span>
          </h3>

          {/* Current alignments as chips */}
          {alignments.length > 0 ? (
            <div className="flex flex-wrap gap-2 mb-3">
              {alignments.map((a) => {
                const outcome = courseOutcomes.find((o) => String(o.id) === String(a.learning_outcome_id));
                const label = a.outcome_title || outcome?.title || outcome?.display_name || `Outcome ${a.learning_outcome_id}`;
                return (
                  <span
                    key={a.id}
                    className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-sm font-medium bg-accent-success/10 text-emerald-700 border border-emerald-200"
                  >
                    <Target className="w-3 h-3" />
                    {label}
                    <button
                      onClick={() => handleRemoveAlignment(a.id)}
                      className="ml-0.5 text-emerald-400 hover:text-accent-danger transition-colors"
                      title="Remove alignment"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </span>
                );
              })}
            </div>
          ) : (
            <p className="text-sm text-text-disabled mb-3">No outcomes aligned to this assignment yet.</p>
          )}

          {/* Add alignment dropdown */}
          {courseOutcomes.length > 0 && (
            <div className="flex items-center gap-2">
              <select
                value={selectedOutcomeId}
                onChange={(e) => setSelectedOutcomeId(e.target.value)}
                className="flex-1 border border-border-strong rounded-md px-3 py-1.5 text-sm text-text-secondary"
              >
                <option value="">Select an outcome to align...</option>
                {courseOutcomes
                  .filter((o) => !alignments.some((a) => String(a.learning_outcome_id) === String(o.id)))
                  .map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.title || o.display_name}{o.group_title ? ` (${o.group_title})` : ''}
                    </option>
                  ))}
              </select>
              <button
                onClick={handleAddAlignment}
                disabled={!selectedOutcomeId || addingAlignment}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600 text-white rounded-md hover:bg-emerald-700 text-sm font-medium disabled:opacity-50 whitespace-nowrap"
              >
                <Target className="w-3.5 h-3.5" />
                {addingAlignment ? 'Adding...' : 'Add Alignment'}
              </button>
            </div>
          )}
          {courseOutcomes.length === 0 && (
            <p className="text-xs text-text-disabled">
              No outcomes found for this course. <Link to={`/courses/${courseId}/outcomes`} className="text-brand-600 hover:underline">Create outcomes</Link> first.
            </p>
          )}
        </div>
      )}

      {/* Student View */}
      {!isTeacher && (
        <div className="space-y-6">
          {/* Submission Status */}
          {mySubmission && (
            <div className="bg-surface-0 rounded-lg shadow p-4">
              <div className="flex items-center space-x-2">
                {(() => {
                  const config = getStatusConfig(mySubmission);
                  const StatusIcon = config.icon;
                  return (
                    <>
                      <StatusIcon className="w-5 h-5" />
                      <span className={`text-sm font-medium px-2 py-1 rounded ${config.color}`}>
                        {config.label}
                      </span>
                      {mySubmission.grade !== null && mySubmission.grade !== undefined && (
                        <span className="ml-auto text-lg font-semibold text-brand-600">
                          {mySubmission.score ?? mySubmission.grade} / {assignment.points_possible}
                        </span>
                      )}
                    </>
                  );
                })()}
              </div>
            </div>
          )}

          {/* Submission Form */}
          {(() => {
            const types = Array.isArray(assignment.submission_types)
              ? assignment.submission_types
              : String(assignment.submission_types || '').split(',').map(t => t.trim());
            const isOnPaperOrNone = types.every(t => t === 'on_paper' || t === 'none' || t === '');
            if (isOnPaperOrNone) {
              return (
                <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">
                  {types.includes('on_paper')
                    ? 'This assignment is submitted on paper. No online submission is required.'
                    : 'No submission is required for this assignment.'}
                </div>
              );
            }
            return (
              <div className="bg-surface-0 rounded-lg shadow p-6">
                <SubmissionForm
                  courseId={courseId}
                  assignmentId={assignmentId}
                  existingSubmission={mySubmission}
                  onSubmit={handleSubmit}
                  submissionTypes={assignment.submission_types}
                />
              </div>
            );
          })()}

          {/* Comments */}
          {mySubmission && mySubmission.workflow_state !== 'unsubmitted' && (
            <div className="bg-surface-0 rounded-lg shadow p-6">
              <SubmissionComments
                courseId={courseId}
                assignmentId={assignmentId}
                userId={user?.id}
                isTeacher={false}
              />
            </div>
          )}

          {/* My Peer Reviews (Student) */}
          {myPeerReviews.length > 0 && (
            <div className="bg-surface-0 rounded-lg shadow">
              <div className="p-4 border-b">
                <h3 className="font-semibold flex items-center gap-2">
                  <MessageCircle className="w-5 h-5 text-purple-600" />
                  My Peer Reviews ({myPeerReviews.length})
                </h3>
              </div>
              <div className="divide-y">
                {myPeerReviews.map((review) => {
                  const statusConfig = getPeerReviewStatusConfig(review.workflow_state || review.status);
                  const ReviewStatusIcon = statusConfig.icon;
                  const isCompleted = review.workflow_state === 'completed' || review.status === 'completed' || review.workflow_state === 'submitted' || review.status === 'submitted';
                  const isExpanded = expandedReview === review.id;
                  const form = reviewForms[review.id] || {};

                  return (
                    <div key={review.id} className="p-4">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-3">
                          <User className="w-5 h-5 text-text-disabled" />
                          <span className="text-sm font-medium text-text-primary">
                            Review for: {review.reviewee_name || review.submission?.user_name || `Student ${review.reviewee_id || review.user_id || ''}`}
                          </span>
                          <span className={`text-xs font-medium px-2 py-0.5 rounded ${statusConfig.color}`}>
                            {statusConfig.label}
                          </span>
                        </div>
                        {!isCompleted && (
                          <button
                            onClick={() => setExpandedReview(isExpanded ? null : review.id)}
                            className="text-sm text-brand-600 hover:text-brand-800 font-medium"
                          >
                            {isExpanded ? 'Collapse' : 'Write Review'}
                          </button>
                        )}
                      </div>

                      {/* Show submission content to review */}
                      {(review.submission?.body || review.submission_body) && (
                        <div
                          className="text-sm text-text-secondary bg-surface-1 rounded p-3 mb-3 prose max-w-none"
                          dangerouslySetInnerHTML={{ __html: sanitizeHTML(review.submission?.body || review.submission_body) }}
                        />
                      )}

                      {/* Show completed review details */}
                      {isCompleted && (
                        <div className="bg-accent-success/10 rounded p-3 mt-2">
                          <div className="flex items-center gap-2 mb-1">
                            <Star className="w-4 h-4 text-accent-success" />
                            <span className="text-sm font-medium text-accent-success">
                              Score: {review.score ?? 'N/A'}{assignment.points_possible ? ` / ${assignment.points_possible}` : ''}
                            </span>
                          </div>
                          {review.comments && (
                            <p className="text-sm text-accent-success mt-1">{review.comments}</p>
                          )}
                        </div>
                      )}

                      {/* Review form for pending reviews */}
                      {!isCompleted && isExpanded && (
                        <div className="mt-3 space-y-3 border-t pt-3">
                          <div>
                            <label className="block text-sm font-medium text-text-secondary mb-1">
                              Score {assignment.points_possible ? `(out of ${assignment.points_possible})` : ''}
                            </label>
                            <input
                              type="number"
                              min="0"
                              max={assignment.points_possible || undefined}
                              value={form.score ?? ''}
                              onChange={(e) => updateReviewForm(review.id, 'score', e.target.value)}
                              className="w-32 border border-border-strong rounded-md px-3 py-2 text-sm"
                              placeholder="0"
                            />
                          </div>
                          <div>
                            <label className="block text-sm font-medium text-text-secondary mb-1">Comments</label>
                            <textarea
                              value={form.comments ?? ''}
                              onChange={(e) => updateReviewForm(review.id, 'comments', e.target.value)}
                              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
                              rows={3}
                              placeholder="Provide feedback on this submission..."
                            />
                          </div>
                          <button
                            onClick={() => handleSubmitPeerReview(review.id)}
                            disabled={submittingReview === review.id || (!form.score && form.score !== 0)}
                            className="inline-flex items-center gap-1.5 px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 text-sm font-medium disabled:opacity-50"
                          >
                            <Send className="w-4 h-4" />
                            {submittingReview === review.id ? 'Submitting...' : 'Submit Review'}
                          </button>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Teacher View */}
      {isTeacher && (
        <div className="bg-surface-0 rounded-lg shadow">
          <div className="p-4 border-b flex items-center justify-between">
            <h3 className="font-semibold">Submissions ({submissions.length})</h3>
            <Link
              to={`/courses/${courseId}/assignments/${assignmentId}/speedgrader`}
              className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Zap className="w-4 h-4" />
              <span>SpeedGrader</span>
            </Link>
          </div>
          {submissions.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary">No submissions yet.</div>
          ) : (
            <div className="divide-y">
              {submissions.map((submission) => {
                const config = getStatusConfig(submission);
                const StatusIcon = config.icon;
                return (
                  <div key={submission.id || submission.user_id} className="p-4">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center space-x-3">
                        <User className="w-5 h-5 text-text-disabled" />
                        <span className="font-medium text-text-primary">
                          {getStudentName(submission)}
                        </span>
                        <span className={`text-xs font-medium px-2 py-0.5 rounded ${config.color}`}>
                          {config.label}
                        </span>
                      </div>
                      <div className="flex items-center space-x-3">
                        {submission.submitted_at && (
                          <span className="text-xs text-text-disabled">
                            {formatDate(submission.submitted_at)}
                          </span>
                        )}
                        <span className="text-sm font-semibold">
                          {submission.score !== null && submission.score !== undefined
                            ? `${submission.score} / ${assignment.points_possible}`
                            : `- / ${assignment.points_possible}`}
                        </span>
                      </div>
                    </div>

                    {submission.body && (
                      <div
                        className="text-sm text-text-secondary bg-surface-1 rounded p-3 mb-3 prose max-w-none"
                        dangerouslySetInnerHTML={{ __html: sanitizeHTML(submission.body) }}
                      />
                    )}

                    {/* Inline comments for each submission */}
                    <div className="mt-3 border-t pt-3">
                      <SubmissionComments
                        courseId={courseId}
                        assignmentId={assignmentId}
                        userId={submission.user_id}
                        isTeacher={true}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Peer Reviews Section (Teacher) */}
      {isTeacher && (
        <div className="bg-surface-0 rounded-lg shadow mt-6">
          <div className="p-4 border-b">
            <h3 className="font-semibold flex items-center gap-2">
              <MessageCircle className="w-5 h-5 text-purple-600" />
              Peer Reviews
            </h3>
          </div>
          <div className="p-4 space-y-4">
            {/* Assign Peer Reviews Controls */}
            <div className="flex items-center gap-3 flex-wrap">
              <label className="text-sm font-medium text-text-secondary">Reviews per student:</label>
              <input
                type="number"
                min="1"
                max="10"
                value={peerReviewCount}
                onChange={(e) => setPeerReviewCount(Math.max(1, parseInt(e.target.value) || 1))}
                className="w-20 border border-border-strong rounded-md px-3 py-2 text-sm"
              />
              <button
                onClick={handleAssignPeerReviews}
                disabled={assigningReviews}
                className="inline-flex items-center gap-1.5 px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 text-sm font-medium disabled:opacity-50"
              >
                <Users className="w-4 h-4" />
                {assigningReviews ? 'Assigning...' : 'Assign Peer Reviews'}
              </button>
            </div>

            {/* List of Assigned Peer Reviews */}
            {peerReviews.length > 0 ? (
              <div>
                <h4 className="text-sm font-medium text-text-secondary mb-2">
                  Assigned Reviews ({peerReviews.length})
                </h4>
                <div className="border rounded-md divide-y">
                  {peerReviews.map((review) => {
                    const statusConfig = getPeerReviewStatusConfig(review.workflow_state || review.status);
                    const ReviewStatusIcon = statusConfig.icon;
                    const reviewerName = review.reviewer_name || review.reviewer?.name || `User ${review.reviewer_id || ''}`;
                    const revieweeName = review.reviewee_name || review.reviewee?.name || review.submission?.user_name || `User ${review.reviewee_id || review.user_id || ''}`;

                    return (
                      <div key={review.id} className="p-3 flex items-center justify-between">
                        <div className="flex items-center gap-2 text-sm">
                          <User className="w-4 h-4 text-text-disabled" />
                          <span className="font-medium text-text-primary">{reviewerName}</span>
                          <ArrowRight className="w-4 h-4 text-text-disabled" />
                          <span className="text-text-secondary">{revieweeName}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          {(review.workflow_state === 'completed' || review.status === 'completed' || review.workflow_state === 'submitted' || review.status === 'submitted') && review.score != null && (
                            <span className="text-xs text-text-tertiary">
                              Score: {review.score}{assignment.points_possible ? `/${assignment.points_possible}` : ''}
                            </span>
                          )}
                          <span className={`text-xs font-medium px-2 py-0.5 rounded inline-flex items-center gap-1 ${statusConfig.color}`}>
                            <ReviewStatusIcon className="w-3 h-3" />
                            {statusConfig.label}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : (
              <p className="text-sm text-text-tertiary">
                No peer reviews have been assigned yet. Use the button above to assign reviews.
              </p>
            )}
          </div>
        </div>
      )}
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default AssignmentPage;
