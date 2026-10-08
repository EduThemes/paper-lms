import {
  anyJobActive,
  anyRestriction,
  blueprintTypeParam,
  buildCourseCopyRequest,
  buildDateShiftOptions,
  buildPackageSettings,
  dateShiftProblem,
  describeRestrictions,
  exportCounts,
  formatBytes,
  isItemSelected,
  parseRestrictions,
  restrictionsToString,
  summarizeCounts,
  toSelectSettings,
  toggleItem,
  toggleType,
} from '../contentMigration';

describe('course copy request', () => {
  test('copies everything by default and leaves dates alone', () => {
    expect(buildCourseCopyRequest({ sourceCourseId: '12', mode: 'all', selection: {}, dateShift: { mode: 'keep' } })).toEqual({
      migration_type: 'course_copy',
      settings: { source_course_id: 12, copy_everything: true },
    });
  });

  test('a selection becomes include_only_types for whole types and select for items', () => {
    let sel = toggleType({}, 'pages', true);
    sel = toggleItem(sel, 'assignments', 5, true, [5, 6, 7]);
    sel = toggleItem(sel, 'assignments', 7, true, [5, 6, 7]);
    const body = buildCourseCopyRequest({ sourceCourseId: 3, mode: 'select', selection: sel });
    expect(body.settings).toEqual({
      source_course_id: 3,
      include_only_types: ['pages'],
      select: { assignments: [5, 7] },
    });
    expect(body.settings.copy_everything).toBeUndefined();
  });

  test('date shift options are passed through in API shape', () => {
    const body = buildCourseCopyRequest({
      sourceCourseId: 3,
      mode: 'all',
      dateShift: {
        mode: 'shift',
        oldStart: '2025-09-01',
        oldEnd: '2025-12-15',
        newStart: '2026-08-31',
        newEnd: '',
        substitutions: [{ from: '3', to: '4' }, { from: '', to: '1' }],
        timeZone: 'America/Chicago',
      },
    });
    expect(body.date_shift_options).toEqual({
      shift_dates: true,
      old_start_date: '2025-09-01',
      old_end_date: '2025-12-15',
      new_start_date: '2026-08-31',
      day_substitutions: { 3: '4' },
      time_zone: 'America/Chicago',
    });
  });

  test('remove dates and keep dates', () => {
    expect(buildDateShiftOptions({ mode: 'remove' })).toEqual({ remove_dates: true });
    expect(buildDateShiftOptions({ mode: 'keep' })).toBeNull();
    expect(buildDateShiftOptions(null)).toBeNull();
  });

  test('date shift form validation', () => {
    expect(dateShiftProblem({ mode: 'shift', newStart: '' })).toMatch(/new start/i);
    expect(dateShiftProblem({ mode: 'shift', newStart: '2026-01-10', newEnd: '2026-01-01' })).toMatch(/end date/i);
    expect(dateShiftProblem({ mode: 'shift', oldStart: '2025-09-01', oldEnd: '2025-08-01', newStart: '2026-01-10' })).toMatch(/old end/i);
    expect(dateShiftProblem({ mode: 'shift', newStart: '2026-01-10' })).toBeNull();
    expect(dateShiftProblem({ mode: 'keep' })).toBeNull();
  });
});

describe('selection state', () => {
  test('toggling every item of a type collapses to the whole type', () => {
    let sel = toggleItem({}, 'quizzes', 1, true, [1, 2]);
    expect(sel.quizzes).toEqual({ all: false, ids: [1] });
    sel = toggleItem(sel, 'quizzes', 2, true, [1, 2]);
    expect(sel.quizzes).toEqual({ all: true, ids: [] });
    expect(isItemSelected(sel, 'quizzes', 2)).toBe(true);
    // Unchecking one item of a whole type keeps the others.
    sel = toggleItem(sel, 'quizzes', 1, false, [1, 2]);
    expect(sel.quizzes).toEqual({ all: false, ids: [2] });
    sel = toggleItem(sel, 'quizzes', 2, false, [1, 2]);
    expect(sel.quizzes).toBeUndefined();
  });

  test('toSelectSettings omits empty parts', () => {
    expect(toSelectSettings({})).toEqual({});
    expect(toSelectSettings(toggleType({}, 'files', true))).toEqual({ include_only_types: ['files'] });
  });
});

describe('package import settings', () => {
  test('null when nothing is customized', () => {
    expect(buildPackageSettings({ excludeTypes: [], dateShift: { mode: 'keep' } })).toBeNull();
  });
  test('exclusions and date removal', () => {
    expect(buildPackageSettings({ excludeTypes: ['files'], dateShift: { mode: 'remove' } })).toEqual({
      exclude_types: ['files'],
      date_shift_options: { remove_dates: true },
    });
  });
});

describe('job status and results', () => {
  test('active jobs keep polling', () => {
    expect(anyJobActive([{ workflow_state: 'completed' }, { workflow_state: 'running' }])).toBe(true);
    expect(anyJobActive([{ workflow_state: 'exporting' }])).toBe(true);
    expect(anyJobActive([{ workflow_state: 'exported' }, { workflow_state: 'failed' }])).toBe(false);
    expect(anyJobActive(undefined)).toBe(false);
  });

  test('summaries skip zero counts', () => {
    expect(summarizeCounts({ assignments: 3, pages: 0, module_items: 2 })).toBe('3 assignments, 2 module items');
    expect(summarizeCounts(null)).toBe('');
    expect(summarizeCounts(exportCounts({ quizzes_exported: 1, questions_exported: 4 }))).toBe('1 quizzes, 4 quiz questions');
  });

  test('formatBytes', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(2048)).toBe('2.0 KB');
    expect(formatBytes(3 * 1024 * 1024)).toBe('3.0 MB');
  });
});

describe('blueprint restrictions', () => {
  test('parse strings and objects, ignoring junk', () => {
    expect(parseRestrictions('{"content":true}')).toEqual({ content: true, points: false, due_dates: false, availability_dates: false });
    expect(parseRestrictions('not json')).toEqual({ content: false, points: false, due_dates: false, availability_dates: false });
    expect(parseRestrictions({ points: 1, extra: true })).toEqual({ content: false, points: true, due_dates: false, availability_dates: false });
    expect(JSON.parse(restrictionsToString({ due_dates: true }))).toEqual({ content: false, points: false, due_dates: true, availability_dates: false });
  });

  test('descriptions and lock state', () => {
    expect(anyRestriction('{}')).toBe(false);
    expect(anyRestriction({ availability_dates: true })).toBe(true);
    expect(describeRestrictions({ content: true, points: true })).toBe('Locked: content, points');
    expect(describeRestrictions({})).toBe('Unlocked');
  });

  test('content types map onto restrict_item values', () => {
    expect(blueprintTypeParam('WikiPage')).toBe('wiki_page');
    expect(blueprintTypeParam('DiscussionTopic')).toBe('discussion_topic');
    expect(blueprintTypeParam('Quiz')).toBe('quiz');
    expect(blueprintTypeParam('Assignment')).toBe('assignment');
  });
});
