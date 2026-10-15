import { describe, expect, it } from 'vitest';
import { scopeCss, scopeSelector, splitSelectors } from '../lib/previewCss';
import { previewShell, sectionIdOf } from '../lib/previewDoc';
import { renderDesign, renderOptions } from '../lib/engine';
import { makeDoc } from './fixtures';

const R = '#paper-page-preview';

describe('scopeSelector', () => {
  it('makes :root, html and body the page, and prefixes everything else', () => {
    expect(scopeSelector(':root', R)).toBe(R);
    expect(scopeSelector('html body', R)).toBe(R);
    expect(scopeSelector(':root a[data-pd-tile]:hover', R)).toBe(`${R} a[data-pd-tile]:hover`);
    expect(scopeSelector(':root img[data-pd-icon="2"]', R)).toBe(`${R} img[data-pd-icon="2"]`);
    expect(scopeSelector(':root.k2 h2', R)).toBe(`${R}.k2 h2`);
    expect(scopeSelector('h2 > a', R)).toBe(`${R} h2 > a`);
    expect(scopeSelector('htmlish', R)).toBe(`${R} htmlish`);
  });

  it('splits selector lists outside brackets and strings', () => {
    expect(splitSelectors('a, b[x=","], :is(c, d)')).toEqual(['a', 'b[x=","]', ':is(c, d)']);
  });
});

describe('scopeCss', () => {
  it('turns the dark colour-scheme query into html.dark and light into its absence', () => {
    const out = scopeCss(':root{--pd-c1:#fff}@media (prefers-color-scheme: dark){:root{--pd-c1:#000}:root img[data-pd-icon="1"]{content:url("/x.svg")}}@media (prefers-color-scheme: light){a{color:red}}');
    expect(out).toContain(`${R}{--pd-c1:#fff}`);
    expect(out).toContain(`html.dark ${R}{--pd-c1:#000}`);
    expect(out).toContain(`html.dark ${R} img[data-pd-icon="1"]{content:url("/x.svg")}`);
    expect(out).toContain(`html:not(.dark) ${R} a{color:red}`);
    expect(out).not.toContain('prefers-color-scheme');
  });

  it('keeps other media, supports and keyframes; drops imports and font-face', () => {
    const out = scopeCss('@import url(x.css);@font-face{font-family:x}@media (min-width: 600px){p{margin:0}}@supports (display:grid){.g{display:grid}}@keyframes spin{to{transform:rotate(1turn)}}/* c { } */');
    expect(out).not.toContain('@import');
    expect(out).not.toContain('@font-face');
    expect(out).toContain(`@media (min-width: 600px){${R} p{margin:0}}`);
    expect(out).toContain(`@supports (display:grid){${R} .g{display:grid}}`);
    expect(out).toContain('@keyframes spin{to{transform:rotate(1turn)}}');
  });

  it('combined media conditions keep the rest', () => {
    expect(scopeCss('@media (min-width: 600px) and (prefers-color-scheme: dark){p{x:y}}')).toBe(`@media (min-width: 600px){html.dark ${R} p{x:y}}`);
  });

  it('never lets </style through and survives junk', () => {
    expect(scopeCss('p{content:"</style><script>"}')).not.toMatch(/<\/style/i);
    expect(scopeCss('p{color:red')).toBe('');
    expect(scopeCss('')).toBe('');
  });

  it('scopes the real engine stylesheet', () => {
    const { css } = renderDesign(makeDoc(), renderOptions('42'));
    const out = scopeCss(css);
    expect(out).not.toMatch(/(^|[\s,}]):root/);
    expect(out).toContain(R);
  });
});

describe('preview document', () => {
  it('wraps the page in Paper’s containment box with the scoped stylesheet', () => {
    const html = previewShell({ html: '<p data-pd-section="s1">Hi</p>', css: ':root{color:red}', dark: true, appStyles: '<link rel="stylesheet" href="/app.css">' });
    expect(html).toContain('<html lang="en" class="dark">');
    expect(html).toContain('<link rel="stylesheet" href="/app.css">');
    expect(html).toContain('id="paper-page-preview"');
    expect(html).toContain('contain: layout paint; isolation: isolate');
    expect(html).toContain(`${R}{color:red}`);
    const doc = new DOMParser().parseFromString(html, 'text/html');
    expect(sectionIdOf(doc.querySelector('p'))).toBe('s1');
  });
});
