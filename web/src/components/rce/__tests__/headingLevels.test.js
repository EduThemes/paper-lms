import { describe, it, expect, afterEach } from 'vitest';
import { Editor } from '@tiptap/react';
import { buildRceExtensions } from '../extensions';

// Heading 1 belongs to the page title, as in Canvas: the editor never
// creates one, but keeps any that arrive in existing or pasted content.
let editor;
function load(html) {
  editor = new Editor({ extensions: buildRceExtensions(), content: html });
  editor.commands.focus('end');
  return editor;
}
function type(text) {
  const { view } = editor;
  const { from, to } = view.state.selection;
  const handled = view.someProp('handleTextInput', (f) => f(view, from, to, text, () => view.state.tr.insertText(text, from, to)));
  if (!handled) view.dispatch(view.state.tr.insertText(text, from, to));
}
function press(key) {
  const { view } = editor;
  const event = new KeyboardEvent('keydown', { key, ctrlKey: true, altKey: true, bubbles: true });
  return view.someProp('handleKeyDown', (f) => f(view, event));
}

afterEach(() => {
  editor?.destroy();
  editor = null;
});

describe('content headings start at Heading 2', () => {
  it('Mod+Alt+2 makes Heading 2; Mod+Alt+1 does nothing', () => {
    load('<p>Topic</p>');
    press('1');
    expect(editor.getHTML()).toMatch(/^<p>Topic<\/p>/);
    press('2');
    expect(editor.getHTML()).toMatch(/^<h2>Topic<\/h2>/);
  });

  it('"## " makes Heading 2 and "# " stays text', () => {
    load('<p>##</p>');
    type(' ');
    expect(editor.getHTML()).toMatch(/^<h2><\/h2>/);
    editor.destroy();
    load('<p>#</p>');
    type(' ');
    expect(editor.getHTML()).toMatch(/^<p># <\/p>/);
  });

  it('keeps an existing <h1> rather than rewriting it', () => {
    load('<h1>Old title</h1><p>Body</p>');
    expect(editor.getHTML()).toContain('<h1>Old title</h1>');
  });
});
