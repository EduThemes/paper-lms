import { describe, it, expect, afterEach, vi } from 'vitest';
import { Editor } from '@tiptap/react';

vi.mock('../../../services/api', () => ({ api: {}, getCSRFToken: () => '' }));

import { buildRceExtensions } from '../extensions';
import { normalizeHref } from '../LinkDialog';
import { isAllowedImageSrc } from '../ImageOptionsDialog';
import { findMatches, replaceAll, replaceMatch, countWords } from '../findReplace';

let editor;
afterEach(() => { editor?.destroy(); editor = null; });

describe('link targets (G014)', () => {
  it('accepts web, mail, phone and course links and rejects script schemes', () => {
    expect(normalizeHref('https://example.com')).toBe('https://example.com');
    expect(normalizeHref('example.com/page')).toBe('https://example.com/page');
    expect(normalizeHref('/courses/1/pages/intro')).toBe('/courses/1/pages/intro');
    expect(normalizeHref('teacher@example.edu')).toBe('mailto:teacher@example.edu');
    expect(normalizeHref('tel:+1 555 555 0100')).toBe('tel:+15555550100');
    expect(normalizeHref('+1 (555) 555-0100')).toBe('tel:+15555550100');
    expect(normalizeHref('javascript:alert(1)')).toBeNull();
    expect(normalizeHref('data:text/html,x')).toBeNull();
    expect(normalizeHref('//cdn.example/x')).toBe('https://cdn.example/x');
  });
});

describe('image sources (G006)', () => {
  it('allows http(s) and same-site paths only', () => {
    expect(isAllowedImageSrc('https://example.com/a.png')).toBe(true);
    expect(isAllowedImageSrc('/api/v1/files/3/download')).toBe(true);
    expect(isAllowedImageSrc('//cdn.example/a.png')).toBe(false);
    expect(isAllowedImageSrc('data:image/png;base64,AAAA')).toBe(false);
    expect(isAllowedImageSrc('javascript:alert(1)')).toBe(false);
  });
});

describe('find and replace (G016)', () => {
  it('finds matches across formatting runs and replaces them', () => {
    editor = new Editor({ extensions: buildRceExtensions(), content: '<p>The <strong>Cat</strong>alog and the cat.</p><p>cat</p>' });
    expect(findMatches(editor.state.doc, 'cat')).toHaveLength(3);
    expect(findMatches(editor.state.doc, 'cat', { caseSensitive: true })).toHaveLength(2);
    expect(findMatches(editor.state.doc, 'catalog')).toHaveLength(1);
    const [first] = findMatches(editor.state.doc, 'catalog');
    replaceMatch(editor, first, 'Index');
    expect(editor.getText()).toContain('The Index and');
    expect(replaceAll(editor, 'cat', 'dog')).toBe(2);
    expect(editor.getText()).toBe('The Index and the dog.\n\ndog');
  });

  it('counts words', () => {
    expect(countWords('')).toBe(0);
    expect(countWords('  one\ttwo \n three ')).toBe(3);
  });
});
