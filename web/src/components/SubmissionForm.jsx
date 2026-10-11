import React, { useEffect, useState } from 'react';
import { Send, Clock, CheckCircle, Upload, Link as LinkIcon, FileText, Paperclip, Mic, PenLine, Plug, X } from 'lucide-react';
import { SubmissionBody } from './SafeHTML';
import RichContentEditorV2 from './rce/RichContentEditorV2';
import { clearRceDraft } from './rce/rceDrafts';
import { hasRichContent } from '../utils/richContent';
import MediaRecorderButton from './grading/MediaRecorderButton';
import AttachmentPreview, { formatSize } from './grading/AttachmentPreview';
import AnnotationSubmission from './grading/AnnotatableDocument';

const MAX_UPLOAD_FILES = 20;

// fileExtension returns a file name's lower-case extension without the dot.
export const fileExtension = (name) => {
  const m = /\.([^.]+)$/.exec(String(name || ''));
  return m ? m[1].toLowerCase() : '';
};

// attemptsRemaining mirrors the server's limit: allowed_attempts plus
// the instructor's extra attempts, minus the attempts already used.
// null means unlimited.
export const attemptsRemaining = (assignment, submission) => {
  const allowed = assignment?.allowed_attempts;
  if (!allowed || allowed <= 0) return null;
  const used = submission && submission.workflow_state !== 'unsubmitted' && submission.submitted_at ? (submission.attempt || 0) : 0;
  return Math.max(0, allowed + (submission?.extra_attempts || 0) - used);
};

// LocalMediaPreview plays a recorded / chosen file before it is uploaded.
const LocalMediaPreview = ({ file }) => {
  const [src, setSrc] = useState('');
  useEffect(() => {
    if (!file || typeof URL.createObjectURL !== 'function') return undefined;
    const u = URL.createObjectURL(file);
    setSrc(u);
    return () => URL.revokeObjectURL?.(u);
  }, [file]);
  if (!src) return null;
  return <AttachmentPreview file={{ url: src, content_type: file.type, display_name: file.name }} showLink={false} />;
};

const SubmissionForm = ({
  courseId,
  assignmentId,
  existingSubmission,
  onSubmit,
  submissionTypes = ['online_text_entry'],
  assignment = null,
  currentUserId = null,
}) => {
  const types = Array.isArray(submissionTypes)
    ? submissionTypes.map(t => t.trim())
    : String(submissionTypes).split(',').map(t => t.trim());

  const hasText = types.includes('online_text_entry');
  const hasUpload = types.includes('online_upload');
  const hasUrl = types.includes('online_url');
  const hasMedia = types.includes('media_recording');
  const hasAnnotation = types.includes('student_annotation');
  const hasExternalTool = types.includes('external_tool');
  const submittableTypes = [
    hasText && 'text',
    hasUpload && 'file',
    hasUrl && 'url',
    hasMedia && 'media',
    hasAnnotation && 'annotation',
    hasExternalTool && 'external_tool',
  ].filter(Boolean);

  const allowedExtensions = Array.isArray(assignment?.allowed_extensions) ? assignment.allowed_extensions : [];
  const remaining = attemptsRemaining(assignment, existingSubmission);
  const outOfAttempts = remaining === 0;

  const [activeTab, setActiveTab] = useState(submittableTypes[0] || 'text');
  const [body, setBody] = useState('');
  // Draft key for the text-entry editor (per assignment and browser).
  const draftKey = `submission-${courseId}-${assignmentId}`;
  // Bumped after a submit so the editor remounts empty.
  const [editorKey, setEditorKey] = useState(0);
  const [url, setUrl] = useState('');
  const [files, setFiles] = useState([]);
  const [mediaFile, setMediaFile] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(false);
    try {
      if (activeTab === 'text') {
        if (!hasRichContent(body)) return;
        // The body is rich content (HTML from the editor, as in Canvas); the
        // server sanitizes it at submit time.
        await onSubmit({ submission_type: 'online_text_entry', body });
        clearRceDraft(draftKey);
        setBody('');
        setEditorKey((k) => k + 1);
      } else if (activeTab === 'url') {
        if (!url.trim()) return;
        await onSubmit({ submission_type: 'online_url', url });
        setUrl('');
      } else if (activeTab === 'file') {
        if (files.length === 0) return;
        const bad = files.find((f) => allowedExtensions.length > 0 && !allowedExtensions.includes(fileExtension(f.name)));
        if (bad) throw new Error(`${bad.name}: allowed file types are ${allowedExtensions.join(', ')}`);
        await onSubmit({ submission_type: 'online_upload', files });
        setFiles([]);
      } else if (activeTab === 'media') {
        if (!mediaFile) return;
        await onSubmit({ submission_type: 'media_recording', files: [mediaFile] });
        setMediaFile(null);
      }
      setSuccess(true);
      setTimeout(() => setSuccess(false), 5000);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const isValid = () => {
    if (activeTab === 'text') return hasRichContent(body);
    if (activeTab === 'url') return url.trim().length > 0;
    if (activeTab === 'file') return files.length > 0;
    if (activeTab === 'media') return mediaFile !== null;
    return false;
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleString();
  };

  const tabLabel = {
    text: 'Text Entry', file: 'File Upload', url: 'Website URL', media: 'Media Recording',
    annotation: 'Student Annotation', external_tool: 'External Tool',
  };
  const tabIcon = { text: FileText, file: Upload, url: LinkIcon, media: Mic, annotation: PenLine, external_tool: Plug };
  const addFiles = (list) => setFiles((prev) => [...prev, ...Array.from(list || [])].slice(0, MAX_UPLOAD_FILES));

  return (
    <div className="space-y-4">
      {existingSubmission && existingSubmission.workflow_state !== 'unsubmitted' && (
        <div className="bg-surface-1 rounded-lg p-4 border">
          <div className="flex items-center justify-between mb-2">
            <h4 className="font-medium text-text-primary flex items-center space-x-2">
              <CheckCircle className="w-4 h-4 text-accent-success" />
              <span>Previous Submission</span>
            </h4>
            {existingSubmission.attempt && (
              <span className="text-xs text-text-tertiary bg-border-default px-2 py-1 rounded">
                Attempt {existingSubmission.attempt}
              </span>
            )}
          </div>
          {existingSubmission.submitted_at && (
            <div className="flex items-center space-x-1 text-sm text-text-tertiary mb-2">
              <Clock className="w-3 h-3" />
              <span>Submitted {formatDate(existingSubmission.submitted_at)}</span>
            </div>
          )}
          {existingSubmission.body && (
            <SubmissionBody
              className="text-sm text-text-secondary prose max-w-none bg-surface-0 p-3 rounded border"
              body={existingSubmission.body}
            />
          )}
          {existingSubmission.url && (
            <div className="text-sm text-text-secondary bg-surface-0 p-3 rounded border">
              <a href={existingSubmission.url} target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline">
                {existingSubmission.url}
              </a>
            </div>
          )}
          {existingSubmission.attachments && existingSubmission.attachments.length > 0 && (
            <div className="bg-surface-0 p-3 rounded border">
              <p className="text-xs font-medium text-text-tertiary mb-2 flex items-center gap-1">
                <Paperclip className="w-3 h-3" /> Attached Files
              </p>
              <div className="space-y-1.5">
                {existingSubmission.attachments.map((att) => (
                  existingSubmission.submission_type === 'media_recording' ? (
                    <AttachmentPreview key={att.id} file={att} showLink />
                  ) : (
                    <a
                      key={att.id}
                      href={att.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-2 text-sm text-brand-600 hover:underline"
                    >
                      <Paperclip className="w-3.5 h-3.5 flex-shrink-0" />
                      <span>{att.display_name || att.filename}</span>
                      {att.size ? <span className="text-xs text-text-disabled">({formatSize(att.size)})</span> : null}
                    </a>
                  )
                ))}
              </div>
            </div>
          )}
          {existingSubmission.grade !== null && existingSubmission.grade !== undefined && (
            <div className="mt-2 text-sm">
              <span className="font-medium text-text-secondary">Grade: </span>
              <span className="text-brand-600 font-semibold">{existingSubmission.grade}</span>
              {existingSubmission.score !== null && existingSubmission.score !== undefined && (
                <span className="text-text-tertiary"> ({existingSubmission.score} pts)</span>
              )}
            </div>
          )}
        </div>
      )}

      {remaining !== null && (
        <p className={`text-sm ${outOfAttempts ? 'text-accent-danger' : 'text-text-tertiary'}`} role="status">
          {outOfAttempts
            ? 'You have used all of your attempts for this assignment.'
            : `${remaining} attempt${remaining === 1 ? '' : 's'} remaining`}
        </p>
      )}

      {/* Submission type tabs */}
      {submittableTypes.length > 1 && (
        <div className="flex border-b border-border-default">
          {submittableTypes.map((tab) => {
            const Icon = tabIcon[tab];
            return (
              <button
                key={tab}
                type="button"
                onClick={() => setActiveTab(tab)}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab
                    ? 'border-brand-600 text-brand-600'
                    : 'border-transparent text-text-tertiary hover:text-text-secondary'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tabLabel[tab]}
              </button>
            );
          })}
        </div>
      )}

      {activeTab === 'annotation' ? (
        <AnnotationSubmission
          courseId={courseId}
          assignmentId={assignmentId}
          userId={currentUserId}
          disabled={outOfAttempts}
          onSubmit={async (data) => {
            await onSubmit(data);
            setSuccess(true);
            setTimeout(() => setSuccess(false), 5000);
          }}
        />
      ) : (
      <form onSubmit={handleSubmit}>
        <label className="block text-sm font-medium text-text-secondary mb-1">
          {existingSubmission && existingSubmission.workflow_state !== 'unsubmitted'
            ? 'Resubmit'
            : 'Your Submission'}
        </label>

        {activeTab === 'text' && (
          // Rich content editor, as in Canvas (G010): formatting, equations,
          // links, images and recorded audio/video. No AI Assist for student
          // work.
          <RichContentEditorV2
            key={editorKey}
            value={body}
            onChange={setBody}
            placeholder="Enter your submission text here..."
            courseId={courseId}
            autoSaveKey={draftKey}
            aiAssist={false}
          />
        )}

        {activeTab === 'url' && (
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            className="w-full border border-border-strong rounded-lg p-3 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
            placeholder="https://example.com"
            disabled={submitting}
          />
        )}

        {activeTab === 'file' && (
          <div className="border-2 border-dashed border-border-strong rounded-lg p-6 text-center hover:border-blue-400 transition-colors space-y-3">
            {files.length > 0 && (
              <ul className="space-y-1 text-left">
                {files.map((f, i) => (
                  <li key={`${f.name}-${i}`} className="flex items-center gap-3">
                    <Upload className="w-4 h-4 text-brand-600" />
                    <span className="text-sm text-text-primary font-medium truncate">{f.name}</span>
                    <span className="text-xs text-text-disabled">({(f.size / 1024).toFixed(1)} KB)</span>
                    <button
                      type="button"
                      onClick={() => setFiles((prev) => prev.filter((_, j) => j !== i))}
                      className="text-accent-danger text-sm ml-auto"
                      aria-label={`Remove ${f.name}`}
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <label className="cursor-pointer block">
              <Upload className="w-8 h-8 text-text-disabled mx-auto mb-2" />
              <p className="text-sm text-text-secondary">
                {files.length > 0 ? 'Add another file' : 'Click to choose files or drag and drop'}
              </p>
              <p className="text-xs text-text-disabled mt-1">
                {allowedExtensions.length > 0
                  ? `Allowed file types: ${allowedExtensions.join(', ')}`
                  : 'PDF, Word, images, or other supported formats'}
              </p>
              <input
                type="file"
                multiple
                data-testid="submission-file-input"
                accept={allowedExtensions.length > 0 ? allowedExtensions.map((e) => `.${e}`).join(',') : undefined}
                onChange={(e) => { addFiles(e.target.files); e.target.value = ''; }}
                className="hidden"
                disabled={submitting}
              />
            </label>
          </div>
        )}

        {activeTab === 'media' && (
          <div className="border-2 border-dashed border-border-strong rounded-lg p-6 space-y-3">
            {mediaFile ? (
              <div className="space-y-2">
                <LocalMediaPreview file={mediaFile} />
                <div className="flex items-center gap-3 text-sm">
                  <span className="text-text-primary font-medium truncate">{mediaFile.name}</span>
                  <button type="button" onClick={() => setMediaFile(null)} className="text-accent-danger">Remove</button>
                </div>
              </div>
            ) : (
              <div className="flex flex-col items-center gap-3 text-center">
                <p className="text-sm text-text-secondary">Record audio or video, or upload a media file.</p>
                <MediaRecorderButton disabled={submitting} onRecorded={setMediaFile} />
                <label className="cursor-pointer text-sm text-brand-600 hover:underline">
                  Upload a media file
                  <input
                    type="file"
                    accept="audio/*,video/*"
                    data-testid="media-file-input"
                    className="hidden"
                    onChange={(e) => { setMediaFile(e.target.files?.[0] || null); e.target.value = ''; }}
                  />
                </label>
              </div>
            )}
          </div>
        )}

        {activeTab === 'external_tool' && (
          <div className="border border-border-default rounded-lg p-6 text-sm text-text-secondary">
            This assignment is completed in an external tool. Open it from the course, and your work is
            submitted here automatically when the tool reports it.
          </div>
        )}

        {success && (
          <div className="flex items-center gap-2 text-accent-success bg-accent-success/10 border border-accent-success/30 rounded-md p-3 text-sm">
            <CheckCircle className="w-4 h-4 flex-shrink-0" />
            <span>Submission recorded successfully{existingSubmission?.attempt ? ` (Attempt ${(existingSubmission.attempt || 0) + 1})` : ''}!</span>
          </div>
        )}
        {error && (
          <p className="text-accent-danger text-sm mt-1">{error}</p>
        )}
        <div className="flex items-center justify-between mt-2">
          {existingSubmission && existingSubmission.attempt > 0 && (
            <span className="text-xs text-text-disabled">
              Current attempt: {existingSubmission.attempt}
            </span>
          )}
          <button
            type="submit"
            disabled={submitting || !isValid() || outOfAttempts || activeTab === 'external_tool'}
            className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium ml-auto"
          >
            <Send className="w-4 h-4" />
            <span>{submitting ? 'Submitting...' : 'Submit Assignment'}</span>
          </button>
        </div>
      </form>
      )}
      {activeTab === 'annotation' && success && (
        <div className="flex items-center gap-2 text-accent-success bg-accent-success/10 border border-accent-success/30 rounded-md p-3 text-sm">
          <CheckCircle className="w-4 h-4 flex-shrink-0" />
          <span>Annotations submitted.</span>
        </div>
      )}
    </div>
  );
};

export default SubmissionForm;
