import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Eye, RotateCcw, LogOut } from 'lucide-react';
import { pagesFilesApi } from '../services/api_pages_files';

// useStudentViewStatus asks the server whether this (masquerade) session
// is Student View (G076). Returns the course id or null.
export function useStudentViewStatus(enabled) {
  const query = useQuery({
    queryKey: ['users', 'self', 'student_view'],
    queryFn: () => pagesFilesApi.getStudentViewStatus(),
    enabled: Boolean(enabled),
    staleTime: 60_000,
    retry: false,
  });
  return enabled ? query.data?.course_id ?? null : null;
}

// StudentViewBanner replaces the masquerade banner while a teacher is in
// Student View: Reset Student starts over with a fresh test student,
// Leave Student View returns to the teacher's own session on the course.
export default function StudentViewBanner({ courseId }) {
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState(null);

  const run = async (kind, fn) => {
    setBusy(kind);
    setError(null);
    try {
      await fn();
      window.location.assign(`/courses/${courseId}`);
    } catch (err) {
      setError(err.message);
      setBusy(null);
    }
  };

  return (
    <div
      className="fixed top-0 left-0 right-0 z-[60] bg-brand-700 text-on-brand px-4 py-2 flex flex-wrap items-center justify-center gap-3 shadow-md"
      role="alert"
      aria-live="polite"
    >
      <Eye className="w-4 h-4 flex-shrink-0" aria-hidden="true" />
      <span className="text-sm font-medium">You are viewing this course as a test student.</span>
      <button
        type="button"
        disabled={busy != null}
        onClick={() => run('reset', () => pagesFilesApi.resetStudentView(courseId))}
        className="inline-flex items-center gap-1 px-3 py-1 text-xs font-semibold bg-black/20 rounded hover:bg-black/30 disabled:opacity-50"
      >
        <RotateCcw className="w-3.5 h-3.5" aria-hidden="true" />
        {busy === 'reset' ? 'Resetting…' : 'Reset Student'}
      </button>
      <button
        type="button"
        disabled={busy != null}
        onClick={() => run('leave', () => pagesFilesApi.leaveStudentView())}
        className="inline-flex items-center gap-1 px-3 py-1 text-xs font-semibold bg-black/20 rounded hover:bg-black/30 disabled:opacity-50"
      >
        <LogOut className="w-3.5 h-3.5" aria-hidden="true" />
        {busy === 'leave' ? 'Leaving…' : 'Leave Student View'}
      </button>
      {error && <span className="text-xs">{error}</span>}
    </div>
  );
}
