import React, { useState } from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../../../services/api', () => ({
  api: {},
  getCSRFToken: () => '',
}));

import RichContentEditorV2 from '../RichContentEditorV2';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

let root;
let container;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
});

function Harness({ initial }) {
  const [value, setValue] = useState(initial);
  return <RichContentEditorV2 value={value} onChange={setValue} />;
}

function mount(initial) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root.render(<Harness initial={initial} />);
  });
  const dom = container.querySelector('.ProseMirror');
  expect(dom).not.toBeNull();
  return dom.editor;
}

describe('RichContentEditorV2 value sync', () => {
  // The sanitizer's output for a task list differs from editor.getHTML()
  // (checkboxes come back disabled). The editor must not mistake its own
  // sanitized onChange echo for an external change and reset the document,
  // which moved the caret on every keystroke.
  it('keeps the caret where the user is typing when the content has a task list', () => {
    const editor = mount(
      '<p>hello world</p><ul data-type="taskList"><li data-type="taskItem" data-checked="false"><p>todo</p></li></ul>',
    );
    // Caret right after "hello".
    act(() => {
      editor.commands.setTextSelection(6);
    });
    act(() => {
      editor.commands.insertContent('X');
    });
    expect(editor.getText()).toContain('helloX world');
    expect(editor.state.selection.from).toBe(7);

    act(() => {
      editor.commands.insertContent('Y');
    });
    expect(editor.getText()).toContain('helloXY world');
    expect(editor.state.selection.from).toBe(8);
  });
});
