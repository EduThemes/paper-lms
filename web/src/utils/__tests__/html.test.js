import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  escapeHTML, htmlToText, htmlPreview, looksLikeHTML, plainTextToHtml,
  highlightMentions, normalizeHTML,
} from '../html';

afterEach(() => {
  vi.restoreAllMocks();
});

describe('htmlToText / htmlPreview (B000)', () => {
  it('parses into an inert document, not the live one', () => {
    const create = vi.spyOn(document, 'createElement');
    const text = htmlToText('<p>What is <b>2+2</b>?</p><img src="x" onerror="window.__pwned = 1">');
    expect(text).toBe('What is 2+2?');
    // No element was created in the live document (where <img> would load).
    expect(create).not.toHaveBeenCalled();
    expect(window.__pwned).toBeUndefined();
  });

  it('truncates previews on text, not markup', () => {
    const html = `<p>${'a'.repeat(30)}</p><img src=x onerror=alert(1)>`;
    const preview = htmlPreview(html, 10);
    expect(preview).toBe(`${'a'.repeat(10)}…`);
    expect(preview).not.toContain('<');
  });
});

describe('plainTextToHtml (B003, B108)', () => {
  it('escapes every character so tag-like text stays literal', () => {
    const html = plainTextToHtml('since x<y and y>z then x<z & done');
    expect(html).toBe('<p>since x&lt;y and y&gt;z then x&lt;z &amp; done</p>');
  });

  it('keeps paragraphs and line breaks', () => {
    expect(plainTextToHtml('one\ntwo\n\nthree')).toBe('<p>one<br>two</p><p>three</p>');
  });

  it('builds lists and headings in markdown-lite mode', () => {
    const html = plainTextToHtml('# Plan\n- first\n- second\n\n1. alpha\n2. beta', { markdownLite: true });
    expect(html).toBe('<h2>Plan</h2><ul><li>first</li><li>second</li></ul><ol><li>alpha</li><li>beta</li></ol>');
  });

  it('does not treat dashes as lists unless asked', () => {
    expect(plainTextToHtml('- not a list')).toBe('<p>- not a list</p>');
  });
});

describe('looksLikeHTML', () => {
  it('distinguishes editor HTML from typed text', () => {
    expect(looksLikeHTML('<p>Essay</p>')).toBe(true);
    expect(looksLikeHTML('line<br>line')).toBe(true);
    expect(looksLikeHTML('Tom &amp; Jerry')).toBe(true);
    expect(looksLikeHTML('since x<y and y>z then x<z')).toBe(false);
    expect(looksLikeHTML('plain\nessay')).toBe(false);
  });
});

describe('highlightMentions (B105, B161)', () => {
  it('wraps @mentions in text', () => {
    const out = highlightMentions('<p>Thanks @alex for this</p>');
    expect(out).toContain('<span class="bg-brand-100 text-brand-700 rounded px-0.5 font-medium">@alex</span>');
  });

  it('never rewrites attribute values or link text', () => {
    const html = '<p><a href="mailto:jane@school.edu">jane@school.edu</a> <img src="/logo@2x.png" alt="logo"></p>';
    const out = highlightMentions(html);
    expect(out).toContain('href="mailto:jane@school.edu"');
    expect(out).toContain('src="/logo@2x.png"');
    expect(out).not.toContain('<span');
  });

  it('ignores emails and URL paths in plain text', () => {
    const out = highlightMentions('<p>mail jane@school.edu or see medium.com/@user</p>');
    expect(out).not.toContain('<span');
  });

  it('escapes text it re-serializes', () => {
    const out = highlightMentions('<p>@sam said 1 &lt; 2</p>');
    expect(out).toContain('1 &lt; 2');
  });
});

describe('normalizeHTML (B103)', () => {
  it('treats server re-serialization as the same content', () => {
    const client = '<p><a href="https://example.com" target="_blank" rel="noopener noreferrer nofollow">x</a> Don\'t</p>';
    const server = '<p><a href="https://example.com" rel="nofollow noopener">x</a> Don&#39;t</p>';
    expect(normalizeHTML(client)).toBe(normalizeHTML(server));
  });

  it('still detects real edits', () => {
    expect(normalizeHTML('<p>a</p>')).not.toBe(normalizeHTML('<p>b</p>'));
  });
});

describe('escapeHTML', () => {
  it('escapes markup and quotes', () => {
    expect(escapeHTML(`<a href="x">'&'</a>`)).toBe('&lt;a href=&quot;x&quot;&gt;&#39;&amp;&#39;&lt;/a&gt;');
  });
});
