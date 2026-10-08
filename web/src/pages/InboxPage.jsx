import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  Mail, Send, Plus, MessageSquare, X, Search, ArrowLeft, Loader2, Star, Archive, ArchiveRestore,
  Trash2, Reply, ReplyAll, Forward, Paperclip, MailOpen, Users,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { inboxApi } from '../services/api_notify_inbox';
import {
  useConversations, useUnreadCount, useInboxContexts, useRecipientSearch, useInvalidateInbox, notifyInboxKeys,
} from '../services/queries_notify_inbox';
import {
  INBOX_SCOPES, INBOX_EVENTS, pageFromLink, participantNames, buildConversationPayload, contextRecipientOptions,
  recipientKey, replyTargets, isUnread, applyInboxEvent, rowStaysInScope, formatBytes,
} from '../utils/notifyInbox';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import SubmissionCommentsMailbox from '../components/SubmissionCommentsMailbox';
import { Skeleton } from '@/components/ui/skeleton';

export { pageFromLink };

const MESSAGES_PER_PAGE = 50;

const SCOPE_LABELS = {
  inbox: 'Inbox',
  unread: 'Unread',
  starred: 'Starred',
  sent: 'Sent',
  archived: 'Archived',
};

// Canvas's "Submission Comments" mailbox: not a conversation scope, a
// separate list of submission comment threads.
const SUBMISSION_COMMENTS = 'submission_comments';

// Messages are returned oldest-first, so the newest replies live on the
// LAST page. Load that page (plus the one before it when it is short) so
// a long thread opens on its latest messages; earlier pages are fetched
// on demand.
export const loadLatestMessages = async (conversationId) => {
  const first = await inboxApi.getMessages(conversationId, 1, MESSAGES_PER_PAGE);
  const lastPage = pageFromLink(first.pagination?.last) || 1;
  if (lastPage <= 1) {
    return { items: first.data || [], oldestPage: 1 };
  }
  const last = await inboxApi.getMessages(conversationId, lastPage, MESSAGES_PER_PAGE);
  let items = last.data || [];
  let oldestPage = lastPage;
  if (items.length < MESSAGES_PER_PAGE / 2 && lastPage > 1) {
    const prevPage = lastPage - 1;
    const prev = prevPage === 1 ? first : await inboxApi.getMessages(conversationId, prevPage, MESSAGES_PER_PAGE);
    items = [...(prev.data || []), ...items];
    oldestPage = prevPage;
  }
  return { items, oldestPage };
};

const formatDate = (dateStr) => {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  const diffDays = Math.floor((Date.now() - date) / (1000 * 60 * 60 * 24));
  if (diffDays === 0) return date.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
  if (diffDays < 7) return date.toLocaleDateString(undefined, { weekday: 'short' });
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const iconButton = 'p-1.5 rounded-md text-text-tertiary hover:text-text-primary hover:bg-surface-2 disabled:opacity-50';

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

const AttachmentPicker = ({ attachments, onChange, disabled }) => {
  const inputRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState(null);

  const onFiles = async (files) => {
    setError(null);
    setUploading(true);
    try {
      const uploaded = [];
      for (const f of Array.from(files)) {
        uploaded.push(await inboxApi.uploadAttachment(f));
      }
      onChange([...attachments, ...uploaded]);
    } catch (err) {
      setError(err.message || 'Upload failed');
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = '';
    }
  };

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        {attachments.map((a) => (
          <span key={a.id} className="inline-flex items-center gap-1 bg-surface-2 text-text-secondary text-xs px-2 py-1 rounded-full">
            <Paperclip className="w-3 h-3" aria-hidden="true" />
            {a.display_name || a.filename}
            <button
              type="button"
              onClick={() => onChange(attachments.filter((x) => x.id !== a.id))}
              aria-label={`Remove attachment ${a.display_name || a.filename}`}
              className="text-text-tertiary hover:text-text-primary"
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
        <label className={`inline-flex items-center gap-1 text-xs text-brand-600 hover:text-brand-800 cursor-pointer ${disabled || uploading ? 'opacity-50 pointer-events-none' : ''}`}>
          {uploading ? <Loader2 className="w-3.5 h-3.5 animate-spin" aria-hidden="true" /> : <Paperclip className="w-3.5 h-3.5" aria-hidden="true" />}
          <span>{uploading ? 'Uploading...' : 'Attach file'}</span>
          <input
            ref={inputRef}
            type="file"
            multiple
            className="sr-only"
            aria-label="Attach file"
            onChange={(e) => e.target.files?.length && onFiles(e.target.files)}
          />
        </label>
      </div>
      {error && <p className="text-xs text-accent-danger mt-1" role="alert">{error}</p>}
    </div>
  );
};

// ---------------------------------------------------------------------------
// Recipient picker (address book)
// ---------------------------------------------------------------------------

export const RecipientPicker = ({ value, onChange, selfId, contexts, contextCode }) => {
  const [term, setTerm] = useState('');
  const [debounced, setDebounced] = useState('');
  const [browse, setBrowse] = useState('');
  const [open, setOpen] = useState(false);
  const boxRef = useRef(null);

  useEffect(() => {
    const id = setTimeout(() => setDebounced(term), 300);
    return () => clearTimeout(id);
  }, [term]);

  useEffect(() => {
    const close = (e) => { if (boxRef.current && !boxRef.current.contains(e.target)) setOpen(false); };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, []);

  // Browsing a context lists its members; otherwise search the whole
  // address book (people who share a course or group with the caller).
  const searchContext = browse || '';
  const query = useRecipientSearch({ search: debounced, context: searchContext });
  const chosen = new Set(value.map(recipientKey));
  const people = (query.data || []).filter((u) => u.id !== selfId && !chosen.has(`user:${u.id}`));

  const ctx = (contexts || []).find((c) => c.context_code === contextCode);
  const groupOptions = contextRecipientOptions(ctx).filter((o) => !chosen.has(`ctx:${o.code}`));

  const add = (r) => {
    onChange([...value, r]);
    setTerm('');
  };

  return (
    <div ref={boxRef} className="relative">
      <div className="border border-border-strong rounded-md px-2 py-1.5 focus-within:ring-2 focus-within:ring-brand-500">
        <div className="flex flex-wrap gap-1.5 items-center">
          {value.map((r) => (
            <span key={recipientKey(r)} className="inline-flex items-center gap-1 bg-brand-100 text-brand-800 text-xs font-medium px-2 py-1 rounded-full">
              {r.code && <Users className="w-3 h-3" aria-hidden="true" />}
              {r.name}
              <button
                type="button"
                onClick={() => onChange(value.filter((x) => recipientKey(x) !== recipientKey(r)))}
                className="text-brand-600 hover:text-brand-800"
                aria-label={`Remove ${r.name}`}
              >
                <X className="w-3 h-3" />
              </button>
            </span>
          ))}
          <input
            type="text"
            value={term}
            onChange={(e) => { setTerm(e.target.value); setOpen(true); }}
            onFocus={() => setOpen(true)}
            onKeyDown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); setOpen(false); } }}
            className="flex-1 min-w-[150px] border-0 px-1 py-0.5 text-sm bg-transparent focus:outline-none focus:ring-0"
            placeholder={value.length === 0 ? 'Search by name...' : 'Add more...'}
            aria-label="Search recipients"
          />
        </div>
      </div>
      {contexts && contexts.length > 0 && (
        <div className="mt-1 flex items-center gap-2 text-xs">
          <label htmlFor="inbox-browse" className="text-text-tertiary">Browse</label>
          <select
            id="inbox-browse"
            value={browse}
            onChange={(e) => { setBrowse(e.target.value); setOpen(true); }}
            className="border border-border-default rounded px-1.5 py-0.5 text-xs bg-surface-0"
          >
            <option value="">All courses and groups</option>
            {contexts.map((c) => (
              <option key={c.context_code} value={c.context_code}>{c.name}</option>
            ))}
          </select>
        </div>
      )}
      {open && (groupOptions.length > 0 || people.length > 0 || query.isFetching || query.isError) && (
        <div className="absolute z-20 mt-1 w-full bg-surface-0 border border-border-default rounded-md shadow-lg max-h-60 overflow-y-auto" role="listbox" aria-label="Recipients">
          {groupOptions.map((o) => (
            <button
              key={o.code}
              type="button"
              role="option"
              aria-selected="false"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => add(o)}
              className="w-full text-left px-3 py-2 hover:bg-brand-50 flex items-center gap-2 text-sm"
            >
              <Users className="w-4 h-4 text-text-tertiary" aria-hidden="true" />
              {o.name}
            </button>
          ))}
          {query.isFetching && <div className="px-3 py-2 text-sm text-text-tertiary">Searching...</div>}
          {query.isError && <div className="px-3 py-2 text-sm text-accent-danger" role="alert">{query.error?.message || 'Recipient search failed'}</div>}
          {people.map((u) => (
            <button
              key={u.id}
              type="button"
              role="option"
              aria-selected="false"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => add({ id: u.id, name: u.name })}
              className="w-full text-left px-3 py-2 hover:bg-brand-50 flex items-center gap-2 text-sm"
            >
              <span className="w-7 h-7 rounded-full bg-border-default flex items-center justify-center text-xs font-medium text-text-secondary flex-shrink-0">
                {(u.name || '?')[0].toUpperCase()}
              </span>
              <span className="font-medium text-text-primary truncate">{u.name}</span>
            </button>
          ))}
        </div>
      )}
      {!browse && term.trim().length > 0 && term.trim().length < 2 && (
        <p className="text-xs text-text-tertiary mt-1">Type at least 2 characters to search</p>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------
// Compose (new message / forward)
// ---------------------------------------------------------------------------

const ComposeDialog = ({ mode, source, onClose, onSent, selfId }) => {
  const { t } = useTranslation();
  const contextsQuery = useInboxContexts();
  const contexts = contextsQuery.data || [];
  const [contextCode, setContextCode] = useState('');
  const [recipients, setRecipients] = useState([]);
  const [subject, setSubject] = useState(mode === 'forward' ? `Fwd: ${source?.conversation?.subject || ''}` : '');
  const [body, setBody] = useState('');
  const [attachments, setAttachments] = useState([]);
  const [individual, setIndividual] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState(null);

  const submit = async (e) => {
    e.preventDefault();
    if (recipients.length === 0) return;
    setSending(true);
    setError(null);
    try {
      let result;
      if (mode === 'forward') {
        const payload = buildConversationPayload({ subject, body, recipients });
        result = await inboxApi.forward(source.conversation.id, {
          recipients: payload.recipients,
          body,
          subject: payload.subject,
          message_ids: source.messageIds,
        });
      } else {
        result = await inboxApi.createConversation(buildConversationPayload({
          subject, body, recipients, attachmentIds: attachments.map((a) => a.id), contextCode, individual,
        }));
      }
      onSent(result);
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  };

  const title = mode === 'forward' ? 'Forward' : t('inboxPage.newMessage');

  return (
    <div className="bg-surface-0 rounded-lg shadow p-6 mb-6" role="region" aria-label={title}>
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold">{title}</h3>
        <button type="button" onClick={onClose} className={iconButton} aria-label={t('common.cancel')}>
          <X className="w-4 h-4" />
        </button>
      </div>
      {error && <div className="bg-accent-danger/10 text-accent-danger rounded-md p-3 mb-4 text-sm" role="alert">{error}</div>}
      <form onSubmit={submit} className="space-y-4">
        {mode !== 'forward' && (
          <div>
            <label htmlFor="inbox-context" className="block text-sm font-medium text-text-secondary mb-1">Course or group</label>
            <select
              id="inbox-context"
              value={contextCode}
              onChange={(e) => setContextCode(e.target.value)}
              className="w-full sm:w-80 border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-0"
            >
              <option value="">No specific course or group</option>
              {contexts.map((c) => (
                <option key={c.context_code} value={c.context_code}>{c.name}</option>
              ))}
            </select>
          </div>
        )}
        <div>
          <span className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.to')}</span>
          <RecipientPicker value={recipients} onChange={setRecipients} selfId={selfId} contexts={contexts} contextCode={contextCode} />
        </div>
        {mode !== 'forward' && recipients.length > 1 && (
          <div className="flex items-center gap-2">
            <input id="inbox-individual" type="checkbox" checked={individual} onChange={(e) => setIndividual(e.target.checked)} className="rounded border-border-strong" />
            <label htmlFor="inbox-individual" className="text-sm text-text-secondary">Send an individual message to each recipient</label>
          </div>
        )}
        <div>
          <label htmlFor="inbox-subject" className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.subject')}</label>
          <input
            id="inbox-subject"
            type="text"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            placeholder="Conversation subject"
          />
        </div>
        <div>
          <label htmlFor="inbox-body" className="block text-sm font-medium text-text-secondary mb-1">{t('inboxPage.message')}</label>
          <textarea
            id="inbox-body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            rows={4}
            placeholder={mode === 'forward' ? 'Add a note (optional)' : 'Write your message...'}
            required={mode !== 'forward'}
          />
        </div>
        {mode === 'forward' ? (
          <p className="text-xs text-text-tertiary">
            {source?.messageIds?.length ? 'The selected message and its attachments will be included.' : 'The whole conversation and its attachments will be included.'}
          </p>
        ) : (
          <AttachmentPicker attachments={attachments} onChange={setAttachments} disabled={sending} />
        )}
        <div className="flex justify-end">
          <button
            type="submit"
            disabled={sending || recipients.length === 0}
            className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
          >
            {sending ? t('inboxPage.sending') : t('inboxPage.send')}
          </button>
        </div>
      </form>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Thread
// ---------------------------------------------------------------------------

const MessageBubble = ({ msg, isOwn, participants, onReply, onReplyAll, onForward }) => {
  const nameOf = (id) => participants.find((p) => p.id === id)?.name || `User #${id}`;
  return (
    <div className={`flex ${isOwn ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[80%] rounded-lg px-4 py-2 ${isOwn ? 'bg-brand-600 text-white' : 'bg-surface-2 text-text-primary'}`}>
        <div className="flex items-center justify-between gap-3 mb-1">
          <p className={`text-xs font-medium ${isOwn ? 'text-blue-100' : 'text-text-tertiary'}`}>
            {isOwn ? 'You' : (msg.user_name || `User #${msg.user_id}`)}
            {msg.recipient_ids?.length > 0 && <> &rarr; {msg.recipient_ids.map(nameOf).join(', ')}</>}
          </p>
          <span className="flex items-center gap-0.5">
            {!isOwn && (
              <button type="button" onClick={() => onReply(msg)} className="p-1 rounded hover:bg-black/10" aria-label="Reply" title="Reply">
                <Reply className="w-3.5 h-3.5" />
              </button>
            )}
            <button type="button" onClick={() => onReplyAll(msg)} className="p-1 rounded hover:bg-black/10" aria-label="Reply all" title="Reply all">
              <ReplyAll className="w-3.5 h-3.5" />
            </button>
            <button type="button" onClick={() => onForward(msg)} className="p-1 rounded hover:bg-black/10" aria-label="Forward message" title="Forward">
              <Forward className="w-3.5 h-3.5" />
            </button>
          </span>
        </div>
        <p className="text-sm whitespace-pre-wrap">{msg.body}</p>
        {msg.attachments?.length > 0 && (
          <ul className="mt-2 space-y-1">
            {msg.attachments.map((a) => (
              <li key={a.id}>
                <a
                  href={a.url}
                  target="_blank"
                  rel="noreferrer"
                  className={`inline-flex items-center gap-1 text-xs underline ${isOwn ? 'text-white' : 'text-brand-700'}`}
                >
                  <Paperclip className="w-3 h-3" aria-hidden="true" />
                  {a.display_name || a.filename} ({formatBytes(a.size)})
                </a>
              </li>
            ))}
          </ul>
        )}
        <p className={`text-xs mt-1 ${isOwn ? 'text-blue-200' : 'text-text-disabled'}`}>{formatDate(msg.created_at)}</p>
      </div>
    </div>
  );
};

const Thread = ({ conversation, selfId, onBack, onAction, onForward }) => {
  const queryClient = useQueryClient();
  const invalidateInbox = useInvalidateInbox();
  const [extra, setExtra] = useState({ items: [], oldestPage: null });
  const [loadingEarlier, setLoadingEarlier] = useState(false);
  const [replyTo, setReplyTo] = useState(undefined); // undefined = reply all
  const [replyText, setReplyText] = useState('');
  const [attachments, setAttachments] = useState([]);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState(null);
  const replyRef = useRef(null);

  const messagesQuery = useQuery({
    queryKey: notifyInboxKeys.messages(conversation.id),
    queryFn: () => loadLatestMessages(conversation.id),
  });

  useEffect(() => {
    setExtra({ items: [], oldestPage: null });
    setReplyTo(undefined);
    setReplyText('');
    setAttachments([]);
    setError(null);
  }, [conversation.id]);

  // Opening an unread conversation marks it read.
  useEffect(() => {
    if (!isUnread(conversation)) return;
    inboxApi.markRead(conversation.id).then(invalidateInbox).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conversation.id]);

  const participants = conversation.participants || [];
  const loaded = messagesQuery.data;
  const messages = [...extra.items, ...(loaded?.items || [])];
  const oldestPage = extra.oldestPage ?? loaded?.oldestPage ?? 1;

  const loadEarlier = async () => {
    if (oldestPage <= 1) return;
    setLoadingEarlier(true);
    try {
      const page = oldestPage - 1;
      const { data } = await inboxApi.getMessages(conversation.id, page, MESSAGES_PER_PAGE);
      const seen = new Set(messages.map((m) => m.id));
      setExtra((prev) => ({ items: [...(data || []).filter((m) => !seen.has(m.id)), ...prev.items], oldestPage: page }));
    } catch (err) {
      setError(err.message);
    } finally {
      setLoadingEarlier(false);
    }
  };

  const startReply = (targets) => {
    setReplyTo(targets);
    replyRef.current?.focus();
  };

  const send = async (e) => {
    e.preventDefault();
    if (!replyText.trim()) return;
    setSending(true);
    setError(null);
    try {
      await inboxApi.addMessage(conversation.id, {
        message: replyText,
        attachment_ids: attachments.map((a) => a.id),
        recipients: replyTo,
      });
      setReplyText('');
      setAttachments([]);
      setReplyTo(undefined);
      setExtra({ items: [], oldestPage: null });
      await queryClient.invalidateQueries({ queryKey: notifyInboxKeys.messages(conversation.id) });
      invalidateInbox();
    } catch (err) {
      setError(err.message);
    } finally {
      setSending(false);
    }
  };

  const nameOf = (id) => participants.find((p) => p.id === id)?.name || `User #${id}`;
  const archived = conversation.workflow_state === 'archived';

  return (
    <>
      <div className="p-4 border-b border-border-default bg-surface-1">
        <button onClick={onBack} className="md:hidden flex items-center text-sm text-brand-600 hover:text-brand-800 mb-2">
          <ArrowLeft className="w-4 h-4 mr-1" />
          Back to conversations
        </button>
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <h3 className="font-semibold text-text-primary">{conversation.subject || '(No subject)'}</h3>
            <p className="text-xs text-text-tertiary mt-1">
              {participantNames(conversation, -1)}
              {conversation.context_name && <> &middot; {conversation.context_name}</>}
            </p>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            <button
              type="button"
              className={iconButton}
              onClick={() => onAction([conversation.id], conversation.starred ? INBOX_EVENTS.unstar : INBOX_EVENTS.star)}
              aria-label={conversation.starred ? 'Unstar conversation' : 'Star conversation'}
              aria-pressed={!!conversation.starred}
            >
              <Star className={`w-4 h-4 ${conversation.starred ? 'fill-yellow-400 text-yellow-500' : ''}`} />
            </button>
            <button type="button" className={iconButton} onClick={() => onAction([conversation.id], INBOX_EVENTS.markUnread)} aria-label="Mark as unread">
              <Mail className="w-4 h-4" />
            </button>
            <button
              type="button"
              className={iconButton}
              onClick={() => onAction([conversation.id], archived ? INBOX_EVENTS.unarchive : INBOX_EVENTS.archive)}
              aria-label={archived ? 'Unarchive conversation' : 'Archive conversation'}
            >
              {archived ? <ArchiveRestore className="w-4 h-4" /> : <Archive className="w-4 h-4" />}
            </button>
            <button type="button" className={iconButton} onClick={() => onForward({ conversation })} aria-label="Forward conversation">
              <Forward className="w-4 h-4" />
            </button>
            <button
              type="button"
              className={`${iconButton} hover:text-accent-danger`}
              onClick={() => {
                if (window.confirm('Delete this conversation from your inbox? The other participants keep their copy.')) {
                  onAction([conversation.id], INBOX_EVENTS.destroy);
                }
              }}
              aria-label="Delete conversation"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {error && <div className="bg-accent-danger/10 text-accent-danger rounded-md p-2 text-sm" role="alert">{error}</div>}
        {messagesQuery.isLoading ? (
          <div className="flex items-center justify-center text-text-tertiary py-8 text-sm">
            <Loader2 className="w-5 h-5 animate-spin mr-2" />
            <span>Loading messages...</span>
          </div>
        ) : messagesQuery.isError ? (
          <div className="text-center text-accent-danger py-8 text-sm" role="alert">{messagesQuery.error?.message}</div>
        ) : messages.length === 0 ? (
          <div className="text-center text-text-disabled py-8 text-sm">No messages yet. Start the conversation!</div>
        ) : (
          <>
            {oldestPage > 1 && (
              <div className="text-center">
                <button type="button" onClick={loadEarlier} disabled={loadingEarlier} className="text-sm text-brand-600 hover:text-brand-800 font-medium disabled:opacity-50">
                  {loadingEarlier ? 'Loading...' : 'Load earlier messages'}
                </button>
              </div>
            )}
            {messages.map((msg) => (
              <MessageBubble
                key={msg.id}
                msg={msg}
                isOwn={msg.user_id === selfId}
                participants={participants}
                onReply={(m) => startReply(replyTargets(m, selfId))}
                onReplyAll={() => startReply(undefined)}
                onForward={(m) => onForward({ conversation, messageIds: [m.id] })}
              />
            ))}
          </>
        )}
      </div>

      <div className="border-t border-border-default p-4">
        <form onSubmit={send} className="space-y-2">
          <div className="flex items-center justify-between text-xs text-text-tertiary">
            <span>
              {replyTo ? <>Replying to {replyTo.map(nameOf).join(', ')}</> : <>Replying to everyone</>}
            </span>
            {replyTo && (
              <button type="button" onClick={() => setReplyTo(undefined)} className="text-brand-600 hover:text-brand-800">
                Reply all instead
              </button>
            )}
          </div>
          <div className="flex items-end space-x-2">
            <textarea
              ref={replyRef}
              value={replyText}
              onChange={(e) => setReplyText(e.target.value)}
              placeholder="Type a message..."
              aria-label="Reply message"
              className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
              rows={2}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  send(e);
                }
              }}
            />
            <button type="submit" disabled={sending || !replyText.trim()} className="bg-brand-600 text-white p-2 rounded-md hover:bg-brand-700 disabled:opacity-50" aria-label="Send message" title="Send message">
              <Send className="w-5 h-5" />
            </button>
          </div>
          <AttachmentPicker attachments={attachments} onChange={setAttachments} disabled={sending} />
        </form>
      </div>
    </>
  );
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

const InboxPage = () => {
  const { t } = useTranslation();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const invalidateInbox = useInvalidateInbox();
  const [scope, setScope] = useState('inbox');
  const [filter, setFilter] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [selectedId, setSelectedId] = useState(null);
  const [checked, setChecked] = useState(() => new Set());
  const [compose, setCompose] = useState(null); // {mode: 'new'|'forward', source}
  const [actionError, setActionError] = useState(null);

  useEffect(() => {
    const id = setTimeout(() => setSearch(searchInput.trim()), 300);
    return () => clearTimeout(id);
  }, [searchInput]);

  const submissionScope = scope === SUBMISSION_COMMENTS;
  const listQuery = useConversations({ scope, filter, search }, { enabled: !submissionScope });
  const unreadQuery = useUnreadCount();
  const contextsQuery = useInboxContexts();

  const conversations = useMemo(() => {
    const seen = new Set();
    const out = [];
    (listQuery.data?.pages || []).forEach((p) => (p.data || []).forEach((c) => {
      if (!seen.has(c.id)) { seen.add(c.id); out.push(c); }
    }));
    return out;
  }, [listQuery.data]);

  const selected = conversations.find((c) => c.id === selectedId) || null;

  useEffect(() => { setChecked(new Set()); }, [scope, filter, search]);

  const runAction = async (ids, event) => {
    setActionError(null);
    // Optimistic update of every cached list, then refetch.
    queryClient.setQueriesData({ queryKey: ['inbox', 'conversations'] }, (old) => {
      if (!old?.pages) return old;
      return {
        ...old,
        pages: old.pages.map((p) => ({ ...p, data: applyInboxEvent(p.data, ids, event) })),
      };
    });
    try {
      await inboxApi.batchUpdate(ids, event);
      if (event === INBOX_EVENTS.destroy && ids.includes(selectedId)) setSelectedId(null);
      setChecked(new Set());
    } catch (err) {
      setActionError(err.message);
    } finally {
      invalidateInbox();
    }
  };

  const toggleChecked = (id) => setChecked((prev) => {
    const next = new Set(prev);
    if (next.has(id)) next.delete(id); else next.add(id);
    return next;
  });

  const checkedIds = [...checked];
  const visible = conversations.filter((c) => rowStaysInScope(c, scope));

  const onSent = (result) => {
    setCompose(null);
    invalidateInbox();
    if (result?.id) setSelectedId(result.id);
    if (scope !== 'inbox' && scope !== 'sent') setScope('inbox');
  };

  if (listQuery.isLoading && !listQuery.data) {
    return (
      <Layout>
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      </Layout>
    );
  }

  const unread = unreadQuery.data || 0;

  return (
    <Layout>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
          <Mail className="w-6 h-6" />
          {t('inboxPage.title')}
          {unread > 0 && (
            <span className="ml-1 inline-flex items-center justify-center rounded-full bg-brand-600 text-white text-xs font-semibold px-2 py-0.5" aria-label={`${unread} unread conversations`}>
              {unread}
            </span>
          )}
        </h2>
        <button
          onClick={() => setCompose(compose ? null : { mode: 'new' })}
          className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
          aria-label={t('inboxPage.compose')}
        >
          {compose ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          <span>{compose ? t('common.cancel') : t('inboxPage.newMessage')}</span>
        </button>
      </div>

      {(listQuery.isError || actionError) && (
        <div className="bg-accent-danger/10 text-accent-danger rounded-md p-3 mb-4 text-sm flex items-center justify-between" role="alert">
          <span>{actionError || listQuery.error?.message}</span>
          <button onClick={() => { setActionError(null); listQuery.refetch(); }} className="ml-3 font-medium underline text-sm flex-shrink-0">
            Retry
          </button>
        </div>
      )}

      {compose && (
        <ComposeDialog
          key={`${compose.mode}-${compose.source?.conversation?.id || 'new'}-${(compose.source?.messageIds || []).join(',')}`}
          mode={compose.mode}
          source={compose.source}
          selfId={user?.id}
          onClose={() => setCompose(null)}
          onSent={onSent}
        />
      )}

      {/* Toolbar: scope, course/group filter, search */}
      <div className="flex flex-col lg:flex-row lg:items-center gap-3 mb-3">
        <div className="flex flex-wrap gap-1" role="tablist" aria-label="Mailbox">
          {INBOX_SCOPES.map((s) => (
            <button
              key={s}
              type="button"
              role="tab"
              aria-selected={scope === s}
              onClick={() => { setScope(s); setSelectedId(null); }}
              className={`px-3 py-1.5 rounded-md text-sm font-medium ${scope === s ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-2'}`}
            >
              {SCOPE_LABELS[s]}
              {s === 'unread' && unread > 0 ? ` (${unread})` : ''}
            </button>
          ))}
          <button
            type="button"
            role="tab"
            aria-selected={submissionScope}
            onClick={() => { setScope(SUBMISSION_COMMENTS); setSelectedId(null); }}
            className={`px-3 py-1.5 rounded-md text-sm font-medium ${submissionScope ? 'bg-brand-600 text-white' : 'bg-surface-0 text-text-secondary hover:bg-surface-2'}`}
          >
            Submission comments
          </button>
        </div>
        <div className="flex flex-1 gap-2">
          <select
            value={filter}
            onChange={(e) => { setFilter(e.target.value); setSelectedId(null); }}
            className="border border-border-strong rounded-md px-2 py-1.5 text-sm bg-surface-0"
            aria-label="Filter by course or group"
          >
            <option value="">All courses and groups</option>
            {(contextsQuery.data || []).map((c) => (
              <option key={c.context_code} value={c.context_code}>{c.name}</option>
            ))}
          </select>
          <div className={`relative flex-1 ${submissionScope ? 'invisible' : ''}`}>
            <Search className="w-4 h-4 absolute left-2 top-2 text-text-tertiary" aria-hidden="true" />
            <input
              type="search"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search subjects"
              aria-label="Search conversations"
              className="w-full border border-border-strong rounded-md pl-8 pr-2 py-1.5 text-sm"
            />
          </div>
        </div>
      </div>

      {checkedIds.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 mb-3 bg-surface-1 border border-border-default rounded-md px-3 py-2 text-sm" role="toolbar" aria-label="Selected conversations">
          <span className="text-text-secondary">{checkedIds.length} selected</span>
          <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.markRead)}><MailOpen className="w-4 h-4" />Mark read</button>
          <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.markUnread)}><Mail className="w-4 h-4" />Mark unread</button>
          <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.star)}><Star className="w-4 h-4" />Star</button>
          <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.unstar)}>Unstar</button>
          {scope === 'archived' ? (
            <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.unarchive)}><ArchiveRestore className="w-4 h-4" />Unarchive</button>
          ) : (
            <button type="button" className="inline-flex items-center gap-1 text-brand-700 hover:underline" onClick={() => runAction(checkedIds, INBOX_EVENTS.archive)}><Archive className="w-4 h-4" />Archive</button>
          )}
          <button
            type="button"
            className="inline-flex items-center gap-1 text-accent-danger hover:underline"
            onClick={() => {
              if (window.confirm(`Delete ${checkedIds.length} conversation(s) from your inbox?`)) runAction(checkedIds, INBOX_EVENTS.destroy);
            }}
          >
            <Trash2 className="w-4 h-4" />Delete
          </button>
        </div>
      )}

      {submissionScope ? (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <SubmissionCommentsMailbox filter={filter} />
        </div>
      ) : (
      <div className="flex flex-col md:flex-row bg-surface-0 rounded-lg shadow overflow-hidden" style={{ minHeight: '500px' }}>
        <div className={`w-full md:w-2/5 border-r border-border-default overflow-y-auto ${selected ? 'hidden md:block' : 'block'}`}>
          {visible.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary text-sm">
              <MessageSquare className="w-8 h-8 mx-auto mb-2 text-gray-300" />
              {scope === 'inbox' && !filter && !search ? 'No conversations yet.' : 'No conversations match.'}
            </div>
          ) : (
            <ul className="divide-y divide-gray-100" aria-label="Conversations">
              {visible.map((conv) => {
                const unreadRow = isUnread(conv);
                return (
                  <li key={conv.id} className={`flex items-start gap-2 p-3 ${selected?.id === conv.id ? 'bg-brand-50 border-l-2 border-brand-600' : 'hover:bg-surface-1'}`}>
                    <input
                      type="checkbox"
                      className="mt-1 rounded border-border-strong"
                      checked={checked.has(conv.id)}
                      onChange={() => toggleChecked(conv.id)}
                      aria-label={`Select ${conv.subject || 'conversation'}`}
                    />
                    <button type="button" onClick={() => setSelectedId(conv.id)} className="flex-1 min-w-0 text-left">
                      <div className="flex items-start justify-between gap-2">
                        <p className={`text-sm truncate ${unreadRow ? 'font-bold text-text-primary' : 'font-medium text-text-primary'}`}>
                          {unreadRow && <span className="inline-block w-2 h-2 rounded-full bg-brand-600 mr-1.5 align-middle" aria-label="Unread" />}
                          {participantNames(conv, user?.id)}
                        </p>
                        <span className="text-xs text-text-disabled flex-shrink-0">{formatDate(conv.last_message_at)}</span>
                      </div>
                      <p className={`text-sm truncate ${unreadRow ? 'font-semibold' : ''} text-text-secondary`}>
                        {conv.subject || '(No subject)'}
                        {conv.message_count > 1 && <span className="text-text-tertiary font-normal"> ({conv.message_count})</span>}
                      </p>
                      {conv.last_message && <p className="text-xs text-text-tertiary truncate mt-0.5">{conv.last_message}</p>}
                      <div className="flex items-center gap-2 mt-1">
                        {conv.context_name && <span className="text-xs text-text-tertiary truncate">{conv.context_name}</span>}
                        {conv.has_attachments && <Paperclip className="w-3 h-3 text-text-tertiary" aria-label="Has attachments" />}
                      </div>
                    </button>
                    <button
                      type="button"
                      onClick={() => runAction([conv.id], conv.starred ? INBOX_EVENTS.unstar : INBOX_EVENTS.star)}
                      className="p-1 text-text-tertiary hover:text-yellow-500"
                      aria-label={conv.starred ? `Unstar ${conv.subject || 'conversation'}` : `Star ${conv.subject || 'conversation'}`}
                      aria-pressed={!!conv.starred}
                    >
                      <Star className={`w-4 h-4 ${conv.starred ? 'fill-yellow-400 text-yellow-500' : ''}`} />
                    </button>
                  </li>
                );
              })}
              {listQuery.hasNextPage && (
                <li className="p-3 text-center">
                  <button
                    type="button"
                    onClick={() => listQuery.fetchNextPage()}
                    disabled={listQuery.isFetchingNextPage}
                    className="text-sm text-brand-600 hover:text-brand-800 font-medium disabled:opacity-50"
                  >
                    {listQuery.isFetchingNextPage ? 'Loading...' : 'Load more conversations'}
                  </button>
                </li>
              )}
            </ul>
          )}
        </div>

        <div className={`flex-1 ${!selected ? 'hidden md:flex' : 'flex'} flex-col`}>
          {selected ? (
            <Thread
              conversation={selected}
              selfId={user?.id}
              onBack={() => setSelectedId(null)}
              onAction={runAction}
              onForward={(source) => setCompose({ mode: 'forward', source })}
            />
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-text-disabled">
              <Mail className="w-12 h-12 mb-3" />
              <p className="text-sm">Select a conversation to view messages</p>
            </div>
          )}
        </div>
      </div>
      )}
    </Layout>
  );
};

export default InboxPage;
