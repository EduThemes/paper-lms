import React, { useState } from 'react';
import { PenLine, Send } from 'lucide-react';
import gradingApi from '../../services/api_grading_ui';
import DocumentViewer from '../DocumentViewer';
import { previewKind } from './AttachmentPreview';

// The document of a student_annotation assignment rendered for marking
// up. The annotations hang off the student's submission row
// (DocViewer annotations), so a grader opens the same view read-only.

export const DocumentImage = ({ doc, height = 720 }) => {
  if (!doc?.url) return null;
  const kind = previewKind(doc);
  if (kind === 'image') {
    return <img src={doc.url} alt={doc.display_name || 'Document'} className="max-w-full" />;
  }
  return (
    <iframe
      title={doc.display_name || 'Document'}
      src={doc.url}
      className="w-full bg-surface-1 border border-border-default rounded"
      style={{ height }}
    />
  );
};

// AnnotationSubmission is the student's flow: open the document,
// annotate it, then submit.
const AnnotationSubmission = ({ courseId, assignmentId, userId, onSubmit, disabled = false }) => {
  const [session, setSession] = useState(null);
  const [starting, setStarting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);

  const start = async () => {
    setStarting(true);
    setError(null);
    try {
      setSession(await gradingApi.startAnnotation(courseId, assignmentId));
    } catch (err) {
      setError(err.message);
    } finally {
      setStarting(false);
    }
  };

  const submit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({ submission_type: 'student_annotation' });
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  if (!session) {
    return (
      <div className="border-2 border-dashed border-border-strong rounded-lg p-6 text-center space-y-2">
        <PenLine className="w-8 h-8 text-text-tertiary mx-auto" />
        <p className="text-sm text-text-secondary">Annotate the document your instructor provided, then submit it.</p>
        <button
          type="button"
          onClick={start}
          disabled={starting || disabled}
          className="inline-flex items-center gap-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          {starting ? 'Opening...' : 'Open document to annotate'}
        </button>
        {error && <p className="text-sm text-accent-danger">{error}</p>}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="h-[720px]">
        <DocumentViewer
          courseId={courseId}
          assignmentId={assignmentId}
          userId={userId}
          submissionId={session.submission?.id}
          documentTitle={session.document?.display_name}
          documentNode={<DocumentImage doc={session.document} />}
        />
      </div>
      {error && <p className="text-sm text-accent-danger">{error}</p>}
      <div className="flex justify-end">
        <button
          type="button"
          onClick={submit}
          disabled={submitting || disabled}
          className="inline-flex items-center gap-2 bg-brand-600 text-on-brand px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          <Send className="w-4 h-4" />
          {submitting ? 'Submitting...' : 'Submit annotations'}
        </button>
      </div>
    </div>
  );
};

export default AnnotationSubmission;
