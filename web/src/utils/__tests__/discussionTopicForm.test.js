import {
  emptyTopicForm,
  topicToForm,
  formToPayload,
  topicOptionLocks,
  validateTopicForm,
  topicStatusBadges,
  toLocalInput,
  fromLocalInput,
} from '../discussionTopicForm';
import { appendMention, mentionedUserIds, isBlankMessage, formatFileSize } from '../discussionMentions';

describe('discussionTopicForm', () => {
  test('local datetime inputs round-trip', () => {
    const iso = new Date('2026-05-04T13:30:00').toISOString();
    expect(fromLocalInput(toLocalInput(iso))).toBe(iso);
    expect(fromLocalInput('')).toBeNull();
    expect(toLocalInput(null)).toBe('');
  });

  test('a new graded topic sends an assignment object and no clearing nulls', () => {
    const form = { ...emptyTopicForm(), title: ' Debate ', graded: true, points_possible: '12' };
    const payload = formToPayload(form);
    expect(payload.title).toBe('Debate');
    expect(payload.assignment).toEqual({ points_possible: 12, due_at: null });
    expect(payload).not.toHaveProperty('delayed_post_at');
    expect(payload).not.toHaveProperty('group_category_id');
    expect(payload).not.toHaveProperty('anonymous_state');
  });

  test('editing clears dates, group set and grading explicitly', () => {
    const original = {
      title: 'Old',
      delayed_post_at: '2026-01-01T00:00:00Z',
      group_category_id: 3,
      assignment: { id: 7, points_possible: 5 },
      is_section_specific: true,
      section_ids: [2],
    };
    const form = {
      ...topicToForm(original),
      delayed_post_at: '',
      group_category_id: '',
      graded: false,
      is_section_specific: false,
    };
    const payload = formToPayload(form, original);
    expect(payload.delayed_post_at).toBeNull();
    expect(payload.lock_at).toBeNull();
    expect(payload.group_category_id).toBeNull();
    expect(payload.assignment).toBeNull();
    expect(payload.section_ids).toEqual([]);
    expect(payload.is_section_specific).toBe(false);
  });

  test('topicToForm reads the graded assignment and sections', () => {
    const form = topicToForm({
      title: 'T',
      assignment: { points_possible: 8, assignment_group_id: 4 },
      is_section_specific: true,
      section_ids: [1, 2],
      anonymous_state: null,
    });
    expect(form.graded).toBe(true);
    expect(form.points_possible).toBe('8');
    expect(form.assignment_group_id).toBe('4');
    expect(form.section_ids).toEqual([1, 2]);
    expect(form.anonymous_state).toBe('');
  });

  test('option locks mirror the server rules', () => {
    expect(topicOptionLocks({ anonymous_state: 'full_anonymity' })).toEqual({ graded: true, group: true, sections: true, anonymous: false });
    expect(topicOptionLocks({ is_section_specific: true })).toMatchObject({ graded: true, group: true, anonymous: true });
    expect(topicOptionLocks({ graded: true })).toMatchObject({ sections: true, anonymous: true, group: false });
    expect(topicOptionLocks({ group_category_id: '3' })).toMatchObject({ sections: true, anonymous: true, graded: false });
  });

  test('validation catches missing title, sections and a lock date before availability', () => {
    const errors = validateTopicForm({
      ...emptyTopicForm(),
      is_section_specific: true,
      delayed_post_at: '2026-02-02T10:00',
      lock_at: '2026-02-01T10:00',
    });
    expect(errors).toHaveProperty('title');
    expect(errors).toHaveProperty('section_ids');
    expect(errors).toHaveProperty('lock_at');
    expect(validateTopicForm({ ...emptyTopicForm(), title: 'ok', graded: true, points_possible: '-1' })).toHaveProperty('points_possible');
    expect(validateTopicForm({ ...emptyTopicForm(), title: 'ok' })).toEqual({});
  });

  test('status badges', () => {
    const now = new Date('2026-03-01T12:00:00Z');
    const keys = (t) => topicStatusBadges(t, now).map((b) => b.key);
    expect(keys({ published: false })).toContain('unpublished');
    expect(keys({ delayed_post_at: '2026-03-05T00:00:00Z' })).toContain('delayed');
    expect(keys({ lock_at: '2026-02-01T00:00:00Z' })).toContain('closed');
    expect(keys({ lock_at: '2026-04-01T00:00:00Z' })).toContain('until');
    expect(keys({ locked: true })).toContain('closed');
    expect(keys({ assignment: { points_possible: 3 }, anonymous_state: 'full_anonymity', is_section_specific: true }))
      .toEqual(expect.arrayContaining(['graded', 'anon', 'sections']));
  });
});

describe('discussionMentions', () => {
  test('appendMention adds to the last paragraph or starts one', () => {
    expect(appendMention('', 'Jane Doe')).toBe('<p>@Jane Doe&nbsp;</p>');
    expect(appendMention('<p>Hi</p>', 'Jane Doe')).toBe('<p>Hi @Jane Doe&nbsp;</p>');
    expect(appendMention('<p>a</p><ul><li>b</li></ul>', 'John <x>')).toBe('<p>a</p><ul><li>b</li></ul><p>@John &lt;x&gt;&nbsp;</p>');
  });

  test('mentionedUserIds keeps picked users still named', () => {
    const picked = [{ id: 1, name: 'Jane Doe' }, { id: 2, name: 'John Roe' }];
    expect(mentionedUserIds('<p>thanks @jane doe!</p>', picked)).toEqual([1]);
    expect(mentionedUserIds('<p>nobody</p>', picked)).toEqual([]);
    expect(mentionedUserIds('<p>@Jane Doe @John Roe</p>', [])).toEqual([]);
  });

  test('isBlankMessage and formatFileSize', () => {
    expect(isBlankMessage('<p>&nbsp;</p>')).toBe(true);
    expect(isBlankMessage('<p><img src="x.png"></p>')).toBe(false);
    expect(isBlankMessage('<p>hi</p>')).toBe(false);
    expect(formatFileSize(512)).toBe('512 B');
    expect(formatFileSize(2048)).toBe('2 KB');
    expect(formatFileSize(3 * 1048576)).toBe('3.0 MB');
  });
});
