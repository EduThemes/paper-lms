import {
  buildPaceRows,
  indexProgress,
  moduleStatus,
  progressStateLabel,
  requirementLabel,
} from '../moduleProgress';

describe('requirementLabel', () => {
  test('labels each Canvas completion requirement', () => {
    expect(requirementLabel('must_view')).toBe('View');
    expect(requirementLabel('must_submit')).toBe('Submit');
    expect(requirementLabel('must_contribute')).toBe('Contribute');
    expect(requirementLabel('must_mark_done')).toBe('Mark done');
    expect(requirementLabel('min_score', 7.5)).toBe('Score at least 7.5');
    expect(requirementLabel('min_score', null)).toBe('Score at least');
    expect(requirementLabel('')).toBe('');
    expect(requirementLabel(undefined)).toBe('');
  });
});

describe('indexProgress', () => {
  test('indexes modules and items by id', () => {
    const idx = indexProgress({
      modules: [
        { module_id: 1, state: 'started', items: [{ item_id: 10, completed: true }] },
        { module_id: 2, state: 'locked', items: [{ item_id: 20, locked: true }] },
      ],
    });
    expect(idx.modules.get(2).state).toBe('locked');
    expect(idx.items.get(10)).toMatchObject({ completed: true, module_id: 1 });
    expect(idx.items.get(20)).toMatchObject({ locked: true, module_id: 2 });
  });

  test('tolerates a missing payload', () => {
    const idx = indexProgress(undefined);
    expect(idx.modules.size).toBe(0);
    expect(idx.items.size).toBe(0);
  });
});

describe('module states', () => {
  test('badges', () => {
    expect(moduleStatus({ state: 'completed' })).toEqual({ label: 'Completed', tone: 'success' });
    expect(moduleStatus({ state: 'locked' })).toEqual({ label: 'Locked', tone: 'muted' });
    expect(moduleStatus({ state: 'started' })).toEqual({ label: 'In progress', tone: 'info' });
    expect(moduleStatus({ state: 'unlocked' })).toBeNull();
    expect(moduleStatus(undefined)).toBeNull();
  });

  test('grid labels', () => {
    expect(progressStateLabel('unlocked')).toBe('Not started');
    expect(progressStateLabel('completed')).toBe('Completed');
    expect(progressStateLabel(undefined)).toBe('—');
  });
});

describe('buildPaceRows', () => {
  test('lists every item in module/item order with saved durations', () => {
    const modules = [
      { id: 2, name: 'Week 2', position: 2, items: [{ id: 21, title: 'Quiz 2', type: 'Quiz', position: 1 }] },
      {
        id: 1,
        name: 'Week 1',
        position: 1,
        items: [
          { id: 12, title: 'Essay', type: 'Assignment', position: 2 },
          { id: 10, title: 'Intro', type: 'SubHeader', position: 0 },
          { id: 11, title: 'Read', type: 'Page', position: 1 },
        ],
      },
    ];
    const rows = buildPaceRows(modules, [{ module_item_id: 12, duration: 3 }, { module_item_id: 21, duration: 0 }]);
    expect(rows.map((r) => r.module_item_id)).toEqual([11, 12, 21]);
    expect(rows.map((r) => r.duration)).toEqual([1, 3, 0]);
    expect(rows[0]).toMatchObject({ title: 'Read', module_name: 'Week 1' });
  });

  test('handles empty input', () => {
    expect(buildPaceRows(undefined, undefined)).toEqual([]);
  });
});
