import React, { useEffect, useRef, useState } from 'react';
import { AtSign, Loader2, Paperclip, Send, X } from 'lucide-react';
import RichContentEditorV2 from '../rce/RichContentEditorV2';
import { clearRceDraft } from '../rce/rceDrafts';
import { discussionsApi } from '../../services/api_discussions';
import {
  appendMention,
  formatFileSize,
  isBlankMessage,
  mentionedUserIds,
} from '../../utils/discussionMentions';

/**
 * Reply / post composer for discussions: the shared rich content editor
 * (images, media embeds, equations, links), a file attachment, an
 * @mention picker that notifies the people named, and — in a partially
 * anonymous topic — a "post anonymously" choice.
 *
 * onSubmit receives `{ message, attachment_id?, anonymous?,
 * mentioned_user_ids? }` and may throw; the composer then keeps its
 * content and shows the error.
 */
export default function DiscussionComposer({
  courseId,
  topicId,
  onSubmit,
  onCancel,
  initialValue = '',
  submitLabel = 'Post',
  placeholder = 'Write your reply...',
  allowAttachment = true,
  allowAnonymous = false,
  draftKey,
}) {
  const [message, setMessage] = useState(initialValue);
  const [attachment, setAttachment] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [anonymous, setAnonymous] = useState(false);
  const [picked, setPicked] = useState([]);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [candidates, setCandidates] = useState([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const fileRef = useRef(null);

  useEffect(() => {
    if (!pickerOpen) return undefined;
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const users = await discussionsApi.mentionableUsers(courseId, topicId, search);
        if (!cancelled) setCandidates(Array.isArray(users) ? users : []);
      } catch {
        if (!cancelled) setCandidates([]);
      }
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [pickerOpen, search, courseId, topicId]);

  const handleFile = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      const uploaded = await discussionsApi.uploadAttachment(courseId, topicId, file);
      setAttachment(uploaded);
    } catch (err) {
      setError(err.message || 'Could not upload the file');
    } finally {
      setUploading(false);
    }
  };

  const pick = (user) => {
    setMessage((prev) => appendMention(prev, user.name));
    setPicked((prev) => (prev.some((u) => u.id === user.id) ? prev : [...prev, user]));
    setPickerOpen(false);
    setSearch('');
  };

  const blank = isBlankMessage(message) && !attachment;

  const handleSubmit = async () => {
    if (blank || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      const body = { message };
      if (attachment) body.attachment_id = attachment.id;
      if (allowAnonymous && anonymous) body.anonymous = true;
      const ids = mentionedUserIds(message, picked);
      if (ids.length > 0) body.mentioned_user_ids = ids;
      await onSubmit(body);
      setMessage('');
      setAttachment(null);
      setPicked([]);
      setAnonymous(false);
      if (draftKey) clearRceDraft(draftKey);
    } catch (err) {
      setError(err?.message || 'Could not post');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="border border-border-strong rounded-lg bg-surface-0" data-testid="discussion-composer">
      <RichContentEditorV2
        value={message}
        onChange={setMessage}
        placeholder={placeholder}
        courseId={courseId}
        autoSaveKey={draftKey}
      />

      {attachment && (
        <div className="flex items-center gap-2 mx-3 my-2 px-2 py-1 rounded bg-surface-1 text-xs text-text-secondary w-fit">
          <Paperclip className="w-3.5 h-3.5" aria-hidden="true" />
          <span>{attachment.display_name}</span>
          <span className="text-text-tertiary">{formatFileSize(attachment.size)}</span>
          <button
            type="button"
            onClick={() => setAttachment(null)}
            className="p-0.5 rounded hover:bg-surface-2"
            aria-label={`Remove ${attachment.display_name}`}
          >
            <X className="w-3 h-3" />
          </button>
        </div>
      )}

      {pickerOpen && (
        <div className="mx-3 my-2 border border-border-default rounded-md p-2 bg-surface-0" role="dialog" aria-label="Mention someone">
          <input
            type="search"
            autoFocus
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search people…"
            aria-label="Search people to mention"
            className="w-full border border-border-strong rounded px-2 py-1 text-sm"
          />
          <ul className="mt-1 max-h-40 overflow-y-auto">
            {candidates.length === 0 && <li className="px-2 py-1 text-xs text-text-tertiary">No one found</li>}
            {candidates.map((u) => (
              <li key={u.id}>
                <button
                  type="button"
                  onClick={() => pick(u)}
                  className="w-full text-left px-2 py-1 text-sm rounded hover:bg-surface-1"
                >
                  {u.name}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {error && <p className="mx-3 my-1 text-xs text-accent-danger" role="alert">{error}</p>}

      <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 border-t border-border-default bg-surface-1 rounded-b-lg">
        <div className="flex items-center gap-1">
          {allowAttachment && (
            <>
              <input ref={fileRef} type="file" className="hidden" onChange={handleFile} data-testid="discussion-attach-input" />
              <button
                type="button"
                onClick={() => fileRef.current?.click()}
                disabled={uploading || !!attachment}
                className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-text-secondary hover:bg-surface-2 disabled:opacity-50"
                aria-label="Attach a file"
              >
                {uploading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Paperclip className="w-3.5 h-3.5" />}
                Attach
              </button>
            </>
          )}
          <button
            type="button"
            onClick={() => setPickerOpen((v) => !v)}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs text-text-secondary hover:bg-surface-2"
            aria-label="Mention someone"
            aria-expanded={pickerOpen}
          >
            <AtSign className="w-3.5 h-3.5" />
            Mention
          </button>
          {allowAnonymous && (
            <label className="inline-flex items-center gap-1 ml-2 text-xs text-text-secondary">
              <input type="checkbox" checked={anonymous} onChange={(e) => setAnonymous(e.target.checked)} />
              Post anonymously
            </label>
          )}
        </div>
        <div className="flex items-center gap-2">
          {onCancel && (
            <button
              type="button"
              onClick={onCancel}
              className="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary rounded-md hover:bg-surface-2"
            >
              Cancel
            </button>
          )}
          <button
            type="button"
            onClick={handleSubmit}
            disabled={blank || submitting || uploading}
            className="inline-flex items-center gap-1.5 px-4 py-1.5 bg-brand-600 text-white text-sm font-medium rounded-md hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {submitting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
            {submitLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
