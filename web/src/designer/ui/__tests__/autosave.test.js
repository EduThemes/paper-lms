import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clearDraft, debounce, draftKey, isPageDoc, loadDraft, saveDraft } from '../lib/autosave';
import { makeDoc } from './fixtures';
import { stubLocalStorage, stubThrowingStorage } from './memoryStorage';

describe('drafts', () => {
  beforeEach(() => stubLocalStorage());
  afterEach(() => vi.unstubAllGlobals());

  it('round-trips a draft per course', () => {
    const doc = makeDoc();
    expect(saveDraft('42', doc, { pageUrl: 'home', now: new Date('2026-10-01T10:00:00Z') })).toBe(true);
    const d = loadDraft('42');
    expect(d.doc).toEqual(doc);
    expect(d.pageUrl).toBe('home');
    expect(d.savedAt).toBe('2026-10-01T10:00:00.000Z');
    expect(loadDraft('43')).toBeNull();
    clearDraft('42');
    expect(loadDraft('42')).toBeNull();
  });

  it('ignores corrupt or foreign data', () => {
    localStorage.setItem(draftKey('42'), '{not json');
    expect(loadDraft('42')).toBeNull();
    localStorage.setItem(draftKey('42'), JSON.stringify({ version: 1, savedAt: 'x', doc: { version: 2 } }));
    expect(loadDraft('42')).toBeNull();
  });

  it('cleans a draft on the way in: localStorage is outside the designer', () => {
    const doc = makeDoc([
      { id: 'w', type: 'welcome', heading: 'Hi', body: '<p>Hi</p><img src=x onerror=alert(1)><iframe srcdoc="x"></iframe>' },
      { id: 'q', type: 'quick-links', heading: 'L', links: [{ id: 'l', label: 'L', href: 'javascript:alert(1)', icon: 'book' }] },
      { id: 'z', type: 'evil', html: '<script>alert(1)</script>' },
    ]);
    localStorage.setItem(draftKey('42'), JSON.stringify({ version: 1, savedAt: 'x', pageUrl: null, doc }));
    const d = loadDraft('42');
    expect(d.doc.sections.map((s) => s.id)).toEqual(['w', 'q']);
    expect(d.doc.sections[0].body).toBe('<p>Hi</p>');
    expect(d.doc.sections[1].links[0].href).toBe('');
  });

  it('survives storage that throws (private mode, blocked storage)', () => {
    stubThrowingStorage();
    expect(saveDraft('42', makeDoc())).toBe(false);
    expect(loadDraft('42')).toBeNull();
    expect(() => clearDraft('42')).not.toThrow();
  });

  it('validates PageDocs', () => {
    expect(isPageDoc(makeDoc())).toBe(true);
    expect(isPageDoc({ ...makeDoc(), customCss: ':root{}' })).toBe(true);
    expect(isPageDoc({ ...makeDoc(), customCss: 5 })).toBe(false);
    expect(isPageDoc({ ...makeDoc(), id: 'bad id!' })).toBe(false);
    expect(isPageDoc({ ...makeDoc(), kind: 'blog' })).toBe(false);
    expect(isPageDoc({ ...makeDoc(), sections: [{}] })).toBe(false);
    expect(isPageDoc(null)).toBe(false);
  });
});

describe('debounce', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('runs once after quiet, flushes and cancels', () => {
    const fn = vi.fn();
    const d = debounce(fn, 100);
    d(1);
    d(2);
    vi.advanceTimersByTime(99);
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(fn).toHaveBeenCalledWith(2);
    d(3);
    d.flush();
    expect(fn).toHaveBeenLastCalledWith(3);
    d(4);
    d.cancel();
    vi.advanceTimersByTime(200);
    expect(fn).toHaveBeenCalledTimes(2);
  });
});
