import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { FileCheck, ChevronDown, ChevronRight, Plus, Eye, EyeOff, Search, Users } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { Skeleton } from '@/components/ui/skeleton';

const AssignmentsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [assignments, setAssignments] = useState([]);
  const [groups, setGroups] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionError, setActionError] = useState(null);
  const [collapsedGroups, setCollapsedGroups] = useState({});
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [newAssignment, setNewAssignment] = useState({
    name: '',
    description: '',
    points_possible: 100,
    due_at: '',
    submission_types: ['online_text_entry'],
    anonymous_grading: false,
    post_policy: 'automatic',
    is_group_assignment: false,
    group_category_id: '',
  });
  const [groupCategories, setGroupCategories] = useState([]);
  const [loadingGroupCategories, setLoadingGroupCategories] = useState(false);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  const fetchData = async () => {
    try {
      const [assignmentsResult, groupsResult] = await Promise.all([
        api.getAssignments(courseId),
        api.getAssignmentGroups(courseId),
      ]);
      setAssignments(assignmentsResult.data || []);
      setGroups(groupsResult.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [courseId]);

  useEffect(() => {
    if (showCreate && groupCategories.length === 0 && !loadingGroupCategories) {
      setLoadingGroupCategories(true);
      api.getGroupCategories(courseId)
        .then((result) => setGroupCategories(result.data || []))
        .catch(() => setGroupCategories([]))
        .finally(() => setLoadingGroupCategories(false));
    }
  }, [showCreate, courseId]);

  const doCreate = async () => {
    setCreating(true);
    setActionError(null);
    try {
      const payload = {
        name: newAssignment.name,
        description: newAssignment.description,
        points_possible: Number(newAssignment.points_possible),
        due_at: newAssignment.due_at ? new Date(newAssignment.due_at).toISOString() : null,
        submission_types: newAssignment.submission_types,
        anonymous_grading: newAssignment.anonymous_grading,
        post_policy: newAssignment.post_policy,
        published: true,
      };
      if (newAssignment.is_group_assignment && newAssignment.group_category_id) {
        payload.group_category_id = Number(newAssignment.group_category_id);
      }
      const created = await api.createAssignment(courseId, payload);
      setAssignments((prev) => [...prev, created]);
      setNewAssignment({ name: '', description: '', points_possible: 100, due_at: '', submission_types: ['online_text_entry'], anonymous_grading: false, post_policy: 'automatic', is_group_assignment: false, group_category_id: '' });
      setShowCreate(false);
    } catch (err) {
      setActionError(err.message || 'Could not create assignment. Please try again.');
    } finally {
      setCreating(false);
    }
  };

  const handleCreate = (e) => {
    e.preventDefault();
    checkAndSave(newAssignment.description, doCreate);
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const toggleGroup = (groupId) => {
    setCollapsedGroups((prev) => ({
      ...prev,
      [groupId]: !prev[groupId],
    }));
  };

  const togglePublish = async (e, assignment) => {
    e.preventDefault();
    e.stopPropagation();
    const newPublished = !assignment.published;
    try {
      await api.updateAssignment(courseId, assignment.id, { published: newPublished });
      setAssignments((prev) =>
        prev.map((a) => a.id === assignment.id ? { ...a, published: newPublished } : a)
      );
    } catch (err) {
      setActionError(err.message || 'Could not update assignment. Please try again.');
    }
  };

  // Filter assignments by search query and status
  const now = new Date();
  const filteredAssignments = assignments.filter((a) => {
    if (searchQuery && !a.name.toLowerCase().includes(searchQuery.toLowerCase())) return false;
    if (statusFilter === 'published') return a.published !== false;
    if (statusFilter === 'unpublished') return a.published === false;
    if (statusFilter === 'upcoming') return a.due_at && new Date(a.due_at) >= now;
    if (statusFilter === 'past_due') return a.due_at && new Date(a.due_at) < now;
    return true;
  });

  // Group assignments by assignment_group_id
  const assignmentsByGroup = {};
  const ungrouped = [];

  filteredAssignments.forEach((assignment) => {
    const groupId = assignment.assignment_group_id;
    if (groupId) {
      if (!assignmentsByGroup[groupId]) {
        assignmentsByGroup[groupId] = [];
      }
      assignmentsByGroup[groupId].push(assignment);
    } else {
      ungrouped.push(assignment);
    }
  });

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }
  if (error) {
    return <Layout><div className="text-center py-12">
  <p className="text-accent-danger mb-3">{error}</p>
  <button onClick={() => { setError(null); setLoading(true); fetchData(); }} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
</div></Layout>;
  }

  const renderAssignment = (assignment) => (
    <Link
      key={assignment.id}
      to={`/courses/${courseId}/assignments/${assignment.id}`}
      className="flex items-center justify-between p-4 hover:bg-surface-1"
    >
      <div className="flex items-center space-x-3 min-w-0">
        <FileCheck className="w-5 h-5 text-text-disabled flex-shrink-0" />
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-text-primary truncate">{assignment.name}</span>
            {assignment.is_group_assignment && (
              <span className="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 rounded bg-indigo-100 text-indigo-700">
                <Users className="w-3 h-3" />
                Group
              </span>
            )}
            {assignment.published === false && (
              <span className="text-xs px-1.5 py-0.5 rounded bg-border-default text-text-tertiary">Unpublished</span>
            )}
          </div>
          <div className="flex items-center space-x-3 text-xs text-text-disabled mt-0.5">
            {assignment.due_at && (
              <span>Due {formatDate(assignment.due_at)}</span>
            )}
            {assignment.points_possible != null && (
              <span>{assignment.points_possible} pts</span>
            )}
          </div>
        </div>
      </div>
      {isTeacher && (
        <button
          onClick={(e) => togglePublish(e, assignment)}
          className={`flex-shrink-0 p-1.5 rounded-md transition-colors ${
            assignment.published !== false
              ? 'text-accent-success hover:bg-accent-success/10'
              : 'text-text-disabled hover:bg-surface-2'
          }`}
          title={assignment.published !== false ? 'Unpublish' : 'Publish'}
        >
          {assignment.published !== false ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
        </button>
      )}
    </Link>
  );

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Assignments</h2>
          {isTeacher && (
            <button
              onClick={() => setShowCreate(!showCreate)}
              className="inline-flex items-center px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4 mr-1" />
              Assignment
            </button>
          )}
        </div>
      </div>

      {/* Search and Filter */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-disabled" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search assignments..."
            aria-label="Search assignments"
            className="w-full pl-9 pr-3 py-2 border border-border-strong rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="border border-border-strong rounded-md px-3 py-2 text-sm text-text-secondary"
        >
          <option value="all">All ({assignments.length})</option>
          <option value="published">Published</option>
          <option value="unpublished">Unpublished</option>
          <option value="upcoming">Upcoming</option>
          <option value="past_due">Past Due</option>
        </select>
      </div>

      {actionError && (
        <div
          role="alert"
          className="mb-4 flex items-start justify-between gap-3 rounded-md border border-accent-danger/30 bg-accent-danger/10 px-4 py-3 text-sm text-accent-danger"
        >
          <div className="flex-1">
            <p className="font-medium">Something went wrong</p>
            <p className="mt-0.5 text-text-secondary">{actionError}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => { setActionError(null); doCreate(); }}
              className="text-accent-danger hover:underline font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 rounded"
            >
              Try Again
            </button>
            <button
              type="button"
              onClick={() => setActionError(null)}
              className="text-text-tertiary hover:text-text-secondary focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 rounded"
              aria-label="Dismiss error"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} className="bg-surface-0 rounded-lg shadow p-6 mb-6 space-y-4">
          <div>
            <label htmlFor="assignment-name" className="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input
              id="assignment-name"
              type="text"
              required
              value={newAssignment.name}
              onChange={(e) => setNewAssignment({ ...newAssignment, name: e.target.value })}
              className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              placeholder="Assignment name"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
            <RichContentEditorV2
              value={newAssignment.description}
              onChange={(html) => setNewAssignment((prev) => ({ ...prev, description: html }))}
              placeholder="Assignment instructions..."
              minHeight="140px"
              courseId={courseId}
              autoSaveKey={`assignment-${courseId}-new-description`}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="assignment-points" className="block text-sm font-medium text-text-secondary mb-1">Points</label>
              <input
                id="assignment-points"
                type="number"
                min="0"
                value={newAssignment.points_possible}
                onChange={(e) => setNewAssignment({ ...newAssignment, points_possible: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label htmlFor="assignment-due-date" className="block text-sm font-medium text-text-secondary mb-1">Due Date</label>
              <input
                id="assignment-due-date"
                type="datetime-local"
                value={newAssignment.due_at}
                onChange={(e) => setNewAssignment({ ...newAssignment, due_at: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <label className="flex items-center gap-2 text-sm text-text-secondary">
              <input
                type="checkbox"
                checked={newAssignment.anonymous_grading}
                onChange={(e) => setNewAssignment({ ...newAssignment, anonymous_grading: e.target.checked })}
                className="rounded border-border-strong"
              />
              Anonymous Grading
            </label>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Grade Posting</label>
              <select
                value={newAssignment.post_policy}
                onChange={(e) => setNewAssignment({ ...newAssignment, post_policy: e.target.value })}
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
                checked={newAssignment.is_group_assignment}
                onChange={(e) => {
                  const checked = e.target.checked;
                  setNewAssignment({ ...newAssignment, is_group_assignment: checked, group_category_id: checked ? newAssignment.group_category_id : '' });
                }}
                className="rounded border-border-strong"
              />
              <Users className="w-4 h-4 text-text-tertiary" />
              Group Assignment
            </label>
            {newAssignment.is_group_assignment && (
              <div className="mt-2 ml-6">
                <label className="block text-sm font-medium text-text-secondary mb-1">Group Category</label>
                {loadingGroupCategories ? (
                  <p className="text-sm text-text-disabled">Loading group categories...</p>
                ) : groupCategories.length === 0 ? (
                  <p className="text-sm text-text-disabled">No group categories found for this course. Create one on the Groups page first.</p>
                ) : (
                  <select
                    value={newAssignment.group_category_id}
                    onChange={(e) => setNewAssignment({ ...newAssignment, group_category_id: e.target.value })}
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
          <div className="flex justify-end space-x-3">
            <button type="button" onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary">
              Cancel
            </button>
            <button type="submit" disabled={creating} className="px-4 py-2 bg-brand-600 text-white rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50">
              {creating ? 'Creating...' : 'Create Assignment'}
            </button>
          </div>
        </form>
      )}

      {assignments.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">
          No assignments yet.
        </div>
      ) : filteredAssignments.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">
          No assignments match your search.
        </div>
      ) : (
        <div className="space-y-4">
          {groups.map((group) => {
            const groupAssignments = assignmentsByGroup[group.id] || [];
            if (groupAssignments.length === 0) return null;

            const isCollapsed = collapsedGroups[group.id];

            return (
              <div key={group.id} className="bg-surface-0 rounded-lg shadow">
                <button
                  onClick={() => toggleGroup(group.id)}
                  className="w-full flex items-center justify-between p-4 border-b hover:bg-surface-1 text-left"
                >
                  <div className="flex items-center space-x-2">
                    {isCollapsed ? (
                      <ChevronRight className="w-4 h-4 text-text-disabled" />
                    ) : (
                      <ChevronDown className="w-4 h-4 text-text-disabled" />
                    )}
                    <h3 className="font-semibold text-text-primary">{group.name}</h3>
                    <span className="text-xs text-text-disabled">
                      ({groupAssignments.length} assignment{groupAssignments.length !== 1 ? 's' : ''})
                    </span>
                  </div>
                  {group.group_weight != null && group.group_weight > 0 && (
                    <span className="text-xs text-text-tertiary">{group.group_weight}% of grade</span>
                  )}
                </button>
                {!isCollapsed && (
                  <div className="divide-y">
                    {groupAssignments.map(renderAssignment)}
                  </div>
                )}
              </div>
            );
          })}

          {/* Ungrouped assignments */}
          {ungrouped.length > 0 && (
            <div className="bg-surface-0 rounded-lg shadow">
              <div className="p-4 border-b">
                <h3 className="font-semibold text-text-primary">Other Assignments</h3>
              </div>
              <div className="divide-y">
                {ungrouped.map(renderAssignment)}
              </div>
            </div>
          )}
        </div>
      )}
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default AssignmentsPage;
