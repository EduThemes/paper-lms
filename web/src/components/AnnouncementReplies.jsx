// Replies on an announcement (G090). Readers reply when the announcement
// allows comments; the course's instructors can always reply and remove
// replies; authors edit and delete their own. The panel (and its queries)
// only mounts when expanded, so a collapsed list costs no requests.

import React, { useState } from 'react';
import { MessageCircle, ChevronDown, ChevronUp, CornerDownRight, Edit2, Trash2 } from 'lucide-react';
import RichContentViewer from './RichContentViewer';
import {
  useAnnouncementReplies, useCreateAnnouncementReply, useUpdateAnnouncementReply, useDeleteAnnouncementReply,
} from '../services/queries_notify_inbox';
import { textToHtml, threadReplies } from '../utils/notifyInbox';

const formatDate = (s) => (s ? new Date(s).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : '');

// Edits load the stored HTML back into a textarea as text.
const htmlToText = (html) => {
  const div = document.createElement('div');
  div.innerHTML = String(html || '').replace(/<br\s*\/?>/gi, '\n').replace(/<\/p>\s*<p>/gi, '\n\n');
  return div.textContent || '';
};

const ReplyForm = ({ initial = '', submitLabel, onSubmit, onCancel, pending, autoFocus, label }) => {
  const [text, setText] = useState(initial);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!text.trim()) return;
        onSubmit(textToHtml(text), () => setText(''));
      }}
      className="space-y-2"
    >
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={3}
        autoFocus={autoFocus}
        aria-label={label}
        className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
        placeholder="Write a reply..."
      />
      <div className="flex justify-end gap-2">
        {onCancel && (
          <button type="button" onClick={onCancel} className="px-3 py-1.5 text-sm border border-border-strong rounded-md hover:bg-surface-1">
            Cancel
          </button>
        )}
        <button type="submit" disabled={pending || !text.trim()} className="px-3 py-1.5 text-sm bg-brand-600 text-white rounded-md hover:bg-brand-700 disabled:opacity-50">
          {pending ? 'Saving...' : submitLabel}
        </button>
      </div>
    </form>
  );
};

const ReplyItem = ({ reply, depth, ctx }) => {
  const { announcementId, currentUserId, canModerate, canReply } = ctx;
  const [mode, setMode] = useState(null); // 'reply' | 'edit'
  const create = useCreateAnnouncementReply(announcementId, () => setMode(null));
  const update = useUpdateAnnouncementReply(announcementId, () => setMode(null));
  const remove = useDeleteAnnouncementReply(announcementId);
  const own = reply.user_id === currentUserId;

  return (
    <li className={depth > 0 ? 'ml-6 border-l border-border-default pl-3' : ''}>
      <div className="py-2">
        <div className="flex items-center justify-between gap-2">
          <p className="text-xs font-medium text-text-secondary">
            {reply.user_name || `User #${reply.user_id}`} <span className="text-text-disabled font-normal">&middot; {formatDate(reply.created_at)}</span>
          </p>
          <div className="flex items-center gap-1">
            {canReply && depth < 3 && (
              <button type="button" onClick={() => setMode(mode === 'reply' ? null : 'reply')} className="p-1 text-text-tertiary hover:text-text-primary" aria-label={`Reply to ${reply.user_name || 'reply'}`}>
                <CornerDownRight className="w-3.5 h-3.5" />
              </button>
            )}
            {own && (
              <button type="button" onClick={() => setMode(mode === 'edit' ? null : 'edit')} className="p-1 text-text-tertiary hover:text-text-primary" aria-label="Edit reply">
                <Edit2 className="w-3.5 h-3.5" />
              </button>
            )}
            {(own || canModerate) && (
              <button
                type="button"
                onClick={() => { if (window.confirm('Delete this reply?')) remove.mutate(reply.id); }}
                className="p-1 text-text-tertiary hover:text-accent-danger"
                aria-label="Delete reply"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </div>
        {mode === 'edit' ? (
          <ReplyForm
            initial={htmlToText(reply.message)}
            submitLabel="Save"
            label="Edit reply"
            autoFocus
            pending={update.isPending}
            onCancel={() => setMode(null)}
            onSubmit={(html) => update.mutate({ replyId: reply.id, message: html })}
          />
        ) : (
          <RichContentViewer content={reply.message} className="prose-sm text-sm text-text-primary" />
        )}
        {(update.isError || remove.isError || create.isError) && (
          <p className="text-xs text-accent-danger" role="alert">{(update.error || remove.error || create.error)?.message}</p>
        )}
        {mode === 'reply' && (
          <div className="mt-2">
            <ReplyForm
              submitLabel="Reply"
              label="Reply to this reply"
              autoFocus
              pending={create.isPending}
              onCancel={() => setMode(null)}
              onSubmit={(html) => create.mutate({ message: html, parentId: reply.id })}
            />
          </div>
        )}
      </div>
      {reply.children.length > 0 && (
        <ul>
          {reply.children.map((child) => <ReplyItem key={child.id} reply={child} depth={depth + 1} ctx={ctx} />)}
        </ul>
      )}
    </li>
  );
};

const RepliesPanel = ({ announcementId, currentUserId, canModerate, canReply }) => {
  const query = useAnnouncementReplies(announcementId);
  const create = useCreateAnnouncementReply(announcementId);
  const ctx = { announcementId, currentUserId, canModerate, canReply };

  if (query.isLoading) return <p className="text-sm text-text-tertiary py-2">Loading replies...</p>;
  if (query.isError) return <p className="text-sm text-accent-danger py-2" role="alert">{query.error?.message}</p>;
  const tree = threadReplies(query.data);

  return (
    <div className="space-y-2">
      {tree.length === 0 ? (
        <p className="text-sm text-text-tertiary">No replies yet.</p>
      ) : (
        <ul className="divide-y divide-border-default">
          {tree.map((r) => <ReplyItem key={r.id} reply={r} depth={0} ctx={ctx} />)}
        </ul>
      )}
      {canReply && (
        <>
          <ReplyForm
            submitLabel="Reply"
            label="Write a reply"
            pending={create.isPending}
            onSubmit={(html, reset) => create.mutate({ message: html }, { onSuccess: reset })}
          />
          {create.isError && <p className="text-xs text-accent-danger" role="alert">{create.error?.message}</p>}
        </>
      )}
    </div>
  );
};

const AnnouncementReplies = ({ announcement, currentUserId, canModerate }) => {
  const [open, setOpen] = useState(false);
  const canReply = !!announcement.allow_comments || !!canModerate;
  const count = announcement.reply_count || 0;
  if (!canReply && count === 0) return null;
  const panelId = `replies-${announcement.id}`;

  return (
    <div className="mt-3 pt-3 border-t border-border-default">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="inline-flex items-center gap-1 text-sm text-brand-600 hover:text-brand-800"
        aria-expanded={open}
        aria-controls={panelId}
      >
        <MessageCircle className="w-4 h-4" aria-hidden="true" />
        <span>{count === 1 ? '1 reply' : `${count} replies`}</span>
        {open ? <ChevronUp className="w-4 h-4" aria-hidden="true" /> : <ChevronDown className="w-4 h-4" aria-hidden="true" />}
      </button>
      {open && (
        <div id={panelId} className="mt-2">
          <RepliesPanel announcementId={announcement.id} currentUserId={currentUserId} canModerate={canModerate} canReply={canReply} />
        </div>
      )}
    </div>
  );
};

export default AnnouncementReplies;
