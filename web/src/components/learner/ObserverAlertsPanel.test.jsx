import { describe, test, expect } from 'vitest';
import { thresholdChanges } from './ObserverAlertsPanel';

describe('observer alert settings diff (G149)', () => {
  const saved = [
    { id: 1, alert_type: 'course_grade_low', threshold: 70 },
    { id: 2, alert_type: 'assignment_missing', threshold: null },
  ];

  test('creates, updates and removes only what changed', () => {
    const form = {
      course_grade_low: { enabled: true, value: '65' },
      assignment_missing: { enabled: false },
      course_announcement: { enabled: true },
      assignment_grade_high: { enabled: false, value: 90 },
    };
    expect(thresholdChanges(saved, form)).toEqual({
      create: [{ alertType: 'course_announcement', threshold: null }],
      update: [{ id: 1, threshold: 65 }],
      remove: [2],
    });
  });

  test('no changes when the form matches', () => {
    const form = {
      course_grade_low: { enabled: true, value: 70 },
      assignment_missing: { enabled: true },
    };
    expect(thresholdChanges(saved, form)).toEqual({ create: [], update: [], remove: [] });
  });
});
