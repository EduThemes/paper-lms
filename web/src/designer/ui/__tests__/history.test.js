import { describe, expect, it } from 'vitest';
import { COALESCE_MS, HISTORY_LIMIT, canRedo, canUndo, docReducer, emptyHistory } from '../lib/history';
import { makeDoc } from './fixtures';

const loaded = () => docReducer(emptyHistory, { type: 'load', doc: makeDoc() });
const ids = (h) => h.present.sections.map((s) => s.id);
const run = (h, ...actions) => actions.reduce(docReducer, h);

describe('docReducer', () => {
  it('load resets history', () => {
    const h = loaded();
    expect(ids(h)).toEqual(['a', 'b', 'c']);
    expect(canUndo(h)).toBe(false);
    expect(canRedo(h)).toBe(false);
  });

  it('adds after a section, or at the end', () => {
    let h = run(loaded(), { type: 'add', section: { id: 'x', type: 'divider' }, afterId: 'a' });
    expect(ids(h)).toEqual(['a', 'x', 'b', 'c']);
    h = run(h, { type: 'add', section: { id: 'y', type: 'divider' } });
    expect(ids(h)).toEqual(['a', 'x', 'b', 'c', 'y']);
    expect(h.past).toHaveLength(2);
  });

  it('moves to an index and by a delta, clamped to the ends', () => {
    let h = run(loaded(), { type: 'move', id: 'c', to: 0 });
    expect(ids(h)).toEqual(['c', 'a', 'b']);
    h = run(h, { type: 'moveBy', id: 'c', delta: 1 });
    expect(ids(h)).toEqual(['a', 'c', 'b']);
    const before = h;
    h = run(h, { type: 'moveBy', id: 'a', delta: -1 });
    expect(h).toBe(before);
    h = run(h, { type: 'move', id: 'a', to: 99 });
    expect(ids(h)).toEqual(['c', 'b', 'a']);
  });

  it('deletes, undoes and redoes', () => {
    let h = run(loaded(), { type: 'remove', id: 'b' });
    expect(ids(h)).toEqual(['a', 'c']);
    h = run(h, { type: 'undo' });
    expect(ids(h)).toEqual(['a', 'b', 'c']);
    h = run(h, { type: 'redo' });
    expect(ids(h)).toEqual(['a', 'c']);
    expect(canRedo(h)).toBe(false);
  });

  it('updates a section without changing its id or type', () => {
    const h = run(loaded(), { type: 'update', id: 'a', patch: { text: 'Chemistry', id: 'zzz', type: 'divider' } });
    expect(h.present.sections[0]).toEqual({ id: 'a', type: 'title', text: 'Chemistry' });
  });

  it('coalesces fast typing in one field into a single undo step', () => {
    const t0 = 1_000_000;
    let h = run(
      loaded(),
      { type: 'update', id: 'a', patch: { text: 'B' }, coalesce: 'text', at: t0 },
      { type: 'update', id: 'a', patch: { text: 'Bi' }, coalesce: 'text', at: t0 + 100 },
      { type: 'update', id: 'a', patch: { text: 'Bio' }, coalesce: 'text', at: t0 + 200 },
    );
    expect(h.past).toHaveLength(1);
    h = run(h, { type: 'update', id: 'a', patch: { text: 'Biology' }, coalesce: 'text', at: t0 + 200 + COALESCE_MS + 1 });
    expect(h.past).toHaveLength(2);
    h = run(h, { type: 'undo' });
    expect(h.present.sections[0].text).toBe('Bio');
    h = run(h, { type: 'undo' });
    expect(h.present.sections[0].text).toBe('Biology 101');
  });

  it('a new change clears the redo stack', () => {
    let h = run(loaded(), { type: 'remove', id: 'c' }, { type: 'undo' });
    expect(canRedo(h)).toBe(true);
    h = run(h, { type: 'moveBy', id: 'a', delta: 1 });
    expect(canRedo(h)).toBe(false);
  });

  it('setDoc merges options, drops palette/customCss when cleared, and is undoable', () => {
    let h = run(loaded(), { type: 'setDoc', patch: { paletteId: 'teal', options: { density: 'compact' } } });
    expect(h.present.paletteId).toBe('teal');
    expect(h.present.options).toEqual({ density: 'compact', headingStyle: 'band' });
    h = run(h, { type: 'setDoc', patch: { customCss: ':root{color:red}' } }, { type: 'setDoc', patch: { customCss: '' } });
    expect('customCss' in h.present).toBe(false);
    h = run(h, { type: 'setDoc', patch: { palette: undefined } });
    expect('palette' in h.present).toBe(false);
    h = run(h, { type: 'undo' }, { type: 'undo' }, { type: 'undo' }, { type: 'undo' });
    expect(h.present.options.density).toBe('comfortable');
  });

  it('caps history length', () => {
    let h = loaded();
    for (let i = 0; i < HISTORY_LIMIT + 20; i++) h = run(h, { type: 'moveBy', id: 'a', delta: i % 2 ? -1 : 1 });
    expect(h.past.length).toBe(HISTORY_LIMIT);
  });

  it('ignores edits when no doc is open, and unknown ids', () => {
    expect(docReducer(emptyHistory, { type: 'remove', id: 'a' })).toBe(emptyHistory);
    const h = loaded();
    expect(docReducer(h, { type: 'remove', id: 'nope' })).toBe(h);
    expect(docReducer(h, { type: 'undo' })).toBe(h);
  });
});
