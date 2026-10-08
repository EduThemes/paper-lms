import React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../../services/api', () => ({
  api: {},
  getCSRFToken: () => '',
}));

import SubmissionForm from '../SubmissionForm';
import { hasRichContent, toEditorHTML, richValueOrEmpty } from '../../utils/richContent';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

let root;
let container;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
});

describe('text-entry submission uses the rich content editor (G010)', () => {
  it('submits the editor HTML and has no AI Assist', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => {
      root.render(<SubmissionForm courseId="3" assignmentId="9" onSubmit={onSubmit} submissionTypes={['online_text_entry']} />);
    });

    const pm = container.querySelector('.ProseMirror');
    expect(pm).not.toBeNull();
    expect(container.querySelector('textarea')).toBeNull();
    expect(container.querySelector('[aria-label="AI Assist"]')).toBeNull();

    const submit = container.querySelector('button[type="submit"]');
    expect(submit.disabled).toBe(true);

    act(() => {
      pm.editor.chain().focus().insertContent('<p>My <strong>answer</strong>: x &lt; y</p>').run();
    });
    expect(submit.disabled).toBe(false);

    await act(async () => {
      submit.click();
    });
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const payload = onSubmit.mock.calls[0][0];
    expect(payload.submission_type).toBe('online_text_entry');
    expect(payload.body).toContain('<strong>answer</strong>');
    expect(payload.body).toContain('x &lt; y');
  });
});

describe('rich content field helpers', () => {
  it('treats an empty editor as empty and embeds as content', () => {
    expect(hasRichContent('<p></p>')).toBe(false);
    expect(hasRichContent('')).toBe(false);
    expect(hasRichContent('<p><img src="/a.png" alt=""></p>')).toBe(true);
    expect(hasRichContent('<span class="math-tex" data-latex="x"></span>')).toBe(true);
    expect(richValueOrEmpty('<p></p>')).toBe('');
  });

  it('turns legacy plain text into paragraphs for the editor', () => {
    expect(toEditorHTML('Bring goggles\nand gloves')).toContain('<br');
    expect(toEditorHTML('x<y and y>z')).toContain('x&lt;y');
    expect(toEditorHTML('<p>already html</p>')).toBe('<p>already html</p>');
  });
});
