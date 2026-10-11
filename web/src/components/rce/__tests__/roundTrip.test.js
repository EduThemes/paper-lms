import { describe, it, expect, afterEach } from 'vitest';
import { Editor } from '@tiptap/react';
import { buildRceExtensions } from '../extensions';
import { sanitizeHTML } from '../../RichContentViewer';
import { formatHtml, cleanPastedHtml } from '../contentTransforms';

let editor;
function load(html) {
  editor = new Editor({ extensions: buildRceExtensions(), content: html });
  return editor;
}
/** What the editor would emit on change (editor HTML + client sanitizer). */
function roundTrip(html) {
  return sanitizeHTML(load(html).getHTML());
}

afterEach(() => {
  editor?.destroy();
  editor = null;
});

describe('imported Canvas HTML round-trips through the editor (G003)', () => {
  it('keeps colours, font sizes and background colours on spans (G000)', () => {
    const out = roundTrip('<p><span style="color: #e03e2d; font-size: 18pt;">red</span> and <span style="background-color: rgb(255, 255, 0);">yellow</span></p>');
    expect(out).toMatch(/color: (#e03e2d|rgb\(224, 62, 45\))/);
    expect(out).toContain('font-size: 18pt');
    expect(out).toContain('background-color: rgb(255, 255, 0)');
  });

  it('converts legacy <font> markup to styled spans', () => {
    const out = roundTrip('<p><font color="#ff0000" size="5" face="Georgia">old</font></p>');
    expect(out).toMatch(/color: (#ff0000|rgb\(255, 0, 0\))/);
    expect(out).toContain('font-size: 24px');
    expect(out).toContain('font-family: Georgia');
  });

  it('keeps sub/superscript, direction and lang', () => {
    const out = roundTrip('<p dir="rtl" lang="ar">مرحبا</p><p>H<sub>2</sub>O and x<sup>2</sup></p>');
    expect(out).toContain('dir="rtl"');
    expect(out).toContain('lang="ar"');
    expect(out).toContain('<sub>2</sub>');
    expect(out).toContain('<sup>2</sup>');
  });

  it('keeps ids, classes, aria and Canvas data-api attributes', () => {
    const out = roundTrip(
      '<h2 id="unit-1" class="banner">Unit 1</h2>'
      + '<p role="note" aria-label="Tip">Hint</p>'
      + '<p><a href="/courses/1/pages/intro" data-api-endpoint="/api/v1/courses/1/pages/intro" data-api-returntype="Page">Intro</a></p>',
    );
    expect(out).toContain('id="unit-1"');
    expect(out).toContain('class="banner"');
    expect(out).toContain('role="note"');
    expect(out).toContain('aria-label="Tip"');
    expect(out).toContain('data-api-endpoint="/api/v1/courses/1/pages/intro"');
    expect(out).toContain('data-api-returntype="Page"');
  });

  it('keeps wrapper divs with their classes and styles, content still editable', () => {
    const ed = load('<div class="callout" style="border: 1px solid #333; padding: 10px;"><p>Inside</p></div><section id="s1"><p>Sec</p></section>');
    const out = sanitizeHTML(ed.getHTML());
    expect(out).toMatch(/<div class="callout" style="[^"]*border: 1px solid (#333|rgb\(51, 51, 51\))/);
    expect(out).toContain('<section id="s1">');
    expect(ed.getText()).toContain('Inside');
  });

  it('keeps collapsible details/summary sections and their open state', () => {
    const out = roundTrip('<details open><summary>Answer</summary><p>42</p></details><details><summary>Closed</summary><p>x</p></details>');
    expect(out).toContain('<details open=""><summary>Answer</summary><p>42</p></details>');
    expect(out).toContain('<details><summary>Closed</summary>');
  });

  it('keeps saved equations as equation nodes alongside span styling', () => {
    load('<p>Area <span class="math-tex" data-latex="\\pi r^2">\\(\\pi r^2\\)</span> and <span style="color: #e03e2d;">red</span></p>');
    const types = [];
    editor.state.doc.descendants((n) => { types.push(n.type.name); });
    expect(types).toContain('mathInline');
    const out = sanitizeHTML(editor.getHTML());
    expect(out).toContain('data-latex="\\pi r^2"');
    expect(out).toMatch(/color: (#e03e2d|rgb\(224, 62, 45\))/);
  });

  it('keeps figures, video and definition lists (video through the media node)', () => {
    const out = roundTrip(
      '<figure><img src="/files/1/download" alt="Cell"><figcaption>A cell</figcaption></figure>'
      + '<video controls src="/courses/1/files/2/download"></video>'
      + '<dl><dt>Term</dt><dd>Definition</dd></dl>',
    );
    expect(out).toContain('<figcaption>A cell</figcaption>');
    // <video>/<audio> parse into the media node (extensions/Media.js), which
    // re-renders them with normalized attributes.
    expect(out).toMatch(/<video [^>]*src="\/courses\/1\/files\/2\/download"[^>]*controls/);
    expect(out).toContain('<dl><dt>Term</dt><dd>Definition</dd></dl>');
  });

  it('keeps trusted embeds without double-wrapping them', () => {
    const out = roundTrip('<div class="rce-embed"><iframe src="https://www.youtube.com/embed/abc123XYZ" title="Video"></iframe></div>');
    expect(out.match(/rce-embed/g)).toHaveLength(1);
    expect(out).toContain('src="https://www.youtube.com/embed/abc123XYZ"');
  });

  it('keeps semantic inline elements', () => {
    const out = roundTrip('<p><abbr title="HyperText Markup Language">HTML</abbr> and <cite>Hamlet</cite> and <kbd>Ctrl</kbd></p>');
    expect(out).toContain('<abbr title="HyperText Markup Language">HTML</abbr>');
    expect(out).toContain('<cite>Hamlet</cite>');
    expect(out).toContain('<kbd>Ctrl</kbd>');
  });

  it('maps 40px padding indentation to the indent level and back', () => {
    const ed = load('<p style="padding-left: 80px;">twice</p><p style="padding-left: 15px;">odd</p>');
    expect(ed.state.doc.child(0).attrs.indent).toBe(2);
    const out = ed.getHTML();
    expect(out).toContain('padding-left: 80px');
    expect(out).toContain('padding-left: 15px');
  });

  it('keeps table captions, header scope and resized column widths (G015)', () => {
    const ed = load('<table><caption>Grades</caption><tbody><tr><th scope="col" colwidth="150">Name</th><th>Score</th></tr><tr><td>Ann</td><td>90</td></tr></tbody></table>');
    const out = sanitizeHTML(ed.getHTML());
    expect(out).toContain('<caption>Grades</caption>');
    expect(out).toContain('scope="col"');
    expect(out).toContain('colwidth="150"');
    // The caption must not become a table row.
    expect(ed.getText()).not.toMatch(/Grades[\s\S]*Name/);
  });

  it('keeps decorative images and image sizing (G006)', () => {
    const out = roundTrip('<img src="/files/1/download" alt="" role="presentation" data-decorative="true" width="300" style="width: 300px">');
    expect(out).toContain('role="presentation"');
    expect(out).toContain('data-decorative="true"');
    expect(out).toContain('alt=""');
    expect(out).toContain('width="300"');
  });

  it('keeps link targets only where the author set them, and tel: links (G014)', () => {
    const out = roundTrip('<p><a href="https://example.com" target="_blank">new tab</a> <a href="https://example.org">same tab</a> <a href="tel:+15555550100">call</a></p>');
    expect(out).toMatch(/<a [^>]*href="https:\/\/example.com"[^>]*target="_blank"|<a [^>]*target="_blank"[^>]*href="https:\/\/example.com"/);
    expect(out).not.toMatch(/href="https:\/\/example.org"[^>]*target=/);
    expect(out).toContain('href="tel:+15555550100"');
  });

  it('converts legacy V1 equation spans to \\( \\) text (G017)', () => {
    const ed = load('<p>Area: <span class="math-tex" data-latex="\\pi r^2" contenteditable="false"><span class="katex">junk</span></span></p>');
    expect(ed.getText()).toContain('\\(\\pi r^2\\)');
    expect(ed.getText()).not.toContain('junk');
  });
});

describe('formatting commands (G009)', () => {
  it('applies and clears text colour, background colour and font size', () => {
    const ed = load('<p>hello world</p>');
    ed.chain().setTextSelection({ from: 1, to: 6 }).setColor('#c0392b').setFontSize('18pt').setBackgroundColor('#ffff00').run();
    let html = ed.getHTML();
    expect(html).toMatch(/[^-]color: (#c0392b|rgb\(192, 57, 43\))/);
    expect(html).toContain('font-size: 18pt');
    expect(html).toMatch(/background-color: (#ffff00|rgb\(255, 255, 0\))/);
    ed.chain().setTextSelection({ from: 1, to: 6 }).unsetColor().unsetFontSize().unsetBackgroundColor().run();
    html = ed.getHTML();
    expect(html).not.toContain('<span');
  });

  it('toggles subscript and superscript exclusively', () => {
    const ed = load('<p>x2</p>');
    ed.chain().setTextSelection({ from: 2, to: 3 }).toggleSuperscript().run();
    expect(ed.getHTML()).toContain('<sup>2</sup>');
    ed.chain().setTextSelection({ from: 2, to: 3 }).toggleSubscript().run();
    expect(ed.getHTML()).toContain('<sub>2</sub>');
    expect(ed.getHTML()).not.toContain('<sup>');
  });

  it('indents and outdents paragraphs, and nests list items', () => {
    const ed = load('<p>para</p><ul><li><p>one</p></li><li><p>two</p></li></ul>');
    ed.chain().setTextSelection(2).indent().indent().run();
    expect(ed.getHTML()).toContain('<p style="padding-left: 80px;">para</p>');
    ed.chain().setTextSelection(2).outdent().run();
    expect(ed.getHTML()).toContain('<p style="padding-left: 40px;">para</p>');
    // Cursor in the second list item: indent nests it under the first.
    const pos = ed.getText().indexOf('two');
    let target = 0;
    ed.state.doc.descendants((n, p) => { if (n.isText && n.text === 'two') target = p + 1; });
    expect(pos).toBeGreaterThan(-1);
    ed.chain().setTextSelection(target).indent().run();
    expect(ed.getHTML()).toMatch(/<li><p>one<\/p><ul><li><p>two<\/p><\/li><\/ul><\/li>/);
  });

  it('sets and clears text direction', () => {
    const ed = load('<p>abc</p>');
    ed.chain().setTextSelection(2).setTextDirection('rtl').run();
    expect(ed.getHTML()).toContain('<p dir="rtl">abc</p>');
    ed.chain().setTextSelection(2).unsetTextDirection().run();
    expect(ed.getHTML()).toBe('<p>abc</p>');
  });

  it('sets a table caption and fixes header scopes', () => {
    const ed = load('<p>x</p>');
    ed.chain().setTextSelection(1).insertTable({ rows: 3, cols: 2, withHeaderRow: true }).run();
    ed.commands.fixTableHeaderScopes();
    ed.commands.setTableCaption('Weekly plan');
    const html = ed.getHTML();
    expect(html).toContain('<caption>Weekly plan</caption>');
    expect(html.match(/scope="col"/g)).toHaveLength(2);
  });
});

describe('formatHtml (source view)', () => {
  it('puts blocks on their own lines and parses back to the same document', () => {
    const html = '<h2>T</h2><ul><li><p>a <strong>b</strong></p></li></ul><pre><code>x\n  y</code></pre>';
    const pretty = formatHtml(html);
    expect(pretty).toBe('<h2>T</h2>\n<ul>\n  <li>\n    <p>a <strong>b</strong></p>\n  </li>\n</ul>\n<pre><code>x\n  y</code></pre>');
    const a = load(html).getHTML();
    editor.destroy();
    expect(load(pretty).getHTML()).toBe(a);
  });
});

describe('cleanPastedHtml (Google Docs / Word paste)', () => {
  it('drops default-restating styles but keeps real formatting', () => {
    const pasted = '<b style="font-weight:normal;" id="docs-internal-guid-abc"><p dir="ltr" style="line-height:1.38;margin-top:0pt;margin-bottom:0pt;">'
      + '<span style="font-size:11pt;font-family:Arial,sans-serif;color:#000000;background-color:transparent;font-weight:400;font-style:normal;font-variant:normal;text-decoration:none;vertical-align:baseline;white-space:pre-wrap;">plain </span>'
      + '<span style="font-size:11pt;font-family:Arial,sans-serif;color:#ff0000;background-color:#ffff00;font-weight:700;white-space:pre-wrap;">red</span></p></b>';
    const cleaned = cleanPastedHtml(pasted);
    expect(cleaned).not.toContain('docs-internal-guid');
    expect(cleaned).not.toContain('white-space');
    expect(cleaned).not.toContain('font-family');
    const html = load(cleaned).getHTML();
    expect(html).toMatch(/<p dir="ltr">plain <(strong|span)/);
    expect(html).toMatch(/color: (#ff0000|rgb\(255, 0, 0\))/);
    expect(html).toMatch(/background-color: (#ffff00|rgb\(255, 255, 0\))/);
    // The unformatted run is not wrapped in a span.
    expect(html).not.toMatch(/<span[^>]*>plain/);
  });
});
