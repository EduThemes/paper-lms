import { notificationLink } from './notificationLinks';

describe('notificationLink', () => {
  test('course notifications open the matching course page', () => {
    expect(notificationLink({ notification_type: 'submission_grade', context_type: 'Course', context_id: 4 })).toBe('/courses/4/grades');
    expect(notificationLink({ notification_type: 'web_conference_invitation', context_type: 'Course', context_id: 4 })).toBe('/courses/4/conferences');
    expect(notificationLink({ notification_type: 'appointment_reserved_by_user', context_type: 'Course', context_id: 4 })).toBe('/courses/4/appointment_groups');
    expect(notificationLink({ notification_type: 'assignment_created', context_type: 'Course', context_id: 4 })).toBe('/courses/4');
  });

  test('invitations, canceled appointments and observer alerts open their own pages', () => {
    expect(notificationLink({ notification_type: 'enrollment_invitation', context_type: 'Course', context_id: 4 })).toBe('/');
    expect(notificationLink({ notification_type: 'appointment_deleted_for_user', context_type: 'Course', context_id: 4 })).toBe('/appointments');
    expect(notificationLink({ notification_type: 'observer_alert', context_type: 'Course', context_id: 4 })).toBe('/observer');
  });

  test('other types', () => {
    expect(notificationLink({ notification_type: 'new_message', context_type: 'Conversation', context_id: 1 })).toBe('/inbox');
    expect(notificationLink({ notification_type: 'event_start' })).toBe('/calendar');
    expect(notificationLink({ notification_type: 'account_notification' })).toBeNull();
    expect(notificationLink(null)).toBeNull();
  });
});
