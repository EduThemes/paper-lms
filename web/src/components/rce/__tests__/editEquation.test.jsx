import React, { useState } from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../../../services/api', () => ({
  api: {},
  getCSRFToken: () => '',
}));
// The visual editor is lazy-loaded; the test only needs the dialog shell.
vi.mock('mathlive', () => ({}));

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

describe('editing an existing equation (G007)', () => {
  it('renders stored equations with KaTeX and reopens the equation editor on click', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => {
      root.render(<Harness initial={'<p>Area: <span class="math-tex" data-latex="\\pi r^2">\\(\\pi r^2\\)</span></p>'} />);
      await new Promise((r) => setTimeout(r, 0));
    });

    const node = container.querySelector('.rce-math-node');
    expect(node).not.toBeNull();
    expect(node.querySelector('.katex')).not.toBeNull();
    expect(node.getAttribute('aria-label')).toContain('\\pi r^2');

    await act(async () => {
      node.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    expect(document.body.textContent).toContain('Math equation');
  });
});
