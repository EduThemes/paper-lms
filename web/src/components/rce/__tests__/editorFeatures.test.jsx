import React, { useState } from 'react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

const uploadCourseFileWithProgress = vi.fn();
vi.mock('../../../services/api', () => ({
  api: {
    uploadCourseFileWithProgress: (...args) => uploadCourseFileWithProgress(...args),
    getFileDownloadUrl: (id) => `/api/v1/files/${id}/download`,
    getCourseFiles: vi.fn(async () => ({ data: [] })),
  },
  getCSRFToken: () => '',
}));
vi.mock('../../../services/api_rce_editor', () => ({
  fetchRceDraftKey: vi.fn(async () => null),
}));

import RichContentEditorV2 from '../RichContentEditorV2';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

// jsdom has no layout: ProseMirror measures a Range when it scrolls the
// selection into view after an insert.
if (typeof Range !== 'undefined' && !Range.prototype.getClientRects) {
  Range.prototype.getClientRects = () => [];
  Range.prototype.getBoundingClientRect = () => ({ top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0, x: 0, y: 0 });
}

let root;
let container;
let latest;

beforeEach(() => {
  uploadCourseFileWithProgress.mockReset();
});

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
  latest = undefined;
});

function Harness({ initial, courseId }) {
  const [value, setValue] = useState(initial);
  latest = value;
  return <RichContentEditorV2 value={value} onChange={setValue} courseId={courseId} />;
}

function mount(initial, courseId) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root.render(<Harness initial={initial} courseId={courseId} />);
  });
  return container.querySelector('.ProseMirror').editor;
}

const byLabel = (label) => container.querySelector(`[aria-label="${label}"]`);
const click = (el) => act(() => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })); });

function typeInto(el, value) {
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), 'value').set;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

describe('RichContentEditorV2 HTML source view (G004)', () => {
  it('shows the HTML, saves edits made there, and parses them back on return', () => {
    const editor = mount('<p>Hello</p>');
    click(byLabel('Switch to the HTML editor'));
    const ta = byLabel('HTML source');
    expect(ta).not.toBeNull();
    expect(ta.value).toContain('<p>Hello</p>');
    // Toolbar is disabled while the source is shown.
    expect(byLabel('Bold (Ctrl+B)').disabled).toBe(true);

    typeInto(ta, '<p>Hello</p><p><span style="color: #c0392b">embed code</span></p><script>alert(1)</script>');
    // Saving from the source view saves the (sanitized) source.
    expect(latest).toContain('embed code');
    expect(latest).not.toContain('<script');

    click(byLabel('Switch to the rich text editor'));
    expect(byLabel('HTML source')).toBeNull();
    expect(editor.getText()).toContain('embed code');
    expect(editor.getHTML()).toMatch(/color: (#c0392b|rgb\(192, 57, 43\))/);
  });
});

describe('RichContentEditorV2 HTML view without edits', () => {
  it('does not report a change when the source was only looked at', () => {
    const onChange = vi.fn();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => { root.render(<RichContentEditorV2 value="<h2>Unit</h2><p>Hello</p>" onChange={onChange} />); });
    click(byLabel('Switch to the HTML editor'));
    click(byLabel('Switch to the rich text editor'));
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe('RichContentEditorV2 status bar (G005, G016)', () => {
  it('counts words and characters', () => {
    mount('<p>one two three</p>');
    expect(container.textContent).toContain('3 words');
    expect(container.textContent).toContain('13 characters');
  });

  it('shows the accessibility issue count and opens the checker', () => {
    mount('<h1>Title</h1><img src="/a.png">');
    const btn = container.querySelector('[aria-label^="Accessibility checker,"]');
    expect(btn.getAttribute('aria-label')).toBe('Accessibility checker, 2 issues');
    click(btn);
    const panel = container.querySelector('[aria-label="Accessibility checker"][role="region"]');
    expect(panel).not.toBeNull();
    expect(panel.textContent).toContain('Issue 1 of 2');
  });

  it('toggles full screen', () => {
    mount('<p>x</p>');
    click(byLabel('Full screen'));
    expect(container.querySelector('.rce-v2').getAttribute('data-fullscreen')).toBe('true');
    click(byLabel('Exit full screen'));
    expect(container.querySelector('.rce-v2').getAttribute('data-fullscreen')).toBeNull();
  });

  it('opens find and replace and replaces all matches', () => {
    const editor = mount('<p>cat and cat</p>');
    click(byLabel('Find and replace (Ctrl+F)'));
    typeInto(byLabel('Find'), 'cat');
    expect(container.textContent).toContain('1 of 2');
    typeInto(byLabel('Replace with'), 'dog');
    const replaceAllBtn = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Replace all');
    click(replaceAllBtn);
    expect(editor.getText()).toBe('dog and dog');
  });
});

describe('RichContentEditorV2 inside a page form', () => {
  // Most edit pages wrap the editor in their own <form>. The editor's
  // dialogs and inline bars must never submit (save) that form.
  function mountInForm(initial, onPageSubmit) {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    function FormHarness() {
      const [value, setValue] = useState(initial);
      latest = value;
      return (
        <form onSubmit={(e) => { e.preventDefault(); onPageSubmit(); }}>
          <RichContentEditorV2 value={value} onChange={setValue} />
          <button type="submit">Save page</button>
        </form>
      );
    }
    act(() => { root.render(<FormHarness />); });
    return container.querySelector('.ProseMirror').editor;
  }

  it('inserting a link from the link dialog does not save the page', async () => {
    const onPageSubmit = vi.fn();
    const editor = mountInForm('<p>Read this</p>', onPageSubmit);
    act(() => { editor.commands.setTextSelection({ from: 1, to: 5 }); });
    click(byLabel('Insert link (Ctrl+K)'));
    const url = document.getElementById('rce-link-url');
    expect(url).not.toBeNull();
    typeInto(url, 'https://example.com/reading');
    const form = url.closest('form');
    await act(async () => { form.requestSubmit(); });
    expect(editor.getHTML()).toContain('href="https://example.com/reading"');
    expect(onPageSubmit).not.toHaveBeenCalled();
  });

  it('Enter in an inline editor field does not submit the page form', () => {
    const onPageSubmit = vi.fn();
    mountInForm('<p>cat</p>', onPageSubmit);
    click(byLabel('Find and replace (Ctrl+F)'));
    const replace = byLabel('Replace with');
    const ev = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
    act(() => { replace.dispatchEvent(ev); });
    expect(ev.defaultPrevented).toBe(true);
  });
});

describe('RichContentEditorV2 pasted images (G006)', () => {
  it('uploads a pasted screenshot to the course files and inserts it', async () => {
    uploadCourseFileWithProgress.mockResolvedValue({ id: 42 });
    const editor = mount('<p>Before</p>', 5);
    const file = new File([new Uint8Array([137, 80, 78, 71])], 'image.png', { type: 'image/png' });
    const event = { clipboardData: { files: [file] }, preventDefault: vi.fn() };
    let handled;
    await act(async () => {
      handled = editor.view.someProp('handlePaste', (f) => f(editor.view, event));
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(handled).toBe(true);
    expect(event.preventDefault).toHaveBeenCalled();
    expect(uploadCourseFileWithProgress).toHaveBeenCalledWith(5, expect.any(File), expect.any(Function));
    expect(uploadCourseFileWithProgress.mock.calls[0][1].name).toMatch(/^pasted-image-\d+\.png$/);
    expect(editor.getHTML()).toContain('src="/api/v1/files/42/download"');
  });

  it('explains that upload needs a course instead of silently dropping the image', async () => {
    const editor = mount('<p>Before</p>');
    const file = new File([new Uint8Array([1])], 'shot.png', { type: 'image/png' });
    await act(async () => {
      editor.view.someProp('handlePaste', (f) => f(editor.view, { clipboardData: { files: [file] }, preventDefault() {} }));
    });
    expect(uploadCourseFileWithProgress).not.toHaveBeenCalled();
    expect(container.querySelector('[role="alert"]').textContent).toMatch(/only be uploaded inside a course/);
  });

  it('pastes copied Office content as content, not as its image rendering', () => {
    const editor = mount('<p>x</p>', 5);
    const file = new File([new Uint8Array([1])], 'image.png', { type: 'image/png' });
    const word = '<html xmlns:o="urn:schemas-microsoft-com:office:office"><body><p class=MsoNormal>Week 1 reading</p></body></html>';
    const excel = '<meta name=ProgId content=Excel.Sheet><table><tr><td>1</td></tr></table>';
    const plainHtml = '<p>Copied paragraph</p>';
    [word, excel, plainHtml].forEach((html) => {
      const event = { clipboardData: { files: [file], getData: (t) => (t === 'text/html' ? html : '') }, preventDefault: vi.fn() };
      const handled = editor.view.someProp('handlePaste', (f) => f(editor.view, event));
      expect(handled).toBeFalsy();
      expect(event.preventDefault).not.toHaveBeenCalled();
    });
    expect(uploadCourseFileWithProgress).not.toHaveBeenCalled();
  });

  it('still uploads a copied image whose HTML is only the <img>', async () => {
    uploadCourseFileWithProgress.mockResolvedValue({ id: 43 });
    const editor = mount('<p>x</p>', 5);
    const file = new File([new Uint8Array([1])], 'photo.png', { type: 'image/png' });
    const html = '<meta charset="utf-8"><img src="https://example.com/photo.png" alt="">';
    let handled;
    await act(async () => {
      handled = editor.view.someProp('handlePaste', (f) => f(editor.view, {
        clipboardData: { files: [file], getData: (t) => (t === 'text/html' ? html : '') },
        preventDefault() {},
      }));
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(handled).toBe(true);
    expect(uploadCourseFileWithProgress).toHaveBeenCalled();
    expect(editor.getHTML()).toContain('src="/api/v1/files/43/download"');
  });

  it('lets plain-text paste through to the editor', () => {
    const editor = mount('<p>x</p>', 5);
    const handled = editor.view.someProp('handlePaste', (f) => f(editor.view, { clipboardData: { files: [], getData: () => '' }, preventDefault() {} }));
    expect(handled).toBeFalsy();
  });
});

describe('RichContentEditorV2 editing surface containment', () => {
  it('makes the ProseMirror root a containment box, keeping its classes and textbox role', () => {
    const editor = mount('<p style="position: absolute; z-index: 100; top: -400px">cover the chrome</p>');
    const pm = container.querySelector('.ProseMirror');
    expect(pm.editor).toBe(editor);
    expect(pm.style.contain).toBe('layout paint');
    expect(pm.style.isolation).toBe('isolate');
    expect(pm.style.position).toBe('relative');
    expect(pm).toHaveClass('rce-v2-content', 'prose', 'paper-contained');
    expect(pm).toHaveAttribute('role', 'textbox');
    expect(pm).toHaveAttribute('aria-label', 'Rich content editor');
    expect(pm).toHaveAttribute('contenteditable', 'true');
    // The content is inside the box; the toolbar is not.
    expect(pm.textContent).toContain('cover the chrome');
    expect(pm.contains(byLabel('Bold (Ctrl+B)'))).toBe(false);
  });

  it('keeps the bubble and floating menus outside the box, so it never clips them', () => {
    const editor = mount('<p>Hello</p>');
    const pm = container.querySelector('.ProseMirror');
    // jsdom has no focus/layout for the menus' own show logic, so show them
    // the way their plugin views do and see where Tiptap mounts them.
    const menuViews = editor.view.pluginViews.filter((v) => typeof v.show === 'function' && v.element);
    expect(menuViews).toHaveLength(2); // BubbleMenu + FloatingMenu
    act(() => menuViews.forEach((v) => v.show()));
    for (const { element } of menuViews) {
      expect(element.isConnected).toBe(true);
      expect(element.parentElement).toBe(pm.parentElement);
      expect(pm.contains(element)).toBe(false);
    }
    act(() => menuViews.forEach((v) => v.hide()));
  });
});
