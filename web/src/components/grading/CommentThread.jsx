import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Send, MessageSquare, Eye, EyeOff, Paperclip, Pencil, Trash2, Users, X, Check } from 'lucide-react';
import gradingApi from '../../services/api_grading_ui';
import MediaRecorderButton from './MediaRecorderButton';
import AttachmentPreview from './AttachmentPreview';

// Submission comments with files, audio/video recordings, group comments
// and grader drafts (Canvas submission comments). The submission is
// addressed by submissionKey: { userId } or, under anonymous grading,
// { anonymousId } so the student's id never reaches the page.

const MAX_FILES = 20;

const CommentThread = ({
  courseId,
  assignmentId,
  submissionKey,
  isTeacher = false,
  currentUserId = null,
  isGroupAssignment = false,
  initialComments = null,
  onCommentsChange,
  title = 'Comments',
  compact = false,
}) => {
  const [comments, setComments] = useState(initialComments || []);
  const [loading, setLoading] = useState(initialComments == null);
  const [error, setError] = useState(null);
  const [text, setText] = useState('');
  const [files, setFiles] = useState([]);
  const [isDraft, setIsDraft] = useState(false);
  const [toGroup, setToGroup] = useState(isGroupAssignment);
  const [submitting, setSubmitting] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [editText, setEditText] = useState('');
  const fileInputRef = useRef(null);

  const keyId = submissionKey?.anonymousId || submissionKey?.userId;

  // initialComments seeds the thread (SpeedGrader already has them);
  // otherwise the thread loads its comments. The parent remounts the
  // thread (key) when the submission changes.
  useEffect(() => {
    if (initialComments != null || !keyId) return undefined;
    let cancelled = false;
    setLoading(true);
    gradingApi
      .listComments(courseId, assignmentId, submissionKey)
      .then((list) => { if (!cancelled) setComments(Array.isArray(list) ? list : []); })
      .catch((err) => { if (!cancelled) setError(err.message); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId, assignmentId, keyId]);

  useEffect(() => { setToGroup(isGroupAssignment); }, [isGroupAssignment, keyId]);

  const update = useCallback((next) => {
    setComments(next);
    onCommentsChange?.(next);
  }, [onCommentsChange]);

  const addFiles = (list) => {
    const incoming = Array.from(list || []);
    setFiles((prev) => [...prev, ...incoming].slice(0, MAX_FILES));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!text.trim() && files.length === 0) return;
    setSubmitting(true);
    setError(null);
    try {
      const fileIds = [];
      for (const f of files) {
        const uploaded = await gradingApi.uploadCommentFile(courseId, assignmentId, submissionKey, f);
        const id = uploaded?.id ?? uploaded?.data?.id;
        if (id) fileIds.push(id);
      }
      const payload = { text_comment: text };
      if (fileIds.length) payload.file_ids = fileIds;
      if (isTeacher && isDraft) payload.draft = true;
      if (isGroupAssignment && toGroup) payload.group_comment = true;
      const created = await gradingApi.createComment(courseId, assignmentId, submissionKey, payload);
      update([...comments, created]);
      setText('');
      setFiles([]);
      setIsDraft(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const saveEdit = async (comment) => {
    try {
      const saved = await gradingApi.updateComment(courseId, assignmentId, submissionKey, comment.id, { text_comment: editText });
      update(comments.map((c) => (c.id === comment.id ? { ...c, ...saved } : c)));
      setEditingId(null);
    } catch (err) {
      setError(err.message);
    }
  };

  const publishDraft = async (comment) => {
    try {
      const saved = await gradingApi.updateComment(courseId, assignmentId, submissionKey, comment.id, { draft: false });
      update(comments.map((c) => (c.id === comment.id ? { ...c, ...saved, draft: false } : c)));
    } catch (err) {
      setError(err.message);
    }
  };

  const remove = async (comment) => {
    if (!window.confirm('Delete this comment?')) return;
    try {
      await gradingApi.deleteComment(courseId, assignmentId, submissionKey, comment.id);
      update(comments.filter((c) => c.id !== comment.id));
    } catch (err) {
      setError(err.message);
    }
  };

  const isMine = (comment) => currentUserId != null && comment.author_id === currentUserId;

  return (
    <div className={compact ? 'space-y-3' : 'space-y-4'}>
      {title && (
        <div className="flex items-center space-x-2">
          <MessageSquare className="w-5 h-5 text-text-tertiary" />
          <h4 className="font-semibold text-text-primary">{title}</h4>
        </div>
      )}

      {loading ? (
        <div className="text-sm text-text-tertiary py-2">Loading comments...</div>
      ) : comments.length === 0 ? (
        <div className="text-sm text-text-disabled py-2">No comments yet.</div>
      ) : (
        <ul className="space-y-3">
          {comments.map((comment) => (
            <li
              key={comment.id}
              className={`rounded-lg p-3 text-sm ${
                comment.draft ? 'bg-accent-warning/10 border border-accent-warning/30' : 'bg-surface-1 border border-border-default'
              }`}
            >
              <div className="flex items-center justify-between mb-1 gap-2">
                <span className="font-medium text-text-primary">
                  {isMine(comment) ? 'You' : comment.author_name || comment.author?.display_name || 'Unknown'}
                </span>
                <div className="flex items-center gap-2">
                  {comment.group_comment_id && (
                    <span className="inline-flex items-center gap-1 text-xs bg-indigo-100 text-indigo-700 px-1.5 py-0.5 rounded">
                      <Users className="w-3 h-3" /> Group
                    </span>
                  )}
                  {comment.draft && (
                    <span className="text-xs bg-yellow-200 text-accent-warning px-1.5 py-0.5 rounded">Draft</span>
                  )}
                  {comment.attempt ? <span className="text-xs text-text-disabled">Attempt {comment.attempt}</span> : null}
                  <span className="text-xs text-text-disabled">
                    {comment.created_at ? new Date(comment.created_at).toLocaleString() : ''}
                  </span>
                </div>
              </div>
              {editingId === comment.id ? (
                <div className="flex gap-2">
                  <input
                    value={editText}
                    onChange={(e) => setEditText(e.target.value)}
                    aria-label="Edit comment"
                    className="flex-1 border border-border-strong rounded px-2 py-1 text-sm"
                  />
                  <button type="button" onClick={() => saveEdit(comment)} className="text-accent-success" aria-label="Save comment">
                    <Check className="w-4 h-4" />
                  </button>
                  <button type="button" onClick={() => setEditingId(null)} className="text-text-tertiary" aria-label="Cancel edit">
                    <X className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                comment.comment && <p className="text-text-secondary whitespace-pre-wrap">{comment.comment || comment.text_comment}</p>
              )}
              {Array.isArray(comment.attachments) && comment.attachments.length > 0 && (
                <div className="mt-2 space-y-2">
                  {comment.attachments.map((f) => (
                    <AttachmentPreview key={f.id} file={f} height={240} showLink />
                  ))}
                </div>
              )}
              {isMine(comment) && keyId && editingId !== comment.id && (
                <div className="mt-2 flex items-center gap-3 text-xs">
                  {comment.draft && (
                    <button type="button" onClick={() => publishDraft(comment)} className="text-brand-600 hover:underline">
                      Publish
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => { setEditingId(comment.id); setEditText(comment.comment || ''); }}
                    className="inline-flex items-center gap-1 text-text-tertiary hover:text-text-secondary"
                  >
                    <Pencil className="w-3 h-3" /> Edit
                  </button>
                  <button type="button" onClick={() => remove(comment)} className="inline-flex items-center gap-1 text-accent-danger hover:underline">
                    <Trash2 className="w-3 h-3" /> Delete
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      {error && <div className="text-sm text-accent-danger">{error}</div>}

      <form onSubmit={handleSubmit} className="border-t pt-3 space-y-2">
        <div className="flex space-x-2">
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Add a comment..."
            rows={compact ? 2 : 2}
            aria-label="Comment text"
            className="flex-1 border border-border-strong rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500 resize-y"
            disabled={submitting}
          />
          <button
            type="submit"
            disabled={submitting || (!text.trim() && files.length === 0)}
            aria-label="Send comment"
            className="self-end inline-flex items-center space-x-1 bg-brand-600 text-white px-3 py-2 rounded-lg hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>

        {files.length > 0 && (
          <ul className="flex flex-wrap gap-2">
            {files.map((f, i) => (
              <li key={`${f.name}-${i}`} className="inline-flex items-center gap-1 text-xs bg-surface-2 rounded px-2 py-1">
                <Paperclip className="w-3 h-3" />
                <span className="max-w-[12rem] truncate">{f.name}</span>
                <button type="button" onClick={() => setFiles((prev) => prev.filter((_, j) => j !== i))} aria-label={`Remove ${f.name}`}>
                  <X className="w-3 h-3" />
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={submitting || files.length >= MAX_FILES}
            className="inline-flex items-center gap-1 px-2 py-1 rounded border border-border-default text-xs text-text-secondary hover:bg-surface-2 disabled:opacity-50"
          >
            <Paperclip className="w-3 h-3" /> Attach file
          </button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            data-testid="comment-file-input"
            onChange={(e) => { addFiles(e.target.files); e.target.value = ''; }}
          />
          <MediaRecorderButton compact={compact} disabled={submitting} onRecorded={(f) => addFiles([f])} />
          {isGroupAssignment && (
            <label className="inline-flex items-center gap-1 text-xs text-text-secondary">
              <input type="checkbox" checked={toGroup} onChange={(e) => setToGroup(e.target.checked)} className="rounded border-border-strong" />
              Send to the whole group
            </label>
          )}
          {isTeacher && (
            <button
              type="button"
              onClick={() => setIsDraft(!isDraft)}
              className={`inline-flex items-center space-x-1 text-xs px-2 py-1 rounded ${
                isDraft ? 'bg-accent-warning/20 text-accent-warning border border-yellow-300' : 'bg-surface-2 text-text-secondary border border-border-default'
              }`}
            >
              {isDraft ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
              <span>{isDraft ? 'Draft (not visible to student)' : 'Published (visible to student)'}</span>
            </button>
          )}
        </div>
      </form>
    </div>
  );
};

export default CommentThread;
