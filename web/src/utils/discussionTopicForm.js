// Form state <-> API payload for the discussion topic editor, and the
// status badges shown in the index. Mirrors the server's Canvas rules
// (internal/service/discussion_parity_service.go ValidateTopicOptions):
// anonymous topics can't be graded, grouped or section-specific, and
// section-specific topics can't be graded or grouped.

/** datetime-local value ("YYYY-MM-DDTHH:mm") for an ISO time, in local time. */
export function toLocalInput(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** ISO time for a datetime-local value, or null when empty / invalid. */
export function fromLocalInput(value) {
  if (!value) return null;
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

export function emptyTopicForm() {
  return {
    title: '',
    message: '',
    discussion_type: 'threaded',
    pinned: false,
    published: true,
    delayed_post_at: '',
    lock_at: '',
    locked: false,
    require_initial_post: false,
    allow_rating: false,
    only_graders_can_rate: false,
    graded: false,
    points_possible: '10',
    due_at: '',
    assignment_group_id: '',
    group_category_id: '',
    anonymous_state: '',
    is_section_specific: false,
    section_ids: [],
  };
}

/** Form state for an existing topic (GET .../discussion_topics/:id). */
export function topicToForm(topic) {
  const base = emptyTopicForm();
  if (!topic) return base;
  const a = topic.assignment || null;
  return {
    ...base,
    title: topic.title || '',
    message: topic.message || '',
    discussion_type: topic.discussion_type || 'threaded',
    pinned: !!topic.pinned,
    published: topic.published !== false,
    delayed_post_at: toLocalInput(topic.delayed_post_at),
    lock_at: toLocalInput(topic.lock_at),
    locked: !!topic.locked,
    require_initial_post: !!topic.require_initial_post,
    allow_rating: !!topic.allow_rating,
    only_graders_can_rate: !!topic.only_graders_can_rate,
    graded: !!(a || topic.assignment_id),
    points_possible: a && a.points_possible != null ? String(a.points_possible) : base.points_possible,
    due_at: toLocalInput(a?.due_at),
    assignment_group_id: a?.assignment_group_id ? String(a.assignment_group_id) : '',
    group_category_id: topic.group_category_id ? String(topic.group_category_id) : '',
    anonymous_state: topic.anonymous_state || '',
    is_section_specific: !!topic.is_section_specific,
    section_ids: (topic.section_ids || []).map(Number),
  };
}

/**
 * Which options the current choices rule out (Canvas combination rules).
 * Returns `{ graded, group, sections, anonymous }`, each true when that
 * option must stay off.
 */
export function topicOptionLocks(form) {
  const anonymous = !!form.anonymous_state;
  const sections = !!form.is_section_specific;
  const graded = !!form.graded;
  const group = !!form.group_category_id;
  return {
    graded: anonymous || sections,
    group: anonymous || sections,
    sections: anonymous || graded || group,
    anonymous: graded || group || sections,
  };
}

/** Validation messages keyed by field; empty object when valid. */
export function validateTopicForm(form) {
  const errors = {};
  if (!form.title || !form.title.trim()) errors.title = 'Title is required';
  if (form.graded) {
    const pts = Number(form.points_possible);
    if (form.points_possible === '' || Number.isNaN(pts) || pts < 0) {
      errors.points_possible = 'Points must be zero or more';
    }
  }
  if (form.is_section_specific && (!form.section_ids || form.section_ids.length === 0)) {
    errors.section_ids = 'Choose at least one section';
  }
  const locks = topicOptionLocks(form);
  if (form.graded && locks.graded) errors.graded = 'Anonymous or section-specific discussions can\'t be graded';
  if (form.group_category_id && locks.group) errors.group_category_id = 'Anonymous or section-specific discussions can\'t use groups';
  if (form.delayed_post_at && form.lock_at && new Date(form.lock_at) <= new Date(form.delayed_post_at)) {
    errors.lock_at = 'The lock date must be after the available-from date';
  }
  return errors;
}

/**
 * API payload (the inner `discussion_topic` object) for the form.
 * `original` is the topic being edited (undefined when creating): an
 * edit sends nulls to clear dates / group set / anonymity, and
 * `assignment: null` to make a graded topic ungraded.
 */
export function formToPayload(form, original) {
  const editing = !!original;
  const payload = {
    title: form.title.trim(),
    message: form.message,
    discussion_type: form.discussion_type,
    pinned: !!form.pinned,
    published: !!form.published,
    locked: !!form.locked,
    require_initial_post: !!form.require_initial_post,
    allow_rating: !!form.allow_rating,
    only_graders_can_rate: !!form.allow_rating && !!form.only_graders_can_rate,
  };

  const delayed = fromLocalInput(form.delayed_post_at);
  const lockAt = fromLocalInput(form.lock_at);
  if (delayed || editing) payload.delayed_post_at = delayed;
  if (lockAt || editing) payload.lock_at = lockAt;

  const groupSet = form.group_category_id ? Number(form.group_category_id) : null;
  if (groupSet || editing) payload.group_category_id = groupSet;

  const anonymous = form.anonymous_state || null;
  if (anonymous || editing) payload.anonymous_state = anonymous;

  payload.is_section_specific = !!form.is_section_specific;
  if (form.is_section_specific) payload.section_ids = (form.section_ids || []).map(Number);
  else if (editing && original?.is_section_specific) payload.section_ids = [];

  if (form.graded) {
    payload.assignment = {
      points_possible: Number(form.points_possible),
      due_at: fromLocalInput(form.due_at),
    };
    if (form.assignment_group_id) payload.assignment.assignment_group_id = Number(form.assignment_group_id);
  } else if (editing && (original?.assignment || original?.assignment_id)) {
    payload.assignment = null;
  }
  return payload;
}

/**
 * Status badges for a topic row: `[{ key, label, tone }]`.
 * `now` is injectable for tests.
 */
export function topicStatusBadges(topic, now = new Date()) {
  const badges = [];
  if (!topic) return badges;
  if (topic.published === false) badges.push({ key: 'unpublished', label: 'Unpublished', tone: 'muted' });
  if (topic.delayed_post_at && new Date(topic.delayed_post_at) > now) {
    badges.push({
      key: 'delayed',
      label: `Available ${new Date(topic.delayed_post_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}`,
      tone: 'info',
    });
  }
  const lockPassed = topic.lock_at && new Date(topic.lock_at) <= now;
  if (topic.locked || topic.workflow_state === 'locked' || lockPassed) {
    badges.push({ key: 'closed', label: 'Closed for comments', tone: 'muted' });
  } else if (topic.lock_at) {
    badges.push({
      key: 'until',
      label: `Open until ${new Date(topic.lock_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}`,
      tone: 'muted',
    });
  }
  if (topic.assignment) {
    const pts = topic.assignment.points_possible;
    badges.push({ key: 'graded', label: pts != null ? `Graded · ${pts} pts` : 'Graded', tone: 'success' });
  }
  if (topic.group_category_id) badges.push({ key: 'group', label: 'Group discussion', tone: 'info' });
  if (topic.anonymous_state === 'full_anonymity') badges.push({ key: 'anon', label: 'Anonymous', tone: 'info' });
  if (topic.anonymous_state === 'partial_anonymity') badges.push({ key: 'anon', label: 'Partly anonymous', tone: 'info' });
  if (topic.is_section_specific) badges.push({ key: 'sections', label: 'Some sections', tone: 'info' });
  if (topic.require_initial_post) badges.push({ key: 'initial', label: 'Post first', tone: 'muted' });
  return badges;
}
