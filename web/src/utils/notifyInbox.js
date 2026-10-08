// Pure helpers for the inbox, announcement replies and the notification
// preference grid (notify-inbox lane). Kept free of React so they are unit
// tested directly.

export const INBOX_SCOPES = ['inbox', 'unread', 'starred', 'sent', 'archived'];

// Batch events accepted by PUT /conversations.
export const INBOX_EVENTS = {
  markRead: 'mark_as_read',
  markUnread: 'mark_as_unread',
  star: 'star',
  unstar: 'unstar',
  archive: 'archive',
  unarchive: 'unarchive',
  destroy: 'destroy',
};

// Page number of a Link-header URL (e.g. rel="last" / rel="next").
export const pageFromLink = (url) => {
  if (!url) return null;
  const m = String(url).match(/[?&]page=(\d+)/);
  return m ? parseInt(m[1], 10) : null;
};

// Names of the other participants of a conversation ("Me" when alone).
export const participantNames = (conv, selfId) => {
  const others = (conv?.participants || []).filter((p) => p.id !== selfId);
  if (others.length === 0) return 'Me';
  return others.map((p) => p.name || `User #${p.id}`).join(', ');
};

// A recipient chip is either a person ({id, name}) or a whole context
// ({code: "course_5_students", name: "All students"}). The API takes user
// ids and context codes in one array.
export const recipientKey = (r) => (r.code ? `ctx:${r.code}` : `user:${r.id}`);

export const buildConversationPayload = ({
  subject = '',
  body = '',
  recipients = [],
  attachmentIds = [],
  contextCode = '',
  individual = false,
}) => {
  const payload = {
    subject: subject.trim(),
    body,
    recipients: recipients.map((r) => (r.code ? r.code : r.id)),
  };
  if (attachmentIds.length > 0) payload.attachment_ids = attachmentIds;
  if (contextCode) payload.context_code = contextCode;
  // Canvas "Send an individual message to each recipient".
  if (individual) payload.group_conversation = false;
  return payload;
};

// The context recipients an instructor (or admin) can address in one go.
export const contextRecipientOptions = (ctx) => {
  if (!ctx || !ctx.permissions?.send_messages_all) return [];
  const code = ctx.context_code;
  if (ctx.context_type === 'Group') {
    return [{ code, name: `Everyone in ${ctx.name}` }];
  }
  return [
    { code, name: `Everyone in ${ctx.name}` },
    { code: `${code}_students`, name: `All students in ${ctx.name}` },
    { code: `${code}_teachers`, name: `All teachers in ${ctx.name}` },
    { code: `${code}_tas`, name: `All TAs in ${ctx.name}` },
    { code: `${code}_observers`, name: `All observers in ${ctx.name}` },
  ];
};

// "Reply" to one message: its author — or, for the caller's own message,
// the people that message went to (undefined = everyone, i.e. reply all).
export const replyTargets = (message, selfId) => {
  if (!message) return undefined;
  if (message.user_id !== selfId) return [message.user_id];
  return message.recipient_ids && message.recipient_ids.length > 0 ? message.recipient_ids : undefined;
};

export const isUnread = (conv) => conv?.workflow_state === 'unread';

// Optimistically apply a batch event to cached conversation rows.
export const applyInboxEvent = (rows, ids, event) => {
  const set = new Set(ids);
  return (rows || []).map((c) => {
    if (!set.has(c.id)) return c;
    switch (event) {
      case INBOX_EVENTS.markRead:
        return { ...c, workflow_state: c.workflow_state === 'archived' ? 'archived' : 'read' };
      case INBOX_EVENTS.markUnread:
        return { ...c, workflow_state: 'unread' };
      case INBOX_EVENTS.star:
        return { ...c, starred: true };
      case INBOX_EVENTS.unstar:
        return { ...c, starred: false };
      case INBOX_EVENTS.archive:
        return { ...c, workflow_state: 'archived' };
      case INBOX_EVENTS.unarchive:
        return { ...c, workflow_state: c.workflow_state === 'archived' ? 'read' : c.workflow_state };
      default:
        return c;
    }
  });
};

// Whether a row still belongs in the list after an event (archiving drops
// it from the inbox, unarchiving drops it from the archive, ...).
export const rowStaysInScope = (conv, scope) => {
  switch (scope) {
    case 'unread':
      return conv.workflow_state === 'unread';
    case 'starred':
      return !!conv.starred;
    case 'archived':
      return conv.workflow_state === 'archived';
    case 'sent':
      return true;
    default:
      return conv.workflow_state !== 'archived';
  }
};

export const formatBytes = (n) => {
  const size = Number(n) || 0;
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
};

// Plain text typed into a textarea -> safe HTML paragraphs for the reply
// API (the server sanitizes again; this keeps line breaks).
export const textToHtml = (text) => {
  const escape = (s) => s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
  return String(text || '')
    .trim()
    .split(/\n{2,}/)
    .filter((p) => p.trim() !== '')
    .map((p) => `<p>${escape(p).replace(/\n/g, '<br>')}</p>`)
    .join('');
};

// Human audience label for an announcement.
export const audienceLabel = (announcement, sectionNames = {}) => {
  const target = announcement?.target_audience || 'all';
  if (target === 'sections') {
    const ids = announcement.section_ids || [];
    if (ids.length === 0) return 'Specific sections';
    return `Sections: ${ids.map((id) => sectionNames[id] || `Section #${id}`).join(', ')}`;
  }
  if (target.startsWith('section:')) {
    const id = Number(target.slice('section:'.length));
    return `Section: ${sectionNames[id] || `Section #${id}`}`;
  }
  return {
    all: 'Everyone',
    students: 'Students',
    teachers: 'Teachers',
    observers: 'Observers',
  }[target] || target;
};

// Announcement form validation: section targeting needs a section, and a
// scheduled post needs a date in the future.
export const validateAnnouncementForm = (form, now = new Date()) => {
  if (!form.title || !form.title.trim()) return 'Title is required';
  if (form.target_audience === 'sections' && (!form.section_ids || form.section_ids.length === 0)) {
    return 'Choose at least one section';
  }
  if (form.schedule) {
    if (!form.delayed_post_at) return 'Choose when to post the announcement';
    const at = new Date(form.delayed_post_at);
    if (Number.isNaN(at.getTime()) || at <= now) return 'The scheduled time must be in the future';
  }
  return null;
};

// Threads flat replies (parent_id) into a tree, oldest first.
export const threadReplies = (replies) => {
  const byId = new Map();
  (replies || []).forEach((r) => byId.set(r.id, { ...r, children: [] }));
  const roots = [];
  byId.forEach((r) => {
    const parent = r.parent_id ? byId.get(r.parent_id) : null;
    if (parent) parent.children.push(r);
    else roots.push(r);
  });
  return roots;
};

// Where a submission comment thread is read and answered: the student's
// own assignment page, or SpeedGrader for the course's teachers.
export const submissionThreadLink = (thread) => {
  const base = `/courses/${thread.course_id}/assignments/${thread.assignment_id}`;
  return thread.caller_is_student ? base : `${base}/speedgrader`;
};

// The inbox course filter ("course_5") applied to submission comment
// threads; group filters match none (submissions belong to courses).
export const threadsInContext = (threads, filter) => {
  if (!filter) return threads || [];
  return (threads || []).filter((t) => `course_${t.course_id}` === filter);
};

// The cell of the notification grid for one channel and category.
export const policyCell = (grid, channelId, category) =>
  (grid?.policies || []).find((p) => p.channel_id === channelId && p.category === category);

export const channelLabel = (ch) => {
  if (!ch) return '';
  if (ch.primary) return `Email (account): ${ch.address}`;
  const type = { email: 'Email', webhook: 'Webhook', sms: 'SMS', push: 'Push' }[ch.channel_type] || ch.channel_type;
  return `${type}: ${ch.address}`;
};
