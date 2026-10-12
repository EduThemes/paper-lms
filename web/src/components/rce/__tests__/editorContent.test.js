import { describe, it, expect, afterEach } from 'vitest';
import { Editor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Iframe from '../extensions/Iframe';
import { sanitizeHTML } from '../../RichContentViewer';

let editor;
function makeEditor(content = '') {
  editor = new Editor({
    extensions: [StarterKit.configure({ link: false }), Link.configure({ openOnClick: false }), Iframe],
    content,
  });
  return editor;
}

afterEach(() => {
  editor?.destroy();
  editor = null;
});

describe('Iframe node (B006)', () => {
  it('inserts a trusted embed via setIframe', () => {
    const ed = makeEditor('<p>Watch:</p>');
    ed.chain().focus('end').setIframe({ src: 'https://www.youtube.com/embed/abc123XYZ', title: 'Embedded media' }).run();
    const html = ed.getHTML();
    expect(html).toContain('<div class="rce-embed"><iframe');
    expect(html).toContain('src="https://www.youtube.com/embed/abc123XYZ"');
    // Survives the editor's own sanitize-on-change pass.
    expect(sanitizeHTML(html)).toContain('src="https://www.youtube.com/embed/abc123XYZ"');
  });

  it('keeps iframes already in loaded content (imported pages)', () => {
    const ed = makeEditor('<p>a</p><div class="rce-embed"><iframe src="https://player.vimeo.com/video/123456" allowfullscreen="true"></iframe></div><p>b</p>');
    expect(ed.getHTML()).toContain('src="https://player.vimeo.com/video/123456"');
  });

  it('refuses untrusted hosts', () => {
    const ed = makeEditor('<iframe src="https://attacker.example/x"></iframe><p>ok</p>');
    expect(ed.getHTML()).not.toContain('attacker.example');
    expect(ed.commands.setIframe({ src: 'https://attacker.example/x' })).toBe(false);
  });
});

describe('text-node insertion (B101, B106)', () => {
  it('inserts LaTeX containing < and & intact', () => {
    const ed = makeEditor('<p></p>');
    ed.chain().focus().insertContent({ type: 'text', text: '\\(x<y \\& a\\)' }).run();
    expect(ed.getText()).toContain('\\(x<y \\& a\\)');
    expect(ed.getHTML()).toContain('\\(x&lt;y \\&amp; a\\)');
  });

  it('inserts a link whose label looks like a tag', () => {
    const ed = makeEditor('<p></p>');
    ed.chain().focus().insertContent({
      type: 'text', text: 'Grades <Unit 2>', marks: [{ type: 'link', attrs: { href: '/courses/1/pages/grades' } }],
    }).run();
    expect(ed.getText()).toContain('Grades <Unit 2>');
    expect(ed.getHTML()).toContain('href="/courses/1/pages/grades"');
  });
});
