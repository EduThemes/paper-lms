import { describe, expect, it } from 'vitest';
import { __test } from '../a11y';
import { checkAccessibility, checkHtmlAccessibility, createPage, LAYOUTS } from '../index';
import { parseHtml } from '../html';
import type { A11yIssue, PageDoc, Section, Theme } from '../types';
import { THEMES } from './fixtures/themes';
import { everySection } from './helpers';

const ids = (issues: A11yIssue[]) => issues.map(i => i.id.split(':')[0]);
const find = <T extends Section['type']>(doc: PageDoc, type: T) => doc.sections.find(s => s.type === type) as Extract<Section, { type: T }>;

describe('starter pages pass', () => {
  for (const theme of THEMES)
    for (const L of LAYOUTS)
      it(`${theme.id} / ${L.id}: no errors or warnings`, () => {
        const doc = everySection(theme, L.id);
        const issues = checkAccessibility(doc, theme, { courseId: '9' });
        expect(issues.filter(i => i.severity !== 'suggestion')).toEqual([]);
      });
});

describe('content rules fire on crafted bad docs', () => {
  const theme = THEMES[0];
  const base = () => createPage(theme, 'home', 'en', 'classic-band');

  it('img-alt-filename / img-alt-length / redundant "image of"', () => {
    const doc = base();
    find(doc, 'banner').alt = 'IMG_2024_banner-final.jpg';
    let issues = checkAccessibility(doc, theme, { courseId: '1' });
    const fn = issues.find(i => i.id.startsWith('img-alt-filename'));
    expect(fn?.severity).toBe('error');
    expect(fn?.fix?.patch).toEqual({ alt: '' });
    find(doc, 'banner').alt = 'x'.repeat(121);
    expect(ids(checkAccessibility(doc, theme, { courseId: '1' }))).toContain('img-alt-length');
    find(doc, 'banner').alt = 'Image of a campus at dawn';
    issues = checkAccessibility(doc, theme, { courseId: '1' });
    expect(issues.find(i => i.id.startsWith('img-alt-redundant'))?.fix?.patch).toEqual({ alt: 'A campus at dawn' });
  });

  it('instructor photo without alt, http photo', () => {
    const doc = base();
    Object.assign(find(doc, 'instructor'), { photoSrc: 'http://x.edu/me.jpg', name: 'Dr. Ana Ruiz' });
    const issues = checkAccessibility(doc, theme, { courseId: '1' });
    expect(issues.find(i => i.id.startsWith('img-alt'))?.fix?.patch).toEqual({ photoAlt: 'Photo of Dr. Ana Ruiz' });
    expect(issues.find(i => i.id.startsWith('img-https'))?.fix?.patch).toEqual({ photoSrc: 'https://x.edu/me.jpg' });
  });

  it('banner: http, data: and missing images', () => {
    const doc = base();
    const b = find(doc, 'banner');
    b.customSrc = 'http://x.org/b.jpg';
    expect(checkAccessibility(doc, theme).find(i => i.id.startsWith('img-https'))?.fix?.patch).toEqual({ customSrc: 'https://x.org/b.jpg' });
    b.customSrc = 'data:image/png;base64,AA';
    expect(ids(checkAccessibility(doc, theme))).toContain('img-data-uri');
    b.customSrc = undefined;
    b.imageId = 'nope';
    expect(ids(checkAccessibility(doc, theme))).toContain('banner-missing');
  });

  it('vague, empty, duplicate and unsafe links', () => {
    const doc = base();
    const ql = find(doc, 'quick-links');
    ql.links[0].label = 'Click here';
    ql.links[1].label = '';
    ql.links[2].label = 'Grades';
    ql.links[3].href = 'javascript:void(0)';
    find(doc, 'welcome').body = '<p>Read <a href="/x">here</a>.</p>';
    const issues = ids(checkAccessibility(doc, theme, { courseId: '1' }));
    expect(issues.filter(i => i === 'link-text-vague').length).toBe(2);
    expect(issues).toContain('link-text-empty');
    expect(issues).toContain('link-duplicate-text'); // two "Grades" with different hrefs
    expect(issues).toContain('link-href');
  });

  it('{course} links warn only when no course id is known', () => {
    const doc = base();
    expect(ids(checkAccessibility(doc, theme))).toContain('course-token');
    expect(ids(checkAccessibility(doc, theme, { courseId: '5' }))).not.toContain('course-token');
  });

  it('empty heading (with a one-click default), long heading, all caps', () => {
    const doc = base();
    find(doc, 'welcome').heading = '';
    let issues = checkAccessibility(doc, theme, { courseId: '1' });
    expect(issues.find(i => i.id.startsWith('empty-heading'))?.fix?.patch).toEqual({ heading: 'Welcome to the course!' });
    find(doc, 'welcome').heading = 'W'.repeat(130);
    expect(ids(checkAccessibility(doc, theme, { courseId: '1' }))).toContain('paragraphs-for-headings');
    find(doc, 'welcome').heading = 'WELCOME TO BIOLOGY CLASS';
    issues = checkAccessibility(doc, theme, { courseId: '1' });
    expect(issues.find(i => i.id.startsWith('all-caps'))?.fix?.patch).toEqual({ heading: 'Welcome to biology class' });
  });

  it('fake lists become real lists', () => {
    const doc = base();
    find(doc, 'welcome').body = '<p>Intro</p><p>1. Read</p><p>2. Write</p><p>- loose</p>';
    const issue = checkAccessibility(doc, theme, { courseId: '1' }).find(i => i.id.startsWith('list-structure'));
    expect(issue?.severity).toBe('warning');
    expect((issue?.fix?.patch as { body: string }).body).toBe('<p>Intro</p><ol><li>Read</li><li>Write</li></ol><ul><li>loose</li></ul>');
  });

  it('pasted formatting is flagged with a clean-up fix', () => {
    const doc = base();
    find(doc, 'welcome').body = '<p style="color:red">Hi<img src="a.png"></p>';
    expect(checkAccessibility(doc, theme, { courseId: '1' }).find(i => i.id.startsWith('rich-stripped'))?.fix?.patch).toEqual({ body: '<p>Hi</p>' });
  });

  it('phone-width concerns', () => {
    const doc = base();
    const ql = find(doc, 'quick-links');
    ql.links[0].label = 'Everything about the weekly readings';
    ql.links[1].label = 'Unterrichtsmaterialien';
    const issues = ids(checkAccessibility(doc, theme, { courseId: '1' }));
    expect(issues).toContain('phone-crowded');
    expect(issues).toContain('phone-long-word');
  });

  it('palette contrast actually used is checked on the rendered page', () => {
    const bad: Theme = { ...theme, palettes: [{ ...theme.palettes[0], primary: '#9fc6d0', onPrimary: '#ffffff' }] };
    const doc = createPage(bad, 'home', 'en', 'classic-band');
    // our renderer repairs unreadable pairs (ensureContrast), so even a bad palette renders AA
    expect(ids(checkAccessibility(doc, bad, { courseId: '1' }))).not.toContain('small-text-contrast');
  });
});

describe("Canvas's RCE checker rules on HTML", () => {
  const run = (html: string) => {
    const out: string[] = [];
    __test.scanRendered(parseHtml(html), i => out.push(i.rule));
    return out;
  };
  it('headings-start-at-h2, headings-sequence, empty heading', () => {
    expect(run('<h1>T</h1>')).toContain('headings-start-at-h2');
    expect(run('<h2>A</h2><h4>B</h4>')).toContain('headings-sequence');
    expect(run('<h2></h2>')).toContain('empty-heading');
  });
  it('img-alt, adjacent-links, list-structure', () => {
    expect(run('<img src="https://a/x.png">')).toContain('img-alt');
    expect(run('<img src="https://a/x.png" alt="photo.png">')).toContain('img-alt-filename');
    expect(run('<p><a href="/a"><img src="https://x/i.svg" alt=""></a> <a href="/a">Modules</a></p>')).toContain('adjacent-links');
    expect(run('<p>1. step</p>')).toContain('list-structure');
  });
  it('tables: caption, th, scope', () => {
    const r = run('<table><tr><th>H</th></tr><tr><td>x</td></tr></table>');
    expect(r).toContain('table-caption');
    expect(r).toContain('table-header-scope');
    expect(run('<table><caption>C</caption><tr><td>x</td></tr></table>')).toContain('table-header');
  });
  it('small and large text contrast with inherited colours and sizes', () => {
    expect(run('<div style="background:#ffffff; color:#999999;"><p>grey</p></div>')).toContain('small-text-contrast');
    // 3.0:1 passes for large text (24px)
    expect(run('<div style="background:#ffffff; color:#949494; font-size:1.5rem;">big</div>')).toEqual([]);
    expect(run('<div style="background:#ffffff; color:#aaaaaa; font-size:1.5rem;">big</div>')).toContain('large-text-contrast');
    // skipped over background images, and for visually hidden text
    expect(run('<div style="background:url(https://x/y.jpg); color:#eeeeee;">x</div>')).toEqual([]);
  });
});

describe('checkHtmlAccessibility (pages RMT did not make)', () => {
  it('applies the Canvas checker rules to raw HTML with numbered ids', () => {
    const issues = checkHtmlAccessibility('<h1>Hi</h1><img src="a.png"><img src="b.png"><p style="color:#cccccc">faint</p>');
    const rules = ids(issues);
    expect(rules).toContain('img-alt');
    expect(issues.map(i => i.id)).toContain('img-alt:2');
    expect(rules).toContain('small-text-contrast');
    expect(rules).toContain('headings-start-at-h2');
    expect(checkHtmlAccessibility('<h2>Fine</h2><p>Plain text.</p>')).toEqual([]);
  });
});
