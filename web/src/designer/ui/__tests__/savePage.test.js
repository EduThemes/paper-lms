import { describe, expect, it } from 'vitest';
import { canonicalDoc, inspectSavedPage, pageCss, renderForSave, wikiPagePayload } from '../lib/savePage';
import { docHash } from '../lib/engine';
import { makeDoc } from './fixtures';

/** What Postgres jsonb does to key order: shortest keys first. */
function jsonbOrder(v) {
  if (Array.isArray(v)) return v.map(jsonbOrder);
  if (v && typeof v === 'object') {
    const out = {};
    for (const k of Object.keys(v).sort((a, b) => a.length - b.length || (a < b ? -1 : 1))) out[k] = jsonbOrder(v[k]);
    return out;
  }
  return v;
}

describe('saving a design', () => {
  it('builds the wiki_page payload; a front page is always published', () => {
    const doc = makeDoc();
    const r = renderForSave(doc, { courseId: '42', customCssEnabled: false });
    expect(r.html).toContain('data-pd-hash');
    const p = wikiPagePayload(r.doc, { title: ' Home ', html: r.html, css: r.css, published: false, frontPage: true });
    expect(p).toEqual({ title: 'Home', body: r.html, css: r.css, design: r.doc, published: true, front_page: true });
  });

  it('appends custom CSS only when the course allows it', () => {
    const doc = { ...makeDoc(), customCss: ':root{outline:1px solid red}' };
    expect(pageCss('a{}', doc, true)).toBe('a{}\n:root{outline:1px solid red}');
    expect(pageCss('a{}', doc, false)).toBe('a{}');
    expect(renderForSave(doc, { courseId: '42', customCssEnabled: true }).css).toContain('outline:1px solid red');
  });
});

describe('re-opening a saved page', () => {
  const saved = () => {
    const r = renderForSave(makeDoc(), { courseId: '42', customCssEnabled: false });
    // The server stores design as jsonb (key order changes) and returns it with the body.
    return { url: 'home', body: r.html, design: jsonbOrder(r.doc) };
  };

  it('opens a page the designer made, even after jsonb reorders the keys', () => {
    const page = saved();
    expect(docHash(page.design)).not.toBe(docHash(canonicalDoc(page.design)));
    const res = inspectSavedPage(page, '42');
    expect(res.status).toBe('ok');
    expect(res.doc.id).toBe('doc-test-1');
  });

  it('refuses pages without a design', () => {
    expect(inspectSavedPage({ body: '<p>Mine</p>', design: null }, '42').status).toBe('not-designed');
  });

  it('flags HTML changed outside the designer', () => {
    const page = saved();
    expect(inspectSavedPage({ ...page, body: page.body.replace('Biology 101', 'Chemistry') }, '42').status).toBe('changed');
    expect(inspectSavedPage({ ...page, body: page.body.replace(/data-pd-hash="[0-9a-f]+"/, 'data-pd-hash="00000000"') }, '42').status).toBe('changed');
    expect(inspectSavedPage({ ...page, body: '<p>Rewritten by hand</p>' }, '42').status).toBe('changed');
  });
});
