import React, { useState } from 'react';
import { Eye, Globe, EyeOff, UserCheck } from 'lucide-react';
import { api } from '../services/api';
import { pagesFilesApi } from '../services/api_pages_files';
import { publishSteps, unpublishSteps, isCoursePublished, courseStateLabel } from '../utils/coursePublish';

// CourseStaffBar sits on the course home for staff: the course's publish
// state with Publish / Unpublish (G077) and the Student View entry point
// (G076).
export default function CourseStaffBar({ course, onCourseChange }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const published = isCoursePublished(course.workflow_state);

  const changeState = async (steps) => {
    setBusy(true);
    setError(null);
    try {
      let updated = course;
      for (const state of steps) {
        // eslint-disable-next-line no-await-in-loop
        updated = await api.updateCourse(course.id, { workflow_state: state });
      }
      onCourseChange?.(updated);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const enterStudentView = async () => {
    setBusy(true);
    setError(null);
    try {
      await pagesFilesApi.enterStudentView(course.id);
      // A full load drops every cached staff-only query (roles, rosters).
      window.location.assign(`/courses/${course.id}`);
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  };

  const toPublish = publishSteps(course.workflow_state);
  const toUnpublish = unpublishSteps(course.workflow_state);

  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Course status">
      <span
        className={`inline-flex items-center gap-1 rounded-pill px-2.5 py-1 text-xs font-medium ${
          published ? 'bg-accent-success/10 text-accent-success' : 'bg-accent-warning/10 text-accent-warning'
        }`}
      >
        {published ? <Globe className="w-3.5 h-3.5" aria-hidden="true" /> : <EyeOff className="w-3.5 h-3.5" aria-hidden="true" />}
        {courseStateLabel(course.workflow_state)}
      </span>
      {toPublish.length > 0 && (
        <button
          type="button"
          disabled={busy}
          onClick={() => changeState(toPublish)}
          className="px-3 py-1.5 rounded-md text-sm font-medium bg-brand-600 text-on-brand hover:bg-brand-700 disabled:opacity-50"
        >
          Publish course
        </button>
      )}
      {toUnpublish.length > 0 && (
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            if (window.confirm('Unpublish this course? Students will lose access until you publish it again.')) changeState(toUnpublish);
          }}
          className="px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default disabled:opacity-50"
        >
          Unpublish
        </button>
      )}
      <button
        type="button"
        disabled={busy}
        onClick={enterStudentView}
        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-surface-2 text-text-secondary hover:bg-border-default disabled:opacity-50"
      >
        <Eye className="w-4 h-4" aria-hidden="true" />
        Student View
      </button>
      {!published && (
        <span className="text-xs text-text-tertiary inline-flex items-center gap-1">
          <UserCheck className="w-3.5 h-3.5" aria-hidden="true" /> Students can&apos;t open the course until it is published.
        </span>
      )}
      {error && <p role="alert" className="w-full text-sm text-accent-danger">{error}</p>}
    </div>
  );
}
