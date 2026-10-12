import { describe, it, expect, afterEach } from 'vitest';
import { Editor } from '@tiptap/react';
import { buildRceExtensions } from '../extensions';
import { checkDocument, contrastRatio, parseColor } from '../a11yRules';

let editor;
function load(html) {
  editor = new Editor({ extensions: buildRceExtensions(), content: html });
  return editor;
}
const rules = (ed) => checkDocument(ed).map((i) => i.rule);

afterEach(() => {
  editor?.destroy();
  editor = null;
});

describe('a11y checker rules (G005)', () => {
  it('flags images without alt text, and fixes by alt text or decorative', () => {
    const ed = load('<img src="/files/1/download"><img src="/files/2/download">');
    let issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['img-alt', 'img-alt']);
    issues[0].fix.apply(ed, { text: 'A red cell under a microscope', checked: false });
    issues = checkDocument(ed);
    expect(issues).toHaveLength(1);
    issues[0].fix.apply(ed, { text: '', checked: true });
    expect(checkDocument(ed)).toHaveLength(0);
    const html = ed.getHTML();
    expect(html).toContain('alt="A red cell under a microscope"');
    expect(html).toContain('role="presentation"');
  });

  it('flags file-name and over-long alt text', () => {
    const long = 'x'.repeat(130);
    const ed = load(`<img src="/files/IMG_2041.jpg" alt="IMG_2041.jpg"><img src="/a.png" alt="${long}">`);
    expect(rules(ed)).toEqual(['img-alt-filename', 'img-alt-length']);
  });

  it('flags H1 and skipped heading levels, and fixes the level', () => {
    const ed = load('<h1>Title</h1><h2>Ok</h2><h4>Skipped</h4>');
    const issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['headings-start-at-h2', 'headings-sequence']);
    issues[1].fix.apply(ed);
    issues[0].fix.apply(ed);
    expect(ed.getHTML()).toMatch(/^<h2>Title<\/h2><h2>Ok<\/h2><h3>Skipped<\/h3>/);
    expect(checkDocument(ed)).toHaveLength(0);
  });

  it('flags very long headings', () => {
    const ed = load(`<h2>${'word '.repeat(40)}</h2>`);
    const [issue] = checkDocument(ed);
    expect(issue.rule).toBe('paragraphs-for-headings');
    issue.fix.apply(ed);
    expect(ed.getHTML()).toMatch(/^<p>/);
  });

  it('flags tables without caption, headers or header scope and fixes them', () => {
    const ed = load('<table><tbody><tr><td>Name</td><td>Score</td></tr><tr><td>Ann</td><td>9</td></tr></tbody></table>');
    let issues = checkDocument(ed);
    expect(issues.map((i) => i.rule).sort()).toEqual(['table-caption', 'table-header']);
    issues.find((i) => i.rule === 'table-header').fix.apply(ed);
    issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['table-caption']);
    issues[0].fix.apply(ed, 'Quiz scores');
    expect(checkDocument(ed)).toHaveLength(0);
    expect(ed.getHTML()).toContain('<caption>Quiz scores</caption>');
    expect(ed.getHTML()).toMatch(/<th [^>]*scope="col"/);

    editor.destroy();
    const ed2 = load('<table><caption>c</caption><tbody><tr><th>A</th></tr><tr><td>1</td></tr></tbody></table>');
    const [scopeIssue] = checkDocument(ed2);
    expect(scopeIssue.rule).toBe('table-header-scope');
    scopeIssue.fix.apply(ed2);
    expect(checkDocument(ed2)).toHaveLength(0);
  });

  it('flags adjacent links to the same URL and merges them', () => {
    const ed = load('<p><a href="https://example.com/a">Read</a> <a href="https://example.com/a">more</a> and <a href="https://example.com/b">other</a></p>');
    const issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['adjacent-links']);
    issues[0].fix.apply(ed);
    expect(ed.getHTML().match(/href="https:\/\/example.com\/a"/g)).toHaveLength(1);
  });

  it('flags low-contrast coloured text and removes the colour', () => {
    const ed = load('<p><span style="color: #bbbbbb">faint</span> <span style="color: #333333">fine</span></p>');
    const issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['small-text-contrast']);
    issues[0].fix.apply(ed);
    expect(checkDocument(ed)).toHaveLength(0);
  });

  it('uses the large-text threshold for big text', () => {
    // #888 on white is about 3.5:1: fails for body text, passes for 24pt.
    const ed = load('<p><span style="color: #888888; font-size: 24pt">big</span></p><p><span style="color: #888888">small</span></p>');
    expect(rules(ed)).toEqual(['small-text-contrast']);
  });

  it('turns paragraphs that look like a list into a list', () => {
    const ed = load('<p>1. First</p><p>2. Second</p><p>3. Third</p><p>After</p>');
    const issues = checkDocument(ed);
    expect(issues.map((i) => i.rule)).toEqual(['list-structure']);
    issues[0].fix.apply(ed);
    expect(ed.getHTML()).toBe('<ol><li><p>First</p></li><li><p>Second</p></li><li><p>Third</p></li></ol><p>After</p>');
  });

  it('reports nothing for clean content', () => {
    const ed = load('<h2>Unit</h2><p>Text with <a href="/x">a link</a>.</p><img src="/a.png" alt="Diagram of the water cycle">');
    expect(checkDocument(ed)).toEqual([]);
  });
});

describe('colour helpers', () => {
  it('parses hex, rgb and named colours', () => {
    expect(parseColor('#fff')).toEqual([255, 255, 255]);
    expect(parseColor('#c0392b')).toEqual([192, 57, 43]);
    expect(parseColor('rgb(1, 2, 3)')).toEqual([1, 2, 3]);
    expect(parseColor('red')).toEqual([255, 0, 0]);
    expect(parseColor('rgba(0, 0, 0, 0.5)')).toBeNull();
    expect(parseColor('var(--x)')).toBeNull();
  });

  it('computes WCAG contrast', () => {
    expect(contrastRatio([0, 0, 0], [255, 255, 255])).toBeCloseTo(21, 0);
    expect(contrastRatio([255, 255, 255], [255, 255, 255])).toBeCloseTo(1, 5);
  });
});
