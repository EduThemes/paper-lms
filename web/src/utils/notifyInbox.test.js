import {
  buildConversationPayload, contextRecipientOptions, replyTargets, applyInboxEvent, rowStaysInScope,
  textToHtml, audienceLabel, validateAnnouncementForm, threadReplies, policyCell, participantNames,
  formatBytes, channelLabel,
} from './notifyInbox';

describe('notifyInbox helpers', () => {
  test('buildConversationPayload mixes user ids and context codes', () => {
    expect(buildConversationPayload({
      subject: ' Hi ', body: 'Hello',
      recipients: [{ id: 4, name: 'A' }, { code: 'course_5_students', name: 'All students' }],
      attachmentIds: [9], contextCode: 'course_5', individual: true,
    })).toEqual({
      subject: 'Hi', body: 'Hello', recipients: [4, 'course_5_students'],
      attachment_ids: [9], context_code: 'course_5', group_conversation: false,
    });
    expect(buildConversationPayload({ recipients: [{ id: 1 }] })).toEqual({ subject: '', body: '', recipients: [1] });
  });

  test('contextRecipientOptions only for contexts the caller may address whole', () => {
    expect(contextRecipientOptions({ context_code: 'course_5', name: 'Bio', permissions: { send_messages_all: false } })).toEqual([]);
    const course = contextRecipientOptions({ context_type: 'Course', context_code: 'course_5', name: 'Bio', permissions: { send_messages_all: true } });
    expect(course.map((o) => o.code)).toEqual(['course_5', 'course_5_students', 'course_5_teachers', 'course_5_tas', 'course_5_observers']);
    const group = contextRecipientOptions({ context_type: 'Group', context_code: 'group_7', name: 'Team', permissions: { send_messages_all: true } });
    expect(group).toEqual([{ code: 'group_7', name: 'Everyone in Team' }]);
  });

  test('replyTargets: author of another message, own restricted recipients, else reply all', () => {
    expect(replyTargets({ user_id: 4 }, 1)).toEqual([4]);
    expect(replyTargets({ user_id: 1, recipient_ids: [6] }, 1)).toEqual([6]);
    expect(replyTargets({ user_id: 1 }, 1)).toBeUndefined();
  });

  test('applyInboxEvent and rowStaysInScope', () => {
    const rows = [{ id: 1, workflow_state: 'unread', starred: false }, { id: 2, workflow_state: 'archived', starred: false }];
    expect(applyInboxEvent(rows, [1], 'mark_as_read')[0].workflow_state).toBe('read');
    expect(applyInboxEvent(rows, [2], 'mark_as_read')[1].workflow_state).toBe('archived');
    expect(applyInboxEvent(rows, [1], 'star')[0].starred).toBe(true);
    expect(applyInboxEvent(rows, [2], 'unarchive')[1].workflow_state).toBe('read');
    const archived = applyInboxEvent(rows, [1], 'archive')[0];
    expect(rowStaysInScope(archived, 'inbox')).toBe(false);
    expect(rowStaysInScope(archived, 'archived')).toBe(true);
    expect(rowStaysInScope({ starred: false }, 'starred')).toBe(false);
  });

  test('textToHtml escapes markup and keeps line breaks', () => {
    expect(textToHtml('a <b>\nline\n\nnext & "q"')).toBe('<p>a &lt;b&gt;<br>line</p><p>next &amp; &quot;q&quot;</p>');
    expect(textToHtml('   ')).toBe('');
  });

  test('audienceLabel names sections', () => {
    expect(audienceLabel({ target_audience: 'sections', section_ids: [3, 4] }, { 3: 'Period 1' })).toBe('Sections: Period 1, Section #4');
    expect(audienceLabel({ target_audience: 'section:3' }, { 3: 'Period 1' })).toBe('Section: Period 1');
    expect(audienceLabel({ target_audience: 'students' })).toBe('Students');
  });

  test('validateAnnouncementForm', () => {
    const now = new Date('2026-10-08T12:00:00Z');
    expect(validateAnnouncementForm({ title: '' }, now)).toMatch(/Title/);
    expect(validateAnnouncementForm({ title: 'T', target_audience: 'sections', section_ids: [] }, now)).toMatch(/section/);
    expect(validateAnnouncementForm({ title: 'T', schedule: true, delayed_post_at: '' }, now)).toMatch(/when/);
    expect(validateAnnouncementForm({ title: 'T', schedule: true, delayed_post_at: '2026-10-07T12:00:00Z' }, now)).toMatch(/future/);
    expect(validateAnnouncementForm({ title: 'T', schedule: true, delayed_post_at: '2026-10-09T12:00:00Z', target_audience: 'all' }, now)).toBeNull();
  });

  test('threadReplies nests children under parents', () => {
    const tree = threadReplies([{ id: 1 }, { id: 2, parent_id: 1 }, { id: 3 }, { id: 4, parent_id: 99 }]);
    expect(tree.map((r) => r.id)).toEqual([1, 3, 4]);
    expect(tree[0].children.map((r) => r.id)).toEqual([2]);
  });

  test('policyCell, participantNames, formatBytes, channelLabel', () => {
    const grid = { policies: [{ channel_id: 0, category: 'grading', frequency: 'never', explicit: true }] };
    expect(policyCell(grid, 0, 'grading').frequency).toBe('never');
    expect(policyCell(grid, 1, 'grading')).toBeUndefined();
    expect(participantNames({ participants: [{ id: 1, name: 'Me' }, { id: 2, name: 'Pat' }] }, 1)).toBe('Pat');
    expect(participantNames({ participants: [{ id: 1 }] }, 1)).toBe('Me');
    expect(formatBytes(2048)).toBe('2.0 KB');
    expect(channelLabel({ primary: true, address: 'a@example.test' })).toBe('Email (account): a@example.test');
    expect(channelLabel({ channel_type: 'sms', address: '+15555550100' })).toBe('SMS: +15555550100');
  });
});
