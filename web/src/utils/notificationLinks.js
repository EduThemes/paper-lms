// Where an in-app notification links to, from its notification_type and
// context (shared by the notification bell and the notifications page).

const COURSE_PATHS = {
  submission_grade: 'grades',
  new_announcement: 'announcements',
  discussion_reply: 'discussions',
  web_conference_invitation: 'conferences',
  appointment_reserved_by_user: 'appointment_groups',
  appointment_canceled_by_user: 'appointment_groups',
};

export const notificationLink = (n) => {
  if (!n) return null;
  // Not tied to a course page the recipient can open (yet).
  if (n.notification_type === 'enrollment_invitation') return '/';
  if (n.notification_type === 'appointment_deleted_for_user') return '/appointments';
  if (n.notification_type === 'observer_alert') return '/observer';
  if (n.context_type === 'Course' && n.context_id) {
    const sub = COURSE_PATHS[n.notification_type];
    return sub ? `/courses/${n.context_id}/${sub}` : `/courses/${n.context_id}`;
  }
  if (n.notification_type === 'new_message') return '/inbox';
  if (n.notification_type === 'event_start') return '/calendar';
  return null;
};
