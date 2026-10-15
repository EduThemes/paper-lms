import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RichTextEditor } from '../editors/RichTextEditor';

const HOSTILE =
  '<p>Hi</p><img src=x onerror="window.__rtePwned=1"><iframe srcdoc="<script>parent.__rtePwned=1</script>" style="position:fixed;inset:0"></iframe>' +
  '<p onclick="x()"><a href="javascript:alert(1)">link</a></p>';

afterEach(() => {
  delete window.__rtePwned;
});

describe('RichTextEditor', () => {
  it('never writes unsanitized HTML into the editing surface (first render)', () => {
    render(<RichTextEditor label="Body" value={HOSTILE} onChange={vi.fn()} />);
    const box = screen.getByRole('textbox', { name: 'Body' });
    expect(box.innerHTML).toBe('<p>Hi</p><p>link</p>');
    expect(document.querySelector('img, iframe, [onclick], [onerror], a[href^="javascript"]')).toBeNull();
    expect(window.__rtePwned).toBeUndefined();
  });

  it('never writes unsanitized HTML when the value changes from outside (undo, a fix)', () => {
    const { rerender } = render(<RichTextEditor label="Body" value="<p>Safe</p>" onChange={vi.fn()} />);
    rerender(<RichTextEditor label="Body" value={HOSTILE} onChange={vi.fn()} />);
    const box = screen.getByRole('textbox', { name: 'Body' });
    expect(box.innerHTML).toBe('<p>Hi</p><p>link</p>');
    expect(document.querySelector('img, iframe')).toBeNull();
  });

  it('keeps the allowed subset as it is', () => {
    const ok = '<p>A <strong>b</strong> <em>c</em> <a href="https://example.org">d</a></p><ul><li>e</li></ul>';
    render(<RichTextEditor label="Body" value={ok} onChange={vi.fn()} />);
    expect(screen.getByRole('textbox', { name: 'Body' }).innerHTML).toBe(ok);
  });
});
